/*
 Copyright 2023 Friday Author.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package gemini

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/basenana/friday/pkg/llm/prompts"
)

type ChatResult struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
			Role string `json:"role"`
		} `json:"content"`
		FinishReason  string `json:"finishReason"`
		Index         int    `json:"index"`
		SafetyRatings []struct {
			Category    string `json:"category"`
			Probability string `json:"probability"`
		} `json:"safetyRatings"`
	} `json:"candidates"`
	PromptFeedback struct {
		BlockReason   string `json:"blockReason"`
		SafetyRatings []struct {
			Category    string `json:"category"`
			Probability string `json:"probability"`
		} `json:"safetyRatings"`
	} `json:"promptFeedback"`
}

func (g *Gemini) Completion(ctx context.Context, prompt prompts.PromptTemplate, parameters map[string]string) ([]string, map[string]int, error) {
	path := fmt.Sprintf("v1beta/models/%s:generateContent", *g.conf.Model)

	p, err := prompt.String(parameters)
	if err != nil {
		return nil, nil, err
	}

	data := map[string]interface{}{
		"contents": []interface{}{map[string]any{
			"role":  "user",
			"parts": map[string]string{"text": p},
		}},
	}

	buf := make(chan []byte)
	err = g.request(ctx, false, path, "POST", data, buf)
	if err != nil {
		return nil, nil, err
	}

	var res ChatResult
	respBody := <-buf
	err = json.Unmarshal(respBody, &res)
	if err != nil {
		return nil, nil, err
	}
	if len(res.Candidates) == 0 && res.PromptFeedback.BlockReason != "" {
		g.log.Errorf("gemini response: %s ", string(respBody))
		return nil, nil, fmt.Errorf("gemini api block because of %s", res.PromptFeedback.BlockReason)
	}
	ans := make([]string, 0)
	for _, c := range res.Candidates {
		for _, t := range c.Content.Parts {
			ans = append(ans, t.Text)
		}
	}
	return ans, nil, err
}
