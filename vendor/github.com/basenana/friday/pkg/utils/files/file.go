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

package files

import (
	"os"
	"path/filepath"
	"strings"
)

// ReadFiles read files recursively for only txt and md
func ReadFiles(ps string) (docs map[string]string, err error) {
	var p os.FileInfo
	docs = map[string]string{}
	p, err = os.Stat(ps)
	if err != nil {
		return
	}
	if p.IsDir() {
		var subFiles []os.DirEntry
		subFiles, err = os.ReadDir(ps)
		if err != nil {
			return
		}
		for _, subFile := range subFiles {
			subDocs := make(map[string]string)
			subDocs, err = ReadFiles(filepath.Join(ps, subFile.Name()))
			if err != nil {
				return
			}
			for k, v := range subDocs {
				docs[k] = v
			}
		}
		return
	}
	if !strings.HasSuffix(p.Name(), ".md") && !strings.HasSuffix(p.Name(), ".txt") {
		return
	}
	doc, err := os.ReadFile(ps)
	if err != nil {
		return
	}
	docs[ps] = string(doc)
	return
}
