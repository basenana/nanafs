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
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/hyponet/webpage-packer/packer"
	"strings"
)

const (
	htmlParser       = "html"
	webArchiveParser = "webarchive"
)

type HTML struct {
	docPath string
}

func NewHTML(docPath string, option map[string]string) Parser {
	return HTML{docPath: docPath}
}

func (h HTML) Load(ctx context.Context, doc types.DocumentProperties) (result *FDocument, err error) {
	var (
		p packer.Packer
	)
	switch {
	case strings.HasSuffix(h.docPath, ".webarchive"):
		p = packer.NewWebArchivePacker()
	case strings.HasSuffix(h.docPath, ".html") ||
		strings.HasSuffix(h.docPath, ".htm"):
		p = packer.NewHtmlPacker()
	default:
		return nil, fmt.Errorf("unsupported document type: %s", h.docPath)
	}

	content, err := p.ReadContent(ctx, packer.Option{
		FilePath:    h.docPath,
		ClutterFree: true,
	})
	if err != nil {
		return nil, err
	}

	doc.Abstract = utils.GenerateContentAbstract(content)
	doc.HeaderImage = utils.GenerateContentHeaderImage(content)
	return &FDocument{
		Content:            content,
		DocumentProperties: doc,
	}, nil
}
