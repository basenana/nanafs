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
	"fmt"
	"strings"

	"github.com/basenana/friday/pkg/llm/prompts"
	"github.com/basenana/friday/pkg/utils/files"
)

func (s *Summary) MapReduce(ctx context.Context, docs []string) (summary string, err error) {
	// map
	splitedSummaries, err := s.mapSummaries(ctx, docs)
	if err != nil {
		return "", err
	}
	if splitedSummaries == nil {
		return "", fmt.Errorf("fail to split summaries")
	}
	if len(splitedSummaries) == 1 {
		return splitedSummaries[0], nil
	}

	// reduce
	return s.reduce(ctx, splitedSummaries)
}

func (s *Summary) splitDocs(p prompts.PromptTemplate, docs []string) ([][]string, error) {
	collapseDocs := [][]string{}
	subDocs := []string{}

	for _, doc := range docs {
		subDocs = append(subDocs, doc)
		subLength, err := s.getLength(p, subDocs)
		if err != nil {
			return nil, err
		}
		if subLength > s.limitToken {
			if len(subDocs) == 1 {
				return nil, fmt.Errorf("a single part was longer than the context length, can not handle it")
			}
			collapseDocs = append(collapseDocs, subDocs[0:len(subDocs)-1])
			subDocs = subDocs[len(subDocs)-1:]
		}
	}
	collapseDocs = append(collapseDocs, subDocs)
	return collapseDocs, nil
}

func (s *Summary) getLength(p prompts.PromptTemplate, docs []string) (length int, err error) {
	doc := strings.Join(docs, "\n")
	res, err := p.String(map[string]string{"context": doc})
	if err != nil {
		return 0, err
	}

	ress := strings.Split(res, "\n")
	for _, r := range ress {
		length += files.Length(r)
	}
	return length, nil
}

func (s *Summary) mapSummaries(ctx context.Context, docs []string) ([]string, error) {
	newDocs, err := s.splitDocs(s.summaryPrompt, docs)
	if err != nil {
		return nil, err
	}
	s.log.Debugf("spilt docs to %d newDocs", len(newDocs))

	splitedSummaries := []string{}
	for _, splitedDocs := range newDocs {
		d, err := s.Stuff(ctx, splitedDocs)
		if err != nil {
			return nil, err
		}
		splitedSummaries = append(splitedSummaries, d)
	}
	return splitedSummaries, nil
}

func (s *Summary) reduce(ctx context.Context, summaries []string) (summary string, err error) {
	newSummaries, err := s.splitDocs(s.combinePrompt, summaries)
	if err != nil {
		return "", err
	}
	combinedSummaries := []string{}
	for _, subSummaries := range newSummaries {
		subSummary := strings.Join(subSummaries, "\n")
		res, err := s.llm.Chat(ctx, s.combinePrompt, map[string]string{"context": subSummary})
		if err != nil {
			return "", err
		}
		combinedSummaries = append(combinedSummaries, res[0])
	}

	if len(combinedSummaries) == 1 {
		return combinedSummaries[0], nil
	}
	return s.reduce(ctx, combinedSummaries)
}
