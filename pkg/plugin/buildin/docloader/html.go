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
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/hyponet/webpage-packer/packer"
)

const (
	htmlParser       = "html"
	webArchiveParser = "webarchive"
)

var metaContentRegex = regexp.MustCompile(`<meta\s+(?:[^>]*?\s+)?(name|property)=["']([^"']+)["'][^>]*?content=["']([^"']*)["']`)

type HTML struct {
	docPath string
}

func NewHTML(docPath string, option map[string]string) Parser {
	return HTML{docPath: docPath}
}

func (h HTML) Load(ctx context.Context, doc types.DocumentProperties) (*FDocument, error) {
	var p packer.Packer
	switch {
	case strings.HasSuffix(h.docPath, ".webarchive"):
		p = packer.NewWebArchivePacker()
	case strings.HasSuffix(h.docPath, ".html") || strings.HasSuffix(h.docPath, ".htm"):
		p = packer.NewHtmlPacker()
	default:
		return nil, fmt.Errorf("unsupported document type: %s", h.docPath)
	}

	doc = extractHTMLMetadata(h.docPath, doc)

	content, err := p.ReadContent(ctx, packer.Option{
		FilePath:    h.docPath,
		ClutterFree: true,
	})
	if err != nil {
		return nil, err
	}

	if doc.Abstract == "" {
		doc.Abstract = utils.GenerateContentAbstract(content)
	}
	if doc.HeaderImage == "" {
		doc.HeaderImage = utils.GenerateContentHeaderImage(content)
	}
	if doc.PublishAt == 0 {
		if info, err := os.Stat(h.docPath); err == nil {
			doc.PublishAt = info.ModTime().Unix()
		}
	}
	return &FDocument{
		Content:            content,
		DocumentProperties: doc,
	}, nil
}

func extractHTMLMetadata(docPath string, doc types.DocumentProperties) types.DocumentProperties {
	f, err := os.Open(docPath)
	if err != nil {
		return doc
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return doc
	}
	content := string(data)

	mapping := map[string]string{
		"dc.title":       "Title",
		"dc.creator":     "Author",
		"dc.description": "Abstract",
		"dc.subject":     "Keywords",
		"dc.publisher":   "Source",
		"dc.date":        "PublishAt",
		"og:title":       "Title",
		"og:description": "Abstract",
		"og:site_name":   "Source",
		"og:image":       "HeaderImage",
		"author":         "Author",
		"description":    "Abstract",
		"keywords":       "Keywords",
		"site_name":      "Source",
	}

	if titleMatch := regexp.MustCompile(`<title[^>]*>([^<]+)</title>`).FindStringSubmatch(content); titleMatch != nil && doc.Title == "" {
		doc.Title = strings.TrimSpace(titleMatch[1])
	}

	for _, match := range metaContentRegex.FindAllStringSubmatch(content, -1) {
		if len(match) < 4 {
			continue
		}
		attrType, metaName, metaContent := match[1], match[2], match[3]
		key := metaName
		if attrType == "property" {
			key = "og:" + metaName
		}
		fieldName, ok := mapping[key]
		if !ok {
			continue
		}

		switch fieldName {
		case "Title":
			if doc.Title == "" {
				doc.Title = metaContent
			}
		case "Author":
			if doc.Author == "" {
				doc.Author = metaContent
			}
		case "Abstract":
			if doc.Abstract == "" {
				doc.Abstract = metaContent
			}
		case "Keywords":
			for _, k := range regexp.MustCompile(`[,;]`).Split(metaContent, -1) {
				k = strings.TrimSpace(k)
				if k != "" {
					doc.Keywords = append(doc.Keywords, k)
				}
			}
		case "Source":
			if doc.Source == "" {
				doc.Source = metaContent
			}
		case "HeaderImage":
			if doc.HeaderImage == "" {
				doc.HeaderImage = metaContent
			}
		}
	}
	return doc
}
