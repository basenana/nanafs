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

package web

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/basenana/plugin/api"
	"github.com/basenana/plugin/logger"
	"github.com/basenana/plugin/types"
	"github.com/basenana/plugin/utils"
	"go.uber.org/zap"
)

const (
	WebpackPluginName    = "webpack"
	WebpackPluginVersion = "1.0"

	webpackParameterFileName    = "file_name"
	webpackParameterFileType    = "file_type"
	webpackParameterURL         = "url"
	webpackParameterClutterFree = "clutter_free"
)

var WebpackPluginSpec = types.PluginSpec{
	Name:    WebpackPluginName,
	Version: WebpackPluginVersion,
	Type:    types.TypeProcess,
	InitParameters: []types.ParameterSpec{
		{
			Name:        "file_type",
			Required:    false,
			Default:     "webarchive",
			Description: "Output format: html, webarchive",
			Options:     []string{"html", "webarchive"},
		},
		{
			Name:        "clutter_free",
			Required:    false,
			Default:     "true",
			Description: "Enable clutter-free mode",
			Options:     []string{"true", "false"},
		},
	},
	Parameters: []types.ParameterSpec{
		{
			Name:        "file_name",
			Required:    true,
			Description: "Output file name",
		},
		{
			Name:        "url",
			Required:    true,
			Description: "URL to pack",
		},
	},
}

type WebpackPlugin struct {
	logger      *zap.SugaredLogger
	fileRoot    *utils.FileAccess
	fileType    string
	clutterFree bool
}

func NewWebpackPlugin(ps types.PluginCall) types.Plugin {
	fileType := ps.Params[webpackParameterFileType]
	if fileType == "" {
		fileType = "webarchive"
	}

	clutterFree := true
	if v, ok := ps.Params[webpackParameterClutterFree]; ok {
		clutterFree = v == "true" || v == "1"
	}

	return &WebpackPlugin{
		logger:      logger.NewPluginLogger(WebpackPluginName, ps.JobID),
		fileRoot:    utils.NewFileAccess(ps.WorkingPath),
		fileType:    fileType,
		clutterFree: clutterFree,
	}
}

func (w *WebpackPlugin) Name() string {
	return WebpackPluginName
}

func (w *WebpackPlugin) Type() types.PluginType {
	return types.TypeProcess
}

func (w *WebpackPlugin) Version() string {
	return WebpackPluginVersion
}

func (w *WebpackPlugin) Run(ctx context.Context, request *api.Request) (*api.Response, error) {
	var (
		filename = api.GetStringParameter(webpackParameterFileName, request, "")
		urlInfo  = api.GetStringParameter(webpackParameterURL, request, "")
	)

	if filename == "" {
		return nil, fmt.Errorf("file name is empty")
	}

	if urlInfo == "" {
		return nil, fmt.Errorf("url is empty")
	}

	if w.fileType == "" || (w.fileType != "html" && w.fileType != "webarchive") {
		return nil, fmt.Errorf("invalid file type [%s]", w.fileType)
	}

	w.logger.Infow("webpack started", "url", urlInfo, "file_type", w.fileType)

	result, err := w.packFromURL(ctx, filename, urlInfo, w.fileType, w.clutterFree)
	if err != nil {
		w.logger.Warnw("packing failed", "url", urlInfo, "error", err)
		return api.NewFailedResponse(fmt.Sprintf("packing url %s failed: %s", urlInfo, err)), err
	}

	w.logger.Infow("webpack completed", "file_path", result["file_path"])

	resp := api.NewResponseWithResult(result)
	return resp, nil
}

func (w *WebpackPlugin) packFromURL(ctx context.Context, filename, urlInfo, tgtFileType string, clutterFree bool) (map[string]any, error) {
	title := strings.TrimSuffix(filename, filepath.Ext(filename))

	if urlInfo == "" {
		return nil, fmt.Errorf("url is empty")
	}

	filePath, err := PackFromURL(logger.IntoContext(ctx, w.logger), filename, urlInfo, tgtFileType, w.fileRoot.Workdir(), clutterFree)
	if err != nil {
		return nil, err
	}

	fInfo, err := w.fileRoot.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat archive file error: %s", err)
	}
	return map[string]any{
		"file_path": filePath,
		"size":      fInfo.Size(),
		"title":     title,
		"url":       urlInfo,
	}, nil
}
