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
	"bytes"
	"encoding/json"

	"github.com/basenana/friday/pkg/llm/prompts"
)

type CompletionResult struct {
	Id      string         `json:"id"`
	Object  string         `json:"object"`
	Created int            `json:"created"`
	Model   string         `json:"model"`
	Choices []Choice       `json:"choices"`
	Usage   map[string]int `json:"usage"`
}

type Choice struct {
	Index        int         `json:"index"`
	Text         string      `json:"text"`
	FinishReason string      `json:"finish_reason"`
	Logprobs     interface{} `json:"logprobs"`
}

func (o *OpenAIV1) Completion(prompt prompts.PromptTemplate, parameters map[string]string) ([]string, error) {
	path := "completions"

	model := "text-davinci-003"
	p, err := prompt.String(parameters)
	if err != nil {
		return nil, err
	}
	data := map[string]interface{}{
		"model":             model,
		"prompt":            p,
		"max_tokens":        1024,
		"temperature":       0.7,
		"top_p":             1,
		"frequency_penalty": 0,
		"presence_penalty":  0,
		"n":                 1,
		"best_of":           1,
	}
	postBody, _ := json.Marshal(data)

	respBody, err := o.request(path, "POST", bytes.NewBuffer(postBody))
	if err != nil {
		return nil, err
	}

	var res CompletionResult
	err = json.Unmarshal(respBody, &res)
	if err != nil {
		return nil, err
	}
	ans := make([]string, len(res.Choices))
	for i, c := range res.Choices {
		ans[i] = c.Text
	}
	return ans, err
}
