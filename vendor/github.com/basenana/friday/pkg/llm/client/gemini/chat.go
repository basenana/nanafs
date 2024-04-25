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
	"strings"
)

func (g *Gemini) GetUserModel() string {
	return "user"
}

func (g *Gemini) GetSystemModel() string {
	return "user"
}

func (g *Gemini) GetAssistantModel() string {
	return "model"
}

func (g *Gemini) Chat(ctx context.Context, stream bool, history []map[string]string, answers chan<- map[string]string) (tokens map[string]int, err error) {
	defer close(answers)
	var path string
	path = fmt.Sprintf("v1beta/models/%s:generateContent", *g.conf.Model)
	if stream {
		path = fmt.Sprintf("v1beta/models/%s:streamGenerateContent", *g.conf.Model)
	}

	contents := make([]map[string]any, 0)
	for _, hs := range history {
		contents = append(contents, map[string]any{
			"role": hs["role"],
			"parts": []map[string]string{{
				"text": hs["content"],
			}},
		})
	}

	buf := make(chan []byte)
	go func() {
		defer close(buf)
		err = g.request(ctx, stream, path, "POST", map[string]any{"contents": contents}, buf)
		if err != nil {
			return
		}
	}()

	for line := range buf {
		ans := make(map[string]string)
		l := strings.TrimSpace(string(line))
		if stream {
			if l == "EOF" {
				ans["content"] = "EOF"
			} else {
				if !strings.HasPrefix(l, "\"text\"") {
					continue
				}
				// it should be: "text": "xxx"
				ans["content"] = l[9 : len(l)-2]
			}
		} else {
			var res ChatResult
			err = json.Unmarshal(line, &res)
			if err != nil {
				return nil, err
			}
			if len(res.Candidates) == 0 && res.PromptFeedback.BlockReason != "" {
				g.log.Errorf("gemini response: %s ", string(line))
				return nil, fmt.Errorf("gemini api block because of %s", res.PromptFeedback.BlockReason)
			}
			for _, c := range res.Candidates {
				for _, t := range c.Content.Parts {
					ans["role"] = c.Content.Role
					ans["content"] = t.Text
				}
			}
		}
		select {
		case <-ctx.Done():
			err = fmt.Errorf("context timeout in gemini chat")
			return
		case answers <- ans:
			continue
		}
	}
	return nil, err
}
