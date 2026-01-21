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
	"path/filepath"
	"strings"
	"time"

	"github.com/basenana/plugin/api"
	"github.com/basenana/plugin/logger"
	"github.com/basenana/plugin/types"
	"github.com/basenana/plugin/utils"
	"go.uber.org/zap"
)

const (
	PluginName    = "docloader"
	PluginVersion = "1.0"
)

var PluginSpec = types.PluginSpec{
	Name:    PluginName,
	Version: PluginVersion,
	Type:    types.TypeProcess,
	Parameters: []types.ParameterSpec{
		{
			Name:        "file_path",
			Required:    true,
			Description: "Path to document file",
		},
		{
			Name:        "title",
			Required:    false,
			Description: "Document title",
		},
		{
			Name:        "url",
			Required:    false,
			Description: "Document URL",
		},
		{
			Name:        "site_name",
			Required:    false,
			Description: "Site name",
		},
		{
			Name:        "site_url",
			Required:    false,
			Description: "Site URL",
		},
	},
}

type DocLoader struct {
	logger   *zap.SugaredLogger
	fileRoot *utils.FileAccess
}

func NewDocLoader(ps types.PluginCall) types.Plugin {
	return &DocLoader{
		logger:   logger.NewPluginLogger(PluginName, ps.JobID),
		fileRoot: utils.NewFileAccess(ps.WorkingPath),
	}
}

func (d *DocLoader) Name() string           { return PluginName }
func (d *DocLoader) Type() types.PluginType { return types.TypeProcess }
func (d *DocLoader) Version() string        { return PluginVersion }

func (d *DocLoader) Run(ctx context.Context, request *api.Request) (*api.Response, error) {
	filePath := api.GetStringParameter("file_path", request, "")
	if filePath == "" {
		return api.NewFailedResponse("file_path is required"), nil
	}

	d.logger.Infow("docloader started", "file_path", filePath)

	doc, err := d.loadDocument(ctx, filePath)
	if err != nil {
		d.logger.Warnw("load document failed", "file_path", filePath, "error", err)
		return api.NewFailedResponse(fmt.Sprintf("load document %s error: %s", filePath, err.Error())), nil
	}

	if title := api.GetStringParameter("title", request, ""); title != "" {
		doc.Properties.Title = title
	}
	if doc.Properties.URL == "" {
		doc.Properties.URL = api.GetStringParameter("url", request, "")
	}
	if doc.Properties.SiteName == "" {
		doc.Properties.SiteName = api.GetStringParameter("site_name", request, "")
	}
	if doc.Properties.SiteURL == "" {
		doc.Properties.SiteURL = api.GetStringParameter("site_url", request, "")
	}

	// Handle publish time from RSS updated_at parameter
	if doc.Properties.PublishAt == 0 {
		if updatedAtStr := api.GetStringParameter("updated_at", request, ""); updatedAtStr != "" {
			if t, err := time.Parse(time.RFC3339, updatedAtStr); err == nil {
				doc.Properties.PublishAt = t.Unix()
			}
		}
	}

	d.logger.Infow("docloader completed", "file_path", filePath, "title", doc.Properties.Title)

	resp := api.NewResponseWithResult(map[string]any{
		"file_path": filePath,
		"document":  utils.MarshalMap(doc),
	})
	return resp, nil
}

func (d *DocLoader) loadDocument(ctx context.Context, filePath string) (types.Document, error) {
	entryPath, err := d.fileRoot.GetAbsPath(filePath)
	if err != nil {
		return types.Document{}, fmt.Errorf("invalid file path: %w", err)
	}

	var (
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
		return types.Document{}, fmt.Errorf("load %s file unsupported", fileExt)
	}

	doc, err := p.Load(logger.IntoContext(ctx, d.logger))
	if err != nil {
		return types.Document{}, fmt.Errorf("load file %s failed: %w", entryPath, err)
	}

	if doc.Properties.Title == "" {
		doc.Properties.Title = strings.TrimSpace(strings.TrimSuffix(filePath, fileExt))
	}

	return doc, nil
}

type Parser interface {
	Load(ctx context.Context) (doc types.Document, err error)
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
