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
	"fmt"
	"strings"

	"github.com/basenana/friday/pkg/llm/prompts"
)

func (f *Friday) Question(q string) (string, error) {
	prompt := prompts.NewQuestionPrompt()
	c, err := f.searchDocs(q)
	if err != nil {
		return "", err
	}
	if f.LLM != nil {
		ans, err := f.LLM.Chat(prompt, map[string]string{
			"context":  c,
			"question": q,
		})
		if err != nil {
			return "", fmt.Errorf("llm completion error: %w", err)
		}
		f.Log.Debugf("Question result: %s", c)
		return ans[0], nil
	}
	return c, nil
}

func (f *Friday) searchDocs(q string) (string, error) {
	f.Log.Debugf("vector query for %s ...", q)
	qv, _, err := f.Embedding.VectorQuery(q)
	if err != nil {
		return "", fmt.Errorf("vector embedding error: %w", err)
	}
	contexts, err := f.Vector.Search(qv, defaultTopK)
	if err != nil {
		return "", fmt.Errorf("vector search error: %w", err)
	}

	cs := []string{}
	for _, c := range contexts {
		f.Log.Debugf("searched from [%s] for %s", c.Metadata["source"], c.Content)
		cs = append(cs, c.Content)
	}
	return strings.Join(cs, "\n"), nil
}
