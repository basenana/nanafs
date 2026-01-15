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

package utils

import (
	"os"
	"path"
	"regexp"
	"strings"
)

const (
	PathSeparator = string(os.PathSeparator)
)

var (
	fileNameSafety = regexp.MustCompile(`[\\/:*?"<>|]`)
)

func SafetyFilePathJoin(parent, filename string) string {
	safeFilename := SanitizeFilename(filename)
	return path.Clean(path.Join(parent, safeFilename))
}

func SanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, " ", "_")
	name = fileNameSafety.ReplaceAllString(name, " ")
	if len(name) > 100 {
		name = name[:100]
	}
	return name
}
