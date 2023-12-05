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

package prompts

import (
	"bytes"
	"html/template"
)

type KeywordsPrompt struct {
	template string
	Context  string
}

const KeyWordsTemplate = `Extract keywords from the following, separated by comma and reply in zh-CN Language:


"{{ .Context }}"


KEYWORDS: `

var _ PromptTemplate = &KeywordsPrompt{}

func NewKeywordsPrompt(t string) PromptTemplate {
	if t == "" {
		t = KeyWordsTemplate
	}
	return &KeywordsPrompt{template: t}
}

func (p *KeywordsPrompt) String(promptContext map[string]string) (string, error) {
	p.Context = promptContext["context"]
	temp := template.Must(template.New("keywords").Parse(p.template))
	prompt := new(bytes.Buffer)
	err := temp.Execute(prompt, p)
	if err != nil {
		return "", err
	}
	return prompt.String(), nil
}
