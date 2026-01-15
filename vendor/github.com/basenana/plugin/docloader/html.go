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
	"context"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/basenana/plugin/types"
	"github.com/basenana/plugin/utils"
	"github.com/basenana/plugin/web"
)

const (
	htmlParser       = "html"
	webArchiveParser = "webarchive"
)

var metaContentRegex = regexp.MustCompile(`<meta\s+(?:[^>]*?\s+)?(name|property)=["']([^"']+)["'][^>]*?content=["']([^"']*)["'][^>]*?>`)

type HTML struct {
	docPath string
}

func NewHTML(docPath string, option map[string]string) Parser {
	return HTML{docPath: docPath}
}

func (h HTML) Load(ctx context.Context) (types.Document, error) {
	props := extractHTMLMetadata(h.docPath)
	content, err := web.ReadFromFile(ctx, h.docPath)
	if err != nil {
		return types.Document{}, err
	}

	if props.Abstract == "" {
		props.Abstract = utils.GenerateContentAbstract(content)
	}
	if props.HeaderImage == "" {
		props.HeaderImage = utils.GenerateContentHeaderImage(content)
	}
	if props.PublishAt == 0 {
		if info, err := os.Stat(h.docPath); err == nil {
			props.PublishAt = info.ModTime().Unix()
		}
	}

	return types.Document{
		Content:    ReadableHTMLContent(content),
		Properties: props,
	}, nil
}

func extractHTMLMetadata(docPath string) types.Properties {
	props := types.Properties{}
	f, err := os.Open(docPath)
	if err != nil {
		return props
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return props
	}
	content := string(data)

	// Track which fields have been set (non-OG tags only set if empty)
	set := map[string]bool{}

	// Process meta tags - OG tags can override, others only if empty
	matches := metaContentRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		_, metaName, metaContent := match[1], match[2], match[3]

		// OG tags override everything, others only set if empty
		isOGTag := strings.HasPrefix(metaName, "og:")

		switch metaName {
		case "dc.title", "og:title":
			if isOGTag || !set["title"] {
				props.Title = metaContent
				set["title"] = true
			}
		case "dc.creator", "author":
			if isOGTag || !set["author"] {
				props.Author = metaContent
				set["author"] = true
			}
		case "dc.description", "og:description", "description":
			if isOGTag || !set["abstract"] {
				props.Abstract = metaContent
				set["abstract"] = true
			}
		case "dc.subject", "keywords":
			var keywords []string
			for _, k := range regexp.MustCompile(`[,;]`).Split(metaContent, -1) {
				k = strings.TrimSpace(k)
				if k != "" {
					keywords = append(keywords, k)
				}
			}
			if len(keywords) > 0 {
				props.Keywords = keywords
			}
		case "dc.publisher", "og:site_name", "site_name":
			if isOGTag || !set["source"] {
				props.Source = metaContent
				set["source"] = true
			}
		case "dc.date":
			if t, err := strconv.ParseInt(metaContent, 10, 64); err == nil {
				if isOGTag || !set["publish_at"] {
					props.PublishAt = t
					set["publish_at"] = true
				}
			}
		case "og:image":
			if isOGTag || !set["header_image"] {
				props.HeaderImage = metaContent
				set["header_image"] = true
			}
		}
	}

	// HTML title tag as fallback if no OG title
	if props.Title == "" {
		if titleMatch := regexp.MustCompile(`<title[^>]*>([^<]+)</title>`).FindStringSubmatch(content); titleMatch != nil {
			props.Title = strings.TrimSpace(titleMatch[1])
		}
	}

	return props
}

func ReadableHTMLContent(content string) string {
	markdown, err := htmltomarkdown.ConvertString(content)
	if err != nil {
		return HTMLContentTrim(content)
	}
	return markdown
}

var repeatSpace = regexp.MustCompile(`\s+`)
var htmlCharFilterRegexp = regexp.MustCompile(`</?[!\w:]+((\s+[\w-]+(\s*=\s*(?:\\*".*?"|'.*?'|[^'">\s]+))?)+\s*|\s*)/?>`)

func HTMLContentTrim(content string) string {
	content = strings.ReplaceAll(content, "</p>", "</p>\n")
	content = strings.ReplaceAll(content, "</P>", "</P>\n")
	content = strings.ReplaceAll(content, "</div>", "</div>\n")
	content = strings.ReplaceAll(content, "</DIV>", "</DIV>\n")
	content = htmlCharFilterRegexp.ReplaceAllString(content, "")
	content = repeatSpace.ReplaceAllString(content, " ")
	return content
}
