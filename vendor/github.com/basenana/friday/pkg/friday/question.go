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
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"strings"

	"github.com/basenana/friday/pkg/llm/prompts"
	"github.com/basenana/friday/pkg/models"
)

const remainHistoryNum = 5 // must be odd

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

func (f *Friday) OfSummary(summary string) *Friday {
	f.statement.Summary = summary
	return f
}

func (f *Friday) GetRealHistory() []map[string]string {
	return f.statement.history
}

func (f *Friday) preCheck(res *ChatState) error {
	if len(f.statement.history) == 0 {
		return errors.New("history can not be nil")
	}
	if f.LLM == nil {
		return errors.New("llm client of friday is not set")
	}
	if res == nil {
		return errors.New("result can not be nil")
	}
	return nil
}

func (f *Friday) Chat(res *ChatState) *Friday {
	if f.Error = f.preCheck(res); f.Error != nil {
		return f
	}

	var (
		dialogues  = []map[string]string{}
		systemInfo = ""
	)

	// search for docs
	if f.statement.query != nil {
		questions := ""
		for _, d := range f.statement.history {
			if d["role"] == f.LLM.GetUserModel() {
				questions = fmt.Sprintf("%s\n%s", questions, d["content"])
			}
		}
		f.searchDocs(questions)
		if f.Error != nil {
			return f
		}
	}

	// If the number of dialogue rounds exceeds some rounds, should conclude it.
	if len(f.statement.history) >= remainHistoryNum {
		f.statement.HistorySummary = f.summaryHistory().statement.HistorySummary
		if f.Error != nil {
			return f
		}
	}

	// if it already has system Info, rewrite it
	if (f.statement.history)[0]["role"] == "system" {
		f.statement.history = f.statement.history[1:]
	}

	// regenerate system Info
	systemInfo = f.generateSystemInfo()
	dialogues = []map[string]string{
		{"role": f.LLM.GetSystemModel(), "content": systemInfo},
		{"role": f.LLM.GetAssistantModel(), "content": ""},
	}

	if len(f.statement.history) >= remainHistoryNum {
		dialogues = append(dialogues, f.statement.history[len(f.statement.history)-remainHistoryNum:len(f.statement.history)]...)
	} else {
		dialogues = append(dialogues, f.statement.history...)
	}

	// go for llm
	f.statement.history = dialogues // return realHistory
	_, f.Error = f.LLM.Chat(f.statement.context, true, dialogues, res.Response)
	return f
}

func (f *Friday) generateSystemInfo() string {
	systemTemplate := "你是一位知识渊博的文字工作者，负责帮用户阅读文章，基于以下内容，简洁和专业的来回答用户的问题。答案请使用中文。\n"
	if f.statement.Summary != "" {
		systemTemplate += "\n这是文章简介: {{ .Summary }}\n"
	}
	if f.statement.Info != "" {
		systemTemplate += "\n这是相关的已知内容: {{ .Info }}\n"
	}
	if f.statement.HistorySummary != "" {
		systemTemplate += "\n这是历史聊天总结作为前情提要: {{ .HistorySummary }}\n"
	}

	temp := template.Must(template.New("systemInfo").Parse(systemTemplate))
	prompt := new(bytes.Buffer)
	f.Error = temp.Execute(prompt, f.statement)
	if f.Error != nil {
		return ""
	}
	return prompt.String()
}

func (f *Friday) summaryHistory() *Friday {
	sumDialogue := make([]map[string]string, len(f.statement.history))
	copy(sumDialogue, f.statement.history)
	sumDialogue[len(sumDialogue)-1] = map[string]string{
		"role":    f.LLM.GetSystemModel(),
		"content": "简要总结一下对话内容，用作后续的上下文提示 prompt，控制在 200 字以内",
	}
	var (
		sumBuf = make(chan map[string]string)
		sum    = make(map[string]string)
		errCh  = make(chan error)
	)
	go func() {
		defer close(errCh)
		_, err := f.LLM.Chat(f.statement.context, false, sumDialogue, sumBuf)
		errCh <- err
	}()
	select {
	case err := <-errCh:
		if err != nil {
			f.Error = err
		}
	case sum = <-sumBuf:
		f.statement.HistorySummary = sum["content"]
	}
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
		"context":  f.statement.Info,
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
	f.statement.Info = strings.Join(cs, "\n")
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
