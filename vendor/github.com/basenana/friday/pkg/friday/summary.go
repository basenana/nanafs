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

package friday

import (
	"context"

	"github.com/basenana/friday/pkg/friday/summary"
	"github.com/basenana/friday/pkg/models"
	"github.com/basenana/friday/pkg/utils/files"
)

func (f *Friday) Summary(ctx context.Context, elements []models.Element, summaryType summary.SummaryType) (map[string]string, error) {
	result := make(map[string]string)
	s := summary.NewSummary(f.LLM, f.LimitToken)

	docs := make(map[string][]string)
	for _, element := range elements {
		if _, ok := docs[element.Metadata.Source]; !ok {
			docs[element.Metadata.Source] = []string{element.Content}
		} else {
			docs[element.Metadata.Source] = append(docs[element.Metadata.Source], element.Content)
		}
	}
	for source, doc := range docs {
		summaryOfFile, err := s.Summary(ctx, doc, summaryType)
		if err != nil {
			return nil, err
		}
		result[source] = summaryOfFile
	}
	f.Log.Debugf("Summary result: %s", result)
	return result, nil
}

func (f *Friday) SummaryFromFile(ctx context.Context, file models.File, summaryType summary.SummaryType) (map[string]string, error) {
	s := summary.NewSummary(f.LLM, f.LimitToken)
	// split doc
	docs := f.Spliter.Split(file.Content)
	// summary
	summaryOfFile, err := s.Summary(ctx, docs, summaryType)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		file.Source: summaryOfFile,
	}, err
}

func (f *Friday) SummaryFromOriginFile(ctx context.Context, ps string, summaryType summary.SummaryType) (map[string]string, error) {
	s := summary.NewSummary(f.LLM, f.LimitToken)
	fs, err := files.ReadFiles(ps)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for name, file := range fs {
		// split doc
		subDocs := f.Spliter.Split(file)
		summaryOfFile, err := s.Summary(ctx, subDocs, summaryType)
		if err != nil {
			return nil, err
		}
		result[name] = summaryOfFile
	}

	return result, nil
}
