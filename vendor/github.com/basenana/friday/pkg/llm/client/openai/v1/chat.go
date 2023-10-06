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
	"strings"
	"time"

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

func (o *OpenAIV1) Chat(prompt prompts.PromptTemplate, parameters map[string]string) ([]string, error) {
	answer, err := o.chat(prompt, parameters)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "rate_limit_exceeded") {
			o.log.Warnf("meets rate limit exceeded, sleep %d second and retry", o.rateLimit)
			time.Sleep(time.Duration(o.rateLimit) * time.Second)
			return o.chat(prompt, parameters)
		}
		return nil, err
	}
	return answer, err
}

func (o *OpenAIV1) chat(prompt prompts.PromptTemplate, parameters map[string]string) ([]string, error) {
	path := "chat/completions"

	model := "gpt-3.5-turbo"
	p, err := prompt.String(parameters)
	if err != nil {
		return nil, err
	}
	o.log.Debugf("final prompt: %s", p)

	data := map[string]interface{}{
		"model":             model,
		"messages":          []interface{}{map[string]string{"role": "user", "content": p}},
		"max_tokens":        1024,
		"temperature":       0.7,
		"top_p":             1,
		"frequency_penalty": 0,
		"presence_penalty":  0,
		"n":                 1,
	}
	postBody, _ := json.Marshal(data)

	respBody, err := o.request(path, "POST", bytes.NewBuffer(postBody))
	if err != nil {
		return nil, err
	}

	var res ChatResult
	err = json.Unmarshal(respBody, &res)
	if err != nil {
		return nil, err
	}
	ans := make([]string, len(res.Choices))
	for i, c := range res.Choices {
		ans[i] = c.Message["content"]
	}
	return ans, err
}
