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

	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/hyponet/webpage-packer/packer"
	"go.uber.org/zap"
)

const (
	WebpackPluginName    = "webpack"
	WebpackPluginVersion = "1.0"

	packerPostMetaURL   = types.PropertyWebPageURL
	packerPostMetaTitle = types.PropertyWebPageTitle

	webpackParameterFilename    = "filename"
	webpackParameterFileType    = "fileType"
	webpackParameterURL         = "url"
	webpackParameterClutterFree = "clutter_free"
)

var WebpackPluginSpec = types.PluginSpec{
	Name:    WebpackPluginName,
	Version: WebpackPluginVersion,
	Type:    types.TypeProcess,
}

type WebpackPlugin struct{}

func (w *WebpackPlugin) Name() string {
	return WebpackPluginName
}

func (w *WebpackPlugin) Type() types.PluginType {
	return types.TypeProcess
}

func (w *WebpackPlugin) Version() string {
	return WebpackPluginVersion
}

func (w *WebpackPlugin) Run(ctx context.Context, request *pluginapi.Request) (*pluginapi.Response, error) {
	var (
		workdir     = request.WorkingPath
		filename    = pluginapi.GetParameter(webpackParameterFilename, request, "")
		urlInfo     = pluginapi.GetParameter(webpackParameterURL, request, "")
		fileType    = pluginapi.GetParameter(webpackParameterFileType, request, "webarchive")
		clutterFree = strings.ToLower(pluginapi.GetParameter(webpackParameterClutterFree, request, "true")) == "true"
		filePath    = path.Join(workdir, filename)
		logger      = pluginapi.Log(request, WebpackPluginName)
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

	if fileType == "" || (fileType != "html" && fileType != "webarchive") {
		return nil, fmt.Errorf("invalid file type [%s]", fileType)
	}

	result, err := w.packFromURL(ctx, filePath, urlInfo, fileType, clutterFree, logger)
	if err != nil {
		return pluginapi.NewFailedResponse(fmt.Sprintf("packing url %s failed: %s", urlInfo, err)), err
	}

	resp := pluginapi.NewResponseWithResult(result)
	return resp, nil
}

func (w *WebpackPlugin) packFromURL(ctx context.Context, filePath, urlInfo, tgtFileType string, clutterFree bool, logger *zap.SugaredLogger) (map[string]any, error) {

	var (
		filename = path.Base(filePath)
		title    = strings.TrimSuffix(filename, filepath.Ext(filename))
		err      error
	)

	if urlInfo == "" {
		return nil, fmt.Errorf("url is empty")
	}

	filename += "." + tgtFileType
	logger.Infof("packing url %s to %s", urlInfo, filename)

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
			logger.Warnw("pack to webarchive failed", "link", urlInfo, "err", err)
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
			logger.Warnw("pack to raw html file failed", "link", urlInfo, "err", err)
			return nil, fmt.Errorf("pack to html failed: %w", err)
		}
	}

	fInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat archive file error: %s", err)
	}
	return map[string]any{
		"file_path":         filename,
		"size":              fInfo.Size(),
		packerPostMetaTitle: title,
		packerPostMetaURL:   urlInfo,
	}, nil
}

func NewWebpackPlugin() *WebpackPlugin {
	return &WebpackPlugin{}
}
