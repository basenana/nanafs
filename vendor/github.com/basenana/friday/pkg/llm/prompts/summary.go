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

type SummaryPrompt struct {
	template string
	Context  string
}

const SummaryTemplate = `Write a concise summary of the following and reply in zh-CN Language.:


"{{ .Context }}"


CONCISE SUMMARY: `

var _ PromptTemplate = &SummaryPrompt{}

func NewSummaryPrompt() PromptTemplate {
	return &SummaryPrompt{template: SummaryTemplate}
}

func (p *SummaryPrompt) String(promptContext map[string]string) (string, error) {
	p.Context = promptContext["context"]
	temp := template.Must(template.New("summary").Parse(p.template))
	prompt := new(bytes.Buffer)
	err := temp.Execute(prompt, p)
	if err != nil {
		return "", err
	}
	return prompt.String(), nil
}
