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
	"github.com/basenana/friday/pkg/friday/summary"
	"github.com/basenana/friday/pkg/utils/files"
)

func (f *Friday) OfType(summaryType summary.SummaryType) *Friday {
	f.statement.summaryType = summaryType
	return f
}

func (f *Friday) Summary(res *SummaryState) *Friday {
	if f.statement.summaryType == "" {
		f.statement.summaryType = summary.Stuff
	}
	res.Summary = map[string]string{}
	res.Tokens = map[string]int{}
	// init
	s := summary.NewSummary(f.Log, f.LLM, f.LimitToken, f.Prompts)

	if f.statement.file != nil {
		// split doc
		docs := f.Spliter.Split(f.statement.file.Content)
		// Summary
		summaryOfFile, usage, err := s.Summary(f.statement.context, docs, f.statement.summaryType)
		if err != nil {
			f.Error = err
			return f
		}
		res.Summary = map[string]string{f.statement.file.Name: summaryOfFile}
		res.Tokens = usage
		return f
	}

	if f.statement.originFile != nil {
		fs, err := files.ReadFiles(*f.statement.originFile)
		if err != nil {
			f.Error = err
			return f
		}

		for name, file := range fs {
			// split doc
			subDocs := f.Spliter.Split(file)
			summaryOfFile, usage, err := s.Summary(f.statement.context, subDocs, f.statement.summaryType)
			if err != nil {
				f.Error = err
				return f
			}
			res.Summary[name] = summaryOfFile
			res.Tokens = mergeTokens(usage, res.Tokens)
		}
		return f
	}

	if len(f.statement.elements) != 0 {
		docs := make(map[string][]string)
		for _, element := range f.statement.elements {
			if _, ok := docs[element.Name]; !ok {
				docs[element.Name] = []string{element.Content}
			} else {
				docs[element.Name] = append(docs[element.Name], element.Content)
			}
		}
		for source, doc := range docs {
			summaryOfFile, usage, err := s.Summary(f.statement.context, doc, f.statement.summaryType)
			if err != nil {
				f.Error = err
				return f
			}
			res.Summary[source] = summaryOfFile
			res.Tokens = mergeTokens(usage, res.Tokens)
		}
		return f
	}

	return f
}
