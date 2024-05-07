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

package buildin

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/hyponet/webpage-packer/packer"
	"go.uber.org/zap"
	"gopkg.in/ini.v1"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	WebpackPluginName    = "webpack"
	WebpackPluginVersion = "1.0"
)

type WebpackPlugin struct {
	spec  types.PluginSpec
	scope types.PlugScope
	svc   Services
	log   *zap.SugaredLogger
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

func (w *WebpackPlugin) Run(ctx context.Context, request *pluginapi.Request) (*pluginapi.Response, error) {
	if request.EntryId <= 0 {
		return nil, fmt.Errorf("invalid entry id: %d", request.ParentEntryId)
	}

	entryPath := request.EntryPath
	if entryPath == "" {
		resp := pluginapi.NewFailedResponse("entry path is empty")
		return resp, nil
	}

	_, err := os.Stat(entryPath)
	if err != nil {
		resp := pluginapi.NewFailedResponse(fmt.Sprintf("stat entry file %s failed: %s", entryPath, err))
		return resp, nil
	}

	fileExt := filepath.Ext(entryPath)
	switch fileExt {
	case ".url":
		return w.packFromURL(ctx, request)
	default:
		return pluginapi.NewResponseWithResult(nil), nil
	}
}

func (w *WebpackPlugin) packFromURL(ctx context.Context, request *pluginapi.Request) (*pluginapi.Response, error) {
	urlFile, err := ini.Load(request.EntryPath)
	if err != nil {
		w.logger(ctx).Errorw("load url file failed", "file", request.EntryPath, "err", err)
		return nil, err
	}

	var (
		filename    = path.Base(request.EntryPath)
		urlInfo     = urlFile.Section("InternetShortcut").Key("URL").String()
		tgtFileType = urlFile.Section("NanaFSSection").Key("ArchiveType").In("", []string{"webarchive", "html"})
		clutterFree = urlFile.Section("NanaFSSection").Key("ClutterFree").In("true", []string{"true", "false"})
		cleanup     = urlFile.Section("NanaFSSection").Key("Cleanup").In("true", []string{"true", "false"})
	)

	if urlInfo == "" {
		return pluginapi.NewFailedResponse("target url is empty"), nil
	}
	filename = strings.TrimSuffix(filename, filepath.Ext(filename)) + "." + tgtFileType
	w.logger(ctx).Infof("packing url %s to %s", urlInfo, filename)

	filePath := utils.SafetyFilePathJoin(request.WorkPath, filename)

	switch tgtFileType {
	case "webarchive":
		p := packer.NewWebArchivePacker()
		err = p.Pack(ctx, packer.Option{
			URL:              urlInfo,
			FilePath:         filePath,
			Timeout:          60,
			ClutterFree:      clutterFree == "true",
			Headers:          make(map[string]string),
			EnablePrivateNet: enablePrivateNet,
		})
		if err != nil {
			w.logger(ctx).Warnw("pack to webarchive failed", "link", urlInfo, "err", err)
			return pluginapi.NewFailedResponse("pack to webarchive failed"), nil
		}
	case "html":
		p := packer.NewHtmlPacker()
		err = p.Pack(ctx, packer.Option{
			URL:              urlInfo,
			FilePath:         filePath,
			Timeout:          60,
			ClutterFree:      clutterFree == "true",
			Headers:          make(map[string]string),
			EnablePrivateNet: enablePrivateNet,
		})
		if err != nil {
			w.logger(ctx).Warnw("pack to raw html file failed", "link", urlInfo, "err", err)
			return pluginapi.NewFailedResponse("pack to html file failed"), nil
		}
	}

	fInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat archive file error: %s", err)
	}
	newEntries := []pluginapi.CollectManifest{{BaseEntry: request.ParentEntryId, NewFiles: []pluginapi.Entry{
		{Name: path.Base(filePath), Kind: types.FileKind(path.Base(filePath), types.RawKind), Size: fInfo.Size()}}}}
	result := map[string]any{pluginapi.ResCollectManifests: newEntries}
	if cleanup == "true" {
		result[pluginapi.ResEntryActionKey] = "cleanup"
	}
	return pluginapi.NewResponseWithResult(result), nil
}

func (r *WebpackPlugin) logger(ctx context.Context) *zap.SugaredLogger {
	return utils.WorkflowJobLogger(ctx, r.log)
}

func NewWebpackPlugin(spec types.PluginSpec, scope types.PlugScope, svc Services) (*WebpackPlugin, error) {
	return &WebpackPlugin{
		spec:  spec,
		scope: scope,
		svc:   svc,
		log:   logger.NewLogger("webpackPlugin"),
	}, nil
}
