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

package docloader

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/basenana/nanafs/pkg/types"
)

var (
	fileNamePatterns = []*regexp.Regexp{
		regexp.MustCompile(`^(.+)_([^_]+)_(\d{4})$`),
		regexp.MustCompile(`^(.+)[-_](.+)[- ]\((\d{4})\)$`),
	}
	yearRegex = regexp.MustCompile(`(\d{4})`)
)

func extractFileNameMetadata(docPath string, doc types.DocumentProperties) types.DocumentProperties {
	baseName := filepath.Base(docPath)
	ext := filepath.Ext(baseName)
	nameWithoutExt := strings.TrimSuffix(baseName, ext)

	for _, re := range fileNamePatterns {
		matches := re.FindStringSubmatch(nameWithoutExt)
		if matches != nil && len(matches) >= 4 {
			if doc.Author == "" {
				doc.Author = strings.TrimSpace(matches[1])
			}
			if doc.Title == "" {
				doc.Title = strings.TrimSpace(matches[2])
			}
			if doc.Year == "" {
				if _, err := strconv.Atoi(matches[3]); err == nil {
					doc.Year = matches[3]
				}
			}
			break
		}
	}

	if doc.Year == "" {
		if matches := yearRegex.FindStringSubmatch(nameWithoutExt); matches != nil {
			if _, err := strconv.Atoi(matches[1]); err == nil {
				doc.Year = matches[1]
			}
		}
	}

	return doc
}
