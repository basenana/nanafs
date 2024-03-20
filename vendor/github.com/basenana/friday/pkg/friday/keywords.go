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
	"strings"

	"github.com/basenana/friday/pkg/llm/prompts"
)

func (f *Friday) Content(content string) *Friday {
	f.statement.content = content
	return f
}

func (f *Friday) Keywords(res *KeywordsState) *Friday {
	prompt := prompts.NewKeywordsPrompt(f.Prompts[keywordsPromptKey])

	answers, usage, err := f.LLM.Completion(f.statement.context, prompt, map[string]string{"context": f.statement.content})
	if err != nil {
		return f
	}
	answer := answers[0]
	keywords := strings.Split(answer, ",")
	result := []string{}
	for _, keyword := range keywords {
		if len(keyword) != 0 {
			result = append(result, strings.TrimSpace(keyword))
		}
	}
	f.Log.Debugf("Keywords result: %v", result)
	res.Keywords = result
	res.Tokens = usage
	return f
}
