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
	"strings"

	"github.com/basenana/plugin/api"
	"github.com/basenana/plugin/logger"
	"github.com/basenana/plugin/types"
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
	logger *zap.SugaredLogger
}

func NewFileWritePlugin(ps types.PluginCall) types.Plugin {
	return &FileWritePlugin{
		logger: logger.NewPluginLogger(pluginName, ps.JobID),
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

	// Ensure parent directory exists
	parentDir := filepath.Dir(destPath)
	if parentDir != "" && parentDir != "." {
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return api.NewFailedResponse("create directory failed: " + err.Error()), nil
		}
	}

	p.logger.Infow("filewrite started", "dest_path", destPath, "mode", modeStr)

	// Write file
	if err := os.WriteFile(destPath, []byte(content), os.FileMode(mode)); err != nil {
		p.logger.Warnw("write file failed", "dest_path", destPath, "error", err)
		return api.NewFailedResponse("write file failed: " + err.Error()), nil
	}

	p.logger.Infow("filewrite completed", "dest_path", destPath)
	return api.NewResponse(), nil
}

func ResolvePath(path string, workingPath string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	return filepath.Join(workingPath, path), nil
}

func SanitizePath(path string) (string, error) {
	// Remove any null bytes or path traversal attempts
	path = strings.ReplaceAll(path, "\x00", "")
	path = filepath.Clean(path)

	// Check for path traversal
	if strings.Contains(path, "..") {
		return "", fmt.Errorf("path contains invalid traversal")
	}

	return path, nil
}
