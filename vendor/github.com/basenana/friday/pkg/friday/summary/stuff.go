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
	"strings"
)

func (s *Summary) Stuff(ctx context.Context, docs []string) (summaries string, err error) {
	doc := strings.Join(docs, "\n")
	answers, err := s.llm.Chat(ctx, s.summaryPrompt, map[string]string{"context": doc})
	if err != nil {
		return "", err
	}
	return answers[0], nil
}
