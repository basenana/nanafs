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

package filewrite

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/basenana/plugin/api"
	"github.com/basenana/plugin/logger"
	"github.com/basenana/plugin/types"
	"github.com/basenana/plugin/utils"
	"go.uber.org/zap"
)

const (
	pluginName    = "filewrite"
	pluginVersion = "1.0"
)

var PluginSpec = types.PluginSpec{
	Name:    pluginName,
	Version: pluginVersion,
	Type:    types.TypeProcess,
}

type FileWritePlugin struct {
	logger   *zap.SugaredLogger
	fileRoot *utils.FileAccess
}

func NewFileWritePlugin(ps types.PluginCall) types.Plugin {
	return &FileWritePlugin{
		logger:   logger.NewPluginLogger(pluginName, ps.JobID),
		fileRoot: utils.NewFileAccess(ps.WorkingPath),
	}
}

func (p *FileWritePlugin) Name() string {
	return pluginName
}

func (p *FileWritePlugin) Type() types.PluginType {
	return types.TypeProcess
}

func (p *FileWritePlugin) Version() string {
	return pluginVersion
}

func (p *FileWritePlugin) Run(ctx context.Context, request *api.Request) (*api.Response, error) {
	content := api.GetStringParameter("content", request, "")
	destPath := api.GetStringParameter("dest_path", request, "")
	modeStr := api.GetStringParameter("mode", request, "0644")

	if destPath == "" {
		return api.NewFailedResponse("dest_path is required"), nil
	}

	// Parse mode
	mode, err := strconv.ParseUint(modeStr, 8, 32)
	if err != nil {
		return api.NewFailedResponse(fmt.Sprintf("invalid mode: %s", modeStr)), nil
	}

	p.logger.Infow("filewrite started", "dest_path", destPath, "mode", modeStr)

	// Get absolute path and create parent directories
	absPath, err := p.fileRoot.GetAbsPath(destPath)
	if err != nil {
		p.logger.Warnw("get absolute path failed", "dest_path", destPath, "error", err)
		return api.NewFailedResponse("write file failed: " + err.Error()), nil
	}

	// Create parent directory
	parentDir := filepath.Dir(absPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		p.logger.Warnw("create parent directory failed", "parent_dir", parentDir, "error", err)
		return api.NewFailedResponse("write file failed: " + err.Error()), nil
	}

	// Write file
	if err := os.WriteFile(absPath, []byte(content), os.FileMode(mode)); err != nil {
		p.logger.Warnw("write file failed", "dest_path", destPath, "error", err)
		return api.NewFailedResponse("write file failed: " + err.Error()), nil
	}

	p.logger.Infow("filewrite completed", "dest_path", destPath)
	return api.NewResponse(), nil
}
