/*
 * Copyright 2023 friday
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package friday

import (
	"errors"
	"fmt"
	"strings"

	"github.com/basenana/friday/pkg/llm/prompts"
	"github.com/basenana/friday/pkg/models"
)

func (f *Friday) History(history []map[string]string) *Friday {
	f.statement.history = history
	return f
}

func (f *Friday) SearchIn(query *models.DocQuery) *Friday {
	f.statement.query = query
	return f
}

func (f *Friday) Question(q string) *Friday {
	f.statement.question = q
	return f
}

func (f *Friday) Chat(res *ChatState) *Friday {
	if len(f.statement.history) == 0 {
		f.Error = errors.New("history can not be nil")
		return f
	}
	if f.LLM == nil {
		f.Error = errors.New("llm client of friday is not set")
		return f
	}

	if f.statement.query != nil {
		// search for docs
		questions := ""
		for _, d := range f.statement.history {
			if d["role"] == "user" {
				questions = fmt.Sprintf("%s\n%s", questions, d["content"])
			}
		}
		f.searchDocs(questions)
		if f.Error != nil {
			return f
		}
	}
	if (f.statement.history)[0]["role"] == "system" {
		f.statement.history = f.statement.history[1:]
	}
	f.statement.history = append([]map[string]string{
		{
			"role":    "system",
			"content": fmt.Sprintf("基于以下已知信息，简洁和专业的来回答用户的问题。答案请使用中文。 \n\n已知内容: %s", f.statement.info),
		},
	}, f.statement.history...)

	return f.chat(res)
}

func (f *Friday) chat(res *ChatState) *Friday {
	if res == nil {
		f.Error = errors.New("result can not be nil")
		return f
	}
	var (
		tokens    = map[string]int{}
		dialogues = []map[string]string{}
	)

	// If the number of dialogue rounds exceeds 2 rounds, should conclude it.
	if len(f.statement.history) >= 5 {
		sumDialogue := make([]map[string]string, 0, len(f.statement.history))
		copy(sumDialogue, f.statement.history)
		sumDialogue = append(sumDialogue, map[string]string{
			"role":    "system",
			"content": "简要总结一下对话内容，用作后续的上下文提示 prompt，控制在 200 字以内",
		})
		var (
			sumBuf = make(chan map[string]string)
			sum    = make(map[string]string)
			usage  = make(map[string]int)
			err    error
		)
		defer close(sumBuf)
		go func() {
			usage, err = f.LLM.Chat(f.statement.context, false, sumDialogue, sumBuf)
		}()
		if err != nil {
			f.Error = err
			return f
		}
		tokens = mergeTokens(usage, tokens)
		select {
		case <-f.statement.context.Done():
			return f
		case sum = <-sumBuf:
			// add context prompt for dialogue
			dialogues = append(dialogues, []map[string]string{
				f.statement.history[0],
				{
					"role":    "system",
					"content": fmt.Sprintf("这是历史聊天总结作为前情提要：%s", sum["content"]),
				},
			}...)
			dialogues = append(dialogues, f.statement.history[len(f.statement.history)-5:len(f.statement.history)]...)
		}
	} else {
		dialogues = make([]map[string]string, len(f.statement.history))
		copy(dialogues, f.statement.history)
	}

	// go for llm
	usage, err := f.LLM.Chat(f.statement.context, true, dialogues, res.Response)
	if err != nil {
		f.Error = err
		return f
	}
	tokens = mergeTokens(tokens, usage)

	res.Tokens = tokens
	return f
}

func (f *Friday) Complete(res *ChatState) *Friday {
	if res == nil {
		f.Error = errors.New("result can not be nil")
		return f
	}
	if len(f.statement.question) == 0 {
		f.Error = errors.New("question can not be nil")
		return f
	}
	if f.LLM == nil {
		f.Error = errors.New("llm client of friday is not set")
		return f
	}

	prompt := prompts.NewQuestionPrompt(f.Prompts[questionPromptKey])
	if f.statement.query != nil {
		f.searchDocs(f.statement.question)
		if f.Error != nil {
			return f
		}
	}
	ans, usage, err := f.LLM.Completion(f.statement.context, prompt, map[string]string{
		"context":  f.statement.info,
		"question": f.statement.question,
	})
	if err != nil {
		f.Error = fmt.Errorf("llm completion error: %w", err)
		return f
	}
	f.Log.Debugf("Question result: %s", ans[0])
	res.Answer = ans[0]
	res.Tokens = usage
	return f
}

func (f *Friday) searchDocs(q string) {
	f.Log.Debugf("vector query for %s ...", q)
	qv, _, err := f.Embedding.VectorQuery(f.statement.context, q)
	if err != nil {
		f.Error = fmt.Errorf("vector embedding error: %w", err)
		return
	}
	var dq models.DocQuery
	if f.statement.query != nil {
		dq = *f.statement.query
	}
	docs, err := f.Vector.Search(f.statement.context, dq, qv, *f.VectorTopK)
	if err != nil {
		f.Error = fmt.Errorf("vector search error: %w", err)
		return
	}

	cs := []string{}
	for _, c := range docs {
		//f.Log.Debugf("searched from [%s] for %s", c.Name, c.Content)
		cs = append(cs, c.Content)
	}
	f.statement.info = strings.Join(cs, "\n")
	return
}

func mergeTokens(tokens, merged map[string]int) map[string]int {
	result := make(map[string]int)
	for k, v := range tokens {
		result[k] = v
	}
	for k, v := range merged {
		if _, ok := result[k]; !ok {
			result[k] = v
		} else {
			result[k] += v
		}
	}
	return result
}
