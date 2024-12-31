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
	"time"

	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
)

const (
	PluginName    = "docloader"
	PluginVersion = "1.0"
)

var PluginSpec = types.PluginSpec{
	Name:          PluginName,
	Version:       PluginVersion,
	Type:          types.TypeProcess,
	Parameters:    make(map[string]string),
	Customization: []types.PluginConfig{},
}

type DocLoader struct {
	job   *types.WorkflowJob
	scope types.PlugScope
}

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
	var result = pluginapi.CollectManifest{BaseEntry: request.ParentEntryId}
	for i := range request.Entries {
		en := request.Entries[i]
		if err := d.loadEntry(ctx, request.WorkPath, &en); err != nil {
			return pluginapi.NewFailedResponse(fmt.Sprintf("load entry %d error: %s", en.ID, err.Error())), nil
		}

		if en.Document != nil {
			result.NewFiles = append(result.NewFiles, en)
		}
	}

	resp := pluginapi.NewResponse()
	resp.NewEntries = append(resp.NewEntries, result)
	return resp, nil
}

func (d DocLoader) loadEntry(ctx context.Context, workdir string, entry *pluginapi.Entry) error {
	var (
		entryPath   = path.Join(workdir, entry.Name)
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
	default:
		return fmt.Errorf("load %s file unsupported", fileExt)
	}

	document, err := p.Load(ctx)
	if err != nil {
		return fmt.Errorf("load file %s failed: %w", entryPath, err)
	}

	title := strings.TrimSpace(strings.TrimSuffix(entry.Name, fileExt))
	entry.Document = &pluginapi.Document{
		Title:    title,
		Content:  document.Content,
		PublicAt: document.PublicAt,
	}

	for k, v := range document.Metadata {
		entry.Parameters[k] = v
	}

	return nil
}

func NewDocLoader(job *types.WorkflowJob, scope types.PlugScope) *DocLoader {
	return &DocLoader{job: job, scope: scope}
}

type Parser interface {
	Load(ctx context.Context) (result *FDocument, err error)
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

type FDocument struct {
	Title    string
	Content  string
	PublicAt time.Time
	Metadata map[string]string
}
