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
	"bytes"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var repeatSpace = regexp.MustCompile(`\s+`)
var htmlCharFilterRegexp = regexp.MustCompile(`</?[!\w:]+((\s+[\w-]+(\s*=\s*(?:\\*".*?"|'.*?'|[^'">\s]+))?)+\s*|\s*)/?>`)

func ContentTrim(contentType, content string) string {
	switch contentType {
	case "html", "htm", "webarchive", ".webarchive":
		content = strings.ReplaceAll(content, "</p>", "</p>\n")
		content = strings.ReplaceAll(content, "</P>", "</P>\n")
		content = strings.ReplaceAll(content, "</div>", "</div>\n")
		content = strings.ReplaceAll(content, "</DIV>", "</DIV>\n")
		content = htmlCharFilterRegexp.ReplaceAllString(content, "")
	}
	content = repeatSpace.ReplaceAllString(content, " ")
	return content
}

func GenerateContentAbstract(content string) string {
	query, err := goquery.NewDocumentFromReader(bytes.NewReader([]byte(content)))
	if err != nil {
		return ""
	}

	query.Find("script, style, noscript, iframe, nav, header, footer, aside").Remove()

	contents := make([]string, 0)
	query.Find("p, article, section, li, td, th").EachWithBreak(func(i int, selection *goquery.Selection) bool {
		if len(contents) > 10 {
			return false
		}
		t := selection.Text()
		t = strings.ReplaceAll(t, "\n", " ")
		t = strings.TrimSpace(t)
		if len(t) > 5 {
			contents = append(contents, t)
		}
		return true
	})

	if len(contents) > 0 {
		if abs := trimDocumentContent(strings.Join(contents, " "), 400); len([]rune(abs)) > 100 {
			return abs
		}
	}

	bodyContent := query.Find("body").Text()
	return trimDocumentContent(bodyContent, 400)
}

func trimDocumentContent(str string, m int) string {
	str = ContentTrim("html", str)
	runes := []rune(str)
	if len(runes) > m {
		return strings.TrimSpace(string(runes[:m]))
	}
	return strings.TrimSpace(str)
}

func GenerateContentHeaderImage(content string) string {
	query, err := goquery.NewDocumentFromReader(bytes.NewReader([]byte(content)))
	if err != nil {
		return ""
	}

	var imageURL string
	query.Find("img").EachWithBreak(func(i int, selection *goquery.Selection) bool {
		var (
			srcVal    string
			isExisted bool
		)
		srcVal, isExisted = selection.Attr("src")
		if isExisted && isValidImageURL(srcVal) {
			imageURL = srcVal
			return false
		}

		srcVal, isExisted = selection.Attr("data-src")
		if isExisted && isValidImageURL(srcVal) {
			imageURL = srcVal
			return false
		}

		srcVal, isExisted = selection.Attr("data-src-retina")
		if isExisted && isValidImageURL(srcVal) {
			imageURL = srcVal
			return false
		}

		srcVal, isExisted = selection.Attr("data-original")
		if isExisted && isValidImageURL(srcVal) {
			imageURL = srcVal
			return false
		}
		return true
	})

	return imageURL
}

func isValidImageURL(urlVal string) bool {
	if urlVal == "" {
		return false
	}
	if !strings.HasPrefix(urlVal, "http://") && !strings.HasPrefix(urlVal, "https://") {
		return false
	}
	imageExtRegex := regexp.MustCompile(`(?i)\.(jpg|jpeg|png|gif|webp|bmp|tiff|heic|heif|avif)(\?.*)?$`)
	return imageExtRegex.MatchString(urlVal)
}
