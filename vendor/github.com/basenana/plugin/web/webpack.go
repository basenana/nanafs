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
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/basenana/plugin/api"
	"github.com/basenana/plugin/logger"
	"github.com/basenana/plugin/types"
	"github.com/hyponet/webpage-packer/packer"
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
}

type WebpackPlugin struct {
	logger      *zap.SugaredLogger
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
		workdir  = request.WorkingPath
		filename = api.GetStringParameter(webpackParameterFileName, request, "")
		urlInfo  = api.GetStringParameter(webpackParameterURL, request, "")
		filePath = path.Join(workdir, filename)
	)

	if workdir == "" {
		return nil, fmt.Errorf("workdir is empty")
	}

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

	result, err := w.packFromURL(ctx, filePath, urlInfo, w.fileType, w.clutterFree)
	if err != nil {
		w.logger.Warnw("packing failed", "url", urlInfo, "error", err)
		return api.NewFailedResponse(fmt.Sprintf("packing url %s failed: %s", urlInfo, err)), err
	}

	w.logger.Infow("webpack completed", "file_path", result["file_path"])

	resp := api.NewResponseWithResult(result)
	return resp, nil
}

func (w *WebpackPlugin) packFromURL(ctx context.Context, filePath, urlInfo, tgtFileType string, clutterFree bool) (map[string]any, error) {

	var (
		filename = path.Base(filePath)
		title    = strings.TrimSuffix(filename, filepath.Ext(filename))
		err      error
	)

	if urlInfo == "" {
		return nil, fmt.Errorf("url is empty")
	}

	filename += "." + tgtFileType
	w.logger.Infof("packing url %s to %s", urlInfo, filename)

	switch tgtFileType {
	case "webarchive":
		p := packer.NewWebArchivePacker()
		err = p.Pack(ctx, packer.Option{
			URL:              urlInfo,
			FilePath:         filePath,
			Timeout:          60,
			ClutterFree:      clutterFree,
			Headers:          make(map[string]string),
			EnablePrivateNet: enablePrivateNet,
		})
		if err != nil {
			w.logger.Warnw("pack to webarchive failed", "link", urlInfo, "err", err)
			return nil, fmt.Errorf("pack to webarchive failed: %w", err)
		}
	case "html":
		p := packer.NewHtmlPacker()
		err = p.Pack(ctx, packer.Option{
			URL:              urlInfo,
			FilePath:         filePath,
			Timeout:          60,
			ClutterFree:      clutterFree,
			Headers:          make(map[string]string),
			EnablePrivateNet: enablePrivateNet,
		})
		if err != nil {
			w.logger.Warnw("pack to raw html file failed", "link", urlInfo, "err", err)
			return nil, fmt.Errorf("pack to html failed: %w", err)
		}
	}

	fInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat archive file error: %s", err)
	}
	return map[string]any{
		"file_path": filename,
		"size":      fInfo.Size(),
		"title":     title,
		"url":       urlInfo,
	}, nil
}

var (
	enablePrivateNet = os.Getenv("WebPackerEnablePrivateNet") == "true"
)
