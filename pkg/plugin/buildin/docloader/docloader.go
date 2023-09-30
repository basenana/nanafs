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
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
	"os"
	"path/filepath"
)

const (
	PluginName    = "docloader"
	PluginVersion = "1.0"
)

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

func (d DocLoader) Run(ctx context.Context, request *pluginapi.Request, pluginParams map[string]string) (*pluginapi.Response, error) {
	entryPath := request.Parameter[pluginapi.ResEntryPathKey].(string)
	if entryPath == "" {
		resp := pluginapi.NewFailedResponse("entry_path is empty")
		return resp, nil
	}

	_, err := os.Stat(entryPath)
	if err != nil {
		resp := pluginapi.NewFailedResponse(fmt.Sprintf("stat entry file %s failed: %s", entryPath, err))
		return resp, nil
	}

	fileExt := filepath.Ext(entryPath)
	var (
		p           Parser
		parseOption = map[string]string{}
	)

	switch fileExt {
	case ".pdf":
		p = buildInLoaders[pdfParser](entryPath, parseOption)
	case ".txt":
		p = buildInLoaders[textParser](entryPath, parseOption)
	case ".html", ".htm":
		p = buildInLoaders[htmlParser](entryPath, parseOption)
	case ".webarchive":
		p = buildInLoaders[webArchiveParser](entryPath, parseOption)
	default:
		resp := pluginapi.NewFailedResponse(fmt.Sprintf("load %s file unsupported", fileExt))
		return resp, nil
	}

	documents, err := p.Load(ctx)
	if err != nil {
		resp := pluginapi.NewFailedResponse(fmt.Sprintf("load file %s failed: %s", entryPath, err))
		return resp, nil
	}

	return pluginapi.NewResponseWithResult(map[string]any{pluginapi.ResEntryDocumentsKey: documents}), nil
}

type Parser interface {
	Load(ctx context.Context) (result []types.FDocument, err error)
}

type parserBuilder func(docPath string, docOption map[string]string) Parser

var (
	buildInLoaders = map[string]parserBuilder{
		textParser:       NewText,
		pdfParser:        NewPDF,
		htmlParser:       NewHTML,
		webArchiveParser: NewHTML,
	}
)
