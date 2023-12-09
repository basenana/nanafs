/*
 Copyright 2023 Friday Author.

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

package files

import (
	"fmt"
	"regexp"
	"strconv"
)

func Length(doc string) int {
	// todo: it should be more accurate
	// https://platform.openai.com/docs/guides/text-generation/managing-tokens

	// Match words and punctuation using regular expressions
	wordRegex := regexp.MustCompile(`\w+`)
	punctuationRegex := regexp.MustCompile(`[^\w\s]`)

	// Count the number of words
	words := wordRegex.FindAllString(doc, -1)
	wordCount := len(words)

	// Count the number of punctuation marks
	punctuation := punctuationRegex.FindAllString(doc, -1)
	punctuationCount := len(punctuation)

	return wordCount + punctuationCount
}

func Int64ToStr(s int64) string {
	return fmt.Sprintf("doc_%d", s)
}

func StrToInt64(s string) (int64, error) {
	return strconv.ParseInt(s[4:], 10, 64)
}
