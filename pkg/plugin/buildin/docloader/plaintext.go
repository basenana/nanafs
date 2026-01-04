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
	"bytes"
	"context"
	"io"
	"os"
	"strings"

	"github.com/basenana/nanafs/pkg/types"
)

const textParser = "text"

type Text struct {
	docPath string
}

func NewText(docPath string, option map[string]string) Parser {
	return Text{docPath: docPath}
}

func (l Text) Load(_ context.Context, doc types.DocumentProperties) (*FDocument, error) {
	f, err := os.Open(l.docPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, f); err != nil {
		return nil, err
	}

	doc = extractFileNameMetadata(l.docPath, doc)
	doc = extractTextContentMetadata(buf.String(), doc)

	if doc.PublishAt == 0 {
		if info, err := os.Stat(l.docPath); err == nil {
			doc.PublishAt = info.ModTime().Unix()
		}
	}

	return &FDocument{
		Content:            buf.String(),
		DocumentProperties: doc,
	}, nil
}

func extractTextContentMetadata(content string, doc types.DocumentProperties) types.DocumentProperties {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return doc
	}

	if doc.Title == "" {
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "# ") {
				doc.Title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
				break
			}
			if len(line) < 100 && !strings.ContainsAny(line, " \t") {
				continue
			}
			doc.Title = line
			break
		}
	}

	if doc.Abstract == "" {
		var paragraphs []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				if len(paragraphs) > 0 {
					break
				}
				continue
			}
			if strings.HasPrefix(line, "#") {
				continue
			}
			paragraphs = append(paragraphs, line)
			if len(paragraphs) >= 2 {
				break
			}
		}
		if len(paragraphs) > 0 {
			doc.Abstract = strings.Join(paragraphs, "\n")
		}
	}

	return doc
}
