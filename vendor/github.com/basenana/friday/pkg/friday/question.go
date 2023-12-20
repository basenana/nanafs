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
	"fmt"
	"strings"

	"github.com/basenana/friday/pkg/llm/prompts"
)

func (f *Friday) Question(ctx context.Context, parentId int64, q string) (string, map[string]int, error) {
	prompt := prompts.NewQuestionPrompt(f.Prompts[questionPromptKey])
	c, err := f.searchDocs(ctx, parentId, q)
	if err != nil {
		return "", nil, err
	}
	if f.LLM != nil {
		ans, usage, err := f.LLM.Completion(ctx, prompt, map[string]string{
			"context":  c,
			"question": q,
		})
		if err != nil {
			return "", nil, fmt.Errorf("llm completion error: %w", err)
		}
		f.Log.Debugf("Question result: %s", c)
		return ans[0], usage, nil
	}
	return c, nil, nil
}

func (f *Friday) searchDocs(ctx context.Context, parentId int64, q string) (string, error) {
	f.Log.Debugf("vector query for %s ...", q)
	qv, _, err := f.Embedding.VectorQuery(ctx, q)
	if err != nil {
		return "", fmt.Errorf("vector embedding error: %w", err)
	}
	docs, err := f.Vector.Search(ctx, parentId, qv, *f.VectorTopK)
	if err != nil {
		return "", fmt.Errorf("vector search error: %w", err)
	}

	cs := []string{}
	for _, c := range docs {
		f.Log.Debugf("searched from [%s] for %s", c.Name, c.Content)
		cs = append(cs, c.Content)
	}
	return strings.Join(cs, "\n"), nil
}
