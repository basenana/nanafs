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
	"regexp"
	"strings"
)

var htmlCharFilterRegexp = regexp.MustCompile(`</?[!\w:]+((\s+[\w-]+(\s*=\s*(?:\\*".*?"|'.*?'|[^'">\s]+))?)+\s*|\s*)/?>`)

func ContentTrim(contentType, content string) string {
	switch contentType {
	case "html", "htm", "webarchive", ".webarchive":
		content = strings.ReplaceAll(content, "</p>", "</p>\n")
		content = strings.ReplaceAll(content, "</P>", "</P>\n")
		content = strings.ReplaceAll(content, "</div>", "</div>\n")
		content = strings.ReplaceAll(content, "</DIV>", "</DIV>\n")
		return string(htmlCharFilterRegexp.ReplaceAll([]byte(content), []byte{}))
	}
	return content
}
