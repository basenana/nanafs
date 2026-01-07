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
	"archive/zip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/basenana/plugin/types"
)

const epubParser = "epub"

type EPUB struct {
	docPath string
}

func NewEPUB(docPath string, option map[string]string) Parser {
	return EPUB{docPath: docPath}
}

func (e EPUB) Load(_ context.Context) (types.Document, error) {
	r, err := zip.OpenReader(e.docPath)
	if err != nil {
		return types.Document{}, err
	}
	defer r.Close()

	var opfPath string
	var opfData []byte

	for _, file := range r.File {
		if file.Name == "META-INF/container.xml" {
			rc, err := file.Open()
			if err != nil {
				continue
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}

			var container struct {
				Rootfiles []struct {
					FullPath string `xml:"full-path,attr"`
				} `xml:"rootfiles>rootfile"`
			}
			if xml.Unmarshal(data, &container) == nil {
				for _, rf := range container.Rootfiles {
					if strings.HasSuffix(rf.FullPath, ".opf") {
						opfPath = rf.FullPath
						break
					}
				}
			}
			break
		}
	}

	if opfPath == "" {
		return types.Document{}, fmt.Errorf("EPUB: could not find OPF file")
	}

	for _, file := range r.File {
		if file.Name == opfPath {
			rc, err := file.Open()
			if err != nil {
				return types.Document{}, fmt.Errorf("EPUB: failed to open OPF file: %w", err)
			}
			opfData, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return types.Document{}, fmt.Errorf("EPUB: failed to read OPF file: %w", err)
			}
			break
		}
	}

	if opfData == nil {
		return types.Document{}, fmt.Errorf("EPUB: OPF file not found")
	}

	var pkg struct {
		Metadata struct {
			DC []struct {
				Name string `xml:"name,attr"`
				Text string `xml:",chardata"`
			} `xml:"dc>element"`
		} `xml:"metadata"`
		Manifest struct {
			Items []struct {
				ID        string `xml:"id,attr"`
				HRef      string `xml:"href,attr"`
				MediaType string `xml:"media-type,attr"`
			} `xml:"item"`
		} `xml:"manifest"`
		Spine struct {
			Items []struct {
				IDRef string `xml:"idref,attr"`
			} `xml:"itemref"`
		} `xml:"spine"`
	}
	if err := xml.Unmarshal(opfData, &pkg); err != nil {
		return types.Document{}, fmt.Errorf("EPUB: failed to parse OPF file: %w", err)
	}

	props := types.Properties{}

	for _, elem := range pkg.Metadata.DC {
		text := strings.TrimSpace(elem.Text)
		if text == "" {
			continue
		}

		switch elem.Name {
		case "title":
			if props.Title == "" {
				props.Title = text
			}
		case "creator":
			if props.Author == "" {
				props.Author = text
			}
		case "description":
			if props.Abstract == "" {
				props.Abstract = text
			}
		case "subject":
			var keywords []string
			for _, k := range regexp.MustCompile(`[,;]`).Split(text, -1) {
				k = strings.TrimSpace(k)
				if k != "" {
					keywords = append(keywords, k)
				}
			}
			if len(keywords) > 0 {
				props.Keywords = keywords
			}
		case "publisher":
			if props.Source == "" {
				props.Source = text
			}
		case "date":
			if t, err := strconv.ParseInt(text, 10, 64); err == nil && props.PublishAt == 0 {
				props.PublishAt = t
			}
		}
	}

	manifest := make(map[string]struct {
		HRef string
	})
	for _, item := range pkg.Manifest.Items {
		manifest[item.ID] = struct{ HRef string }{item.HRef}
	}

	opfDir := ""
	if idx := strings.LastIndex(opfPath, "/"); idx >= 0 {
		opfDir = opfPath[:idx+1]
	}

	var content strings.Builder
	for _, itemref := range pkg.Spine.Items {
		if item, ok := manifest[itemref.IDRef]; ok {
			if strings.HasSuffix(item.HRef, ".xhtml") || strings.HasSuffix(item.HRef, ".html") {
				contentPath := opfDir + item.HRef
				for _, file := range r.File {
					if file.Name == contentPath {
						rc, _ := file.Open()
						data, _ := io.ReadAll(rc)
						rc.Close()
						content.WriteString(stripHTMLTags(string(data)))
						content.WriteString("\n\n")
						break
					}
				}
			}
		}
	}

	if props.PublishAt == 0 {
		if info, err := os.Stat(e.docPath); err == nil {
			props.PublishAt = info.ModTime().Unix()
		}
	}

	return types.Document{
		Content:    content.String(),
		Properties: props,
	}, nil
}

func stripHTMLTags(html string) string {
	scriptRegex := regexp.MustCompile(`(?s)<script[^>]*>.*?</script>`)
	html = scriptRegex.ReplaceAllString(html, "")
	styleRegex := regexp.MustCompile(`(?s)<style[^>]*>.*?</style>`)
	html = styleRegex.ReplaceAllString(html, "")
	tagRegex := regexp.MustCompile(`<[^>]+>`)
	html = tagRegex.ReplaceAllString(html, "\n")
	wsRegex := regexp.MustCompile(`[ \t]+`)
	html = wsRegex.ReplaceAllString(html, " ")

	var result []string
	for _, line := range strings.Split(html, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}
