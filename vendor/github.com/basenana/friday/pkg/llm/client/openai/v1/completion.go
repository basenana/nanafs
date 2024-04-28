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

func (o *OpenAIV1) Completion(ctx context.Context, prompt prompts.PromptTemplate, parameters map[string]string) ([]string, map[string]int, error) {
	return o.completion(ctx, prompt, parameters)
}

func (o *OpenAIV1) completion(ctx context.Context, prompt prompts.PromptTemplate, parameters map[string]string) ([]string, map[string]int, error) {
	path := "v1/chat/completions"

	p, err := prompt.String(parameters)
	if err != nil {
		return nil, nil, err
	}

	data := map[string]interface{}{
		"model":             *o.conf.Model,
		"messages":          []interface{}{map[string]string{"role": "user", "content": p}},
		"max_tokens":        *o.conf.MaxReturnToken,
		"temperature":       *o.conf.Temperature,
		"top_p":             1,
		"frequency_penalty": *o.conf.FrequencyPenalty,
		"presence_penalty":  *o.conf.PresencePenalty,
		"n":                 1,
	}

	var (
		buf   = make(chan []byte)
		errCh = make(chan error)
	)
	go func() {
		defer close(errCh)
		err = o.request(ctx, false, path, "POST", data, buf)
		if err != nil {
			errCh <- err
		}
	}()

	select {
	case err = <-errCh:
		return nil, nil, err
	case line := <-buf:
		var res ChatResult
		err = json.Unmarshal(line, &res)
		if err != nil {
			return nil, nil, err
		}
		ans := make([]string, len(res.Choices))
		for i, c := range res.Choices {
			ans[i] = c.Message["content"]
		}
		return ans, res.Usage, err
	}
}
