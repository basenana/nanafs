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
	"path"
	"path/filepath"
	"strings"

	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
)

const (
	PluginName    = "docloader"
	PluginVersion = "1.0"
)

var PluginSpec = types.PluginSpec{
	Name:    PluginName,
	Version: PluginVersion,
	Type:    types.TypeProcess,
}

type DocLoader struct{}

func (d DocLoader) Name() string {
	return PluginName
}

func (d DocLoader) Type() types.PluginType {
	return types.TypeProcess
}

func (d DocLoader) Version() string {
	return PluginVersion
}

func (d DocLoader) Run(ctx context.Context, request *pluginapi.Request) (*pluginapi.Response, error) {
	filePath := request.Parameter["file_path"]
	if filePath == "" {
		return pluginapi.NewFailedResponse("file_path is required"), nil
	}

	doc, err := d.loadDocument(ctx, request.WorkingPath, filePath)
	if err != nil {
		return pluginapi.NewFailedResponse(fmt.Sprintf("load document %s error: %s", filePath, err.Error())), nil
	}

	resp := pluginapi.NewResponseWithResult(map[string]any{
		"file_path": filePath,
		"document": map[string]any{
			"title":      doc.Title,
			"content":    doc.Content,
			"properties": doc.DocumentProperties,
		},
	})
	return resp, nil
}

func (d DocLoader) loadDocument(ctx context.Context, workdir, filePath string) (*FDocument, error) {
	var (
		entryPath   = path.Join(workdir, filePath)
		fileExt     = filepath.Ext(entryPath)
		p           Parser
		parseOption = map[string]string{}
	)

	switch fileExt {
	case ".pdf":
		p = buildInLoaders[pdfParser](entryPath, parseOption)
	case ".txt", ".md", ".markdown":
		p = buildInLoaders[textParser](entryPath, parseOption)
	case ".html", ".htm":
		p = buildInLoaders[htmlParser](entryPath, parseOption)
	case ".webarchive":
		p = buildInLoaders[webArchiveParser](entryPath, parseOption)
	case ".epub":
		p = buildInLoaders[epubParser](entryPath, parseOption)
	default:
		return nil, fmt.Errorf("load %s file unsupported", fileExt)
	}

	document, err := p.Load(ctx, types.DocumentProperties{})
	if err != nil {
		return nil, fmt.Errorf("load file %s failed: %w", entryPath, err)
	}

	// set default title
	if document.Title == "" {
		title := strings.TrimSpace(strings.TrimSuffix(filePath, fileExt))
		document.Title = title
	}

	return document, nil
}

func NewDocLoader() *DocLoader {
	return &DocLoader{}
}

type Parser interface {
	Load(ctx context.Context, doc types.DocumentProperties) (result *FDocument, err error)
}

type parserBuilder func(docPath string, docOption map[string]string) Parser

var (
	buildInLoaders = map[string]parserBuilder{
		textParser:       NewText,
		pdfParser:        NewPDF,
		htmlParser:       NewHTML,
		webArchiveParser: NewHTML,
		epubParser:       NewEPUB,
	}
)

type FDocument struct {
	types.DocumentProperties
	Content string
	Extra   map[string]string
}
