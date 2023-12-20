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

package v1

import (
	"context"
	"encoding/json"

	"github.com/basenana/friday/pkg/llm/prompts"
)

type ChatResult struct {
	Id      string         `json:"id"`
	Object  string         `json:"object"`
	Created int            `json:"created"`
	Model   string         `json:"model"`
	Choices []ChatChoice   `json:"choices"`
	Usage   map[string]int `json:"usage"`
}

type ChatChoice struct {
	Index        int               `json:"index"`
	Message      map[string]string `json:"message"`
	FinishReason string            `json:"finish_reason"`
}

func (o *OpenAIV1) Chat(ctx context.Context, history []map[string]string, prompt prompts.PromptTemplate, parameters map[string]string) ([]string, map[string]int, error) {
	return o.chat(ctx, history, prompt, parameters)
}

func (o *OpenAIV1) chat(ctx context.Context, history []map[string]string, prompt prompts.PromptTemplate, parameters map[string]string) ([]string, map[string]int, error) {
	path := "v1/chat/completions"

	p, err := prompt.String(parameters)
	if err != nil {
		return nil, nil, err
	}

	histories := make([]map[string]string, 0)
	for _, hs := range history {
		for user, content := range hs {
			histories = append(histories, map[string]string{
				"role":    user,
				"content": content,
			})
		}
	}
	histories = append(histories,
		map[string]string{
			"role":    "user",
			"content": p,
		},
	)

	data := map[string]interface{}{
		"model":             *o.conf.Model,
		"messages":          []interface{}{histories},
		"max_tokens":        *o.conf.MaxReturnToken,
		"temperature":       *o.conf.Temperature,
		"top_p":             1,
		"frequency_penalty": *o.conf.FrequencyPenalty,
		"presence_penalty":  *o.conf.PresencePenalty,
		"n":                 1,
	}

	respBody, err := o.request(ctx, path, "POST", data)
	if err != nil {
		return nil, nil, err
	}

	var res ChatResult
	err = json.Unmarshal(respBody, &res)
	if err != nil {
		return nil, nil, err
	}
	ans := make([]string, len(res.Choices))
	for i, c := range res.Choices {
		ans[i] = c.Message["content"]
	}
	return ans, res.Usage, err
}
