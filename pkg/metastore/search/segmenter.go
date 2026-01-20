/*
 Copyright 2024 Friday Author.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package search

import (
	"regexp"
	"strings"
	"sync"

	"github.com/hyponet/jiebago"
)

var (
	seg          jiebago.Segmenter
	segInit      sync.Once
	segErr       error
	dictPath     = "dict.txt"
	contentClean = regexp.MustCompile(`[^\p{Han}a-zA-Z0-9\s]`)
)

// SetDictPath sets the dictionary path for Chinese word segmentation.
// Must be called before first segmentation (e.g., before creating DB or querying).
// Default is "dict.txt".
func SetDictPath(path string) {
	dictPath = path
}

// splitTokens uses jiebago to segment content into tokens for Chinese word segmentation.
func splitTokens(content string) []string {
	segInit.Do(func() {
		segErr = seg.LoadDictionary(dictPath)
	})
	if segErr != nil {
		return []string{}
	}

	content = contentClean.ReplaceAllString(content, " ")
	contentCh := seg.CutForSearch(content, true)
	tokens := make([]string, 0, len(contentCh))
	for token := range contentCh {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		tokens = append(tokens, token)
	}

	return tokens
}
