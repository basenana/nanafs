/*
 Copyright 2023 NanaFS Authors.

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

package indexer

import (
	"regexp"
	"strings"

	"github.com/hyponet/jiebago"
)

type Tokenizer interface {
	Tokenize(content string) []string
}

var contentClean = regexp.MustCompile(`[^\p{Han}a-zA-Z0-9\s]`)

type jiebaTokenizer struct {
	jiebaSeg *jiebago.Segmenter
}

func NewJiebaTokenizer(dictPath string) (Tokenizer, error) {
	seg := jiebago.Segmenter{}
	err := seg.LoadDictionary(dictPath)
	if err != nil {
		return nil, err
	}

	return &jiebaTokenizer{jiebaSeg: &seg}, nil
}

func (j *jiebaTokenizer) Tokenize(content string) []string {
	content = contentClean.ReplaceAllString(content, " ")
	contentCh := j.jiebaSeg.CutForSearch(content, true)
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

type spaceTokenizer struct{}

func NewSpaceTokenizer() Tokenizer {
	return &spaceTokenizer{}
}

func (s *spaceTokenizer) Tokenize(content string) []string {
	content = contentClean.ReplaceAllString(content, " ")
	content = strings.ToLower(content)
	tokens := strings.Split(content, " ")
	result := make([]string, 0, len(tokens))
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		result = append(result, token)
	}
	return result
}
