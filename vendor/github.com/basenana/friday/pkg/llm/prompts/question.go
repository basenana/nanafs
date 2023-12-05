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

type QuestionPrompt struct {
	template string
	Context  string
	Question string
}

const QuestionTemplate = `基于以下已知信息，简洁和专业的来回答用户的问题。
如果无法从中得到答案，请说 "根据已知信息无法回答该问题" 或 "没有提供足够的相关信息"，不允许在答案中添加编造成分，答案请使用中文。

已知内容:
{{ .Context }}

问题:
{{ .Question }}`

const QuestionTemplateEN = `Answer user questions concisely and professionally based on the following known information.
If you don't know the answer, just say that you don't know, don't try to make up an answer.

Known content:
{{ .Context }}

Question:
{{ .Question }}`

var _ PromptTemplate = &QuestionPrompt{}

func NewQuestionPrompt(t string) PromptTemplate {
	if t == "" {
		t = QuestionTemplate
	}
	return &QuestionPrompt{template: t}
}

func (p *QuestionPrompt) String(promptContext map[string]string) (string, error) {
	p.Context = promptContext["context"]
	p.Question = promptContext["question"]
	temp := template.Must(template.New("knowledge").Parse(p.template))
	prompt := new(bytes.Buffer)
	err := temp.Execute(prompt, p)
	if err != nil {
		return "", err
	}
	return prompt.String(), nil
}
