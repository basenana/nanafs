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
	"context"

	"github.com/basenana/friday/pkg/llm"
	"github.com/basenana/friday/pkg/llm/prompts"
	"github.com/basenana/friday/pkg/utils/logger"
)

const (
	DefaultSummaryLimitToken = 4000
	combinePromptKey         = "combine"
	summaryPromptKey         = "summary"
)

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

func NewSummary(log logger.Logger, l llm.LLM, limitToken int, ps map[string]string) *Summary {
	if limitToken <= 0 {
		limitToken = DefaultSummaryLimitToken
	}
	return &Summary{
		log:           log,
		llm:           l,
		summaryPrompt: prompts.NewSummaryPrompt(ps[summaryPromptKey]),
		combinePrompt: prompts.NewCombinePrompt(ps[combinePromptKey]),
		limitToken:    limitToken,
	}
}

func (s *Summary) Summary(ctx context.Context, docs []string, summaryType SummaryType) (string, map[string]int, error) {
	switch summaryType {
	case MapReduce:
		return s.MapReduce(ctx, docs)
	case Refine:
		// todo
		return "", nil, nil
	case Stuff:
		fallthrough
	default:
		return s.Stuff(ctx, docs)
	}
}
