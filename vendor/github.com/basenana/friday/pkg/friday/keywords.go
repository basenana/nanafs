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

package friday

import (
	"context"
	"strings"

	"github.com/basenana/friday/pkg/llm/prompts"
)

func (f *Friday) Keywords(ctx context.Context, content string) (keywords []string, err error) {
	prompt := prompts.NewKeywordsPrompt(f.Prompts[keywordsPromptKey])

	answers, err := f.LLM.Chat(ctx, prompt, map[string]string{"context": content})
	if err != nil {
		return []string{}, err
	}
	answer := answers[0]
	keywords = strings.Split(answer, ",")
	result := []string{}
	for _, keyword := range keywords {
		if len(keyword) != 0 {
			result = append(result, strings.TrimSpace(keyword))
		}
	}
	f.Log.Debugf("Keywords result: %v", result)
	return result, nil
}
