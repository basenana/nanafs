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

package summary

import (
	"github.com/basenana/friday/pkg/llm"
	"github.com/basenana/friday/pkg/llm/prompts"
	"github.com/basenana/friday/pkg/utils/logger"
)

const DefaultSummaryLimitToken = 4000

type Summary struct {
	log logger.Logger

	llm           llm.LLM
	summaryPrompt prompts.PromptTemplate
	combinePrompt prompts.PromptTemplate
	limitToken    int
}

type SummaryType string

const (
	Stuff     SummaryType = "Stuff"
	MapReduce SummaryType = "MapReduce"
	Refine    SummaryType = "Refine"
)

func NewSummary(l llm.LLM, limitToken int) *Summary {
	if limitToken <= 0 {
		limitToken = DefaultSummaryLimitToken
	}
	return &Summary{
		log:           logger.NewLogger("summary"),
		llm:           l,
		summaryPrompt: prompts.NewSummaryPrompt(),
		combinePrompt: prompts.NewCombinePrompt(),
		limitToken:    limitToken,
	}
}

func (s *Summary) Summary(docs []string, summaryType SummaryType) (summary string, err error) {
	switch summaryType {
	case MapReduce:
		return s.MapReduce(docs)
	case Refine:
		// todo
		return "", err
	case Stuff:
		fallthrough
	default:
		return s.Stuff(docs)
	}
}