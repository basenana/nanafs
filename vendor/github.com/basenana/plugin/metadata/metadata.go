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

package metadata

import (
	"context"
	"time"

	"github.com/basenana/plugin/api"
	"github.com/basenana/plugin/logger"
	"github.com/basenana/plugin/types"
	"github.com/basenana/plugin/utils"
	"go.uber.org/zap"
)

const (
	pluginName    = "metadata"
	pluginVersion = "1.0"
)

var PluginSpec = types.PluginSpec{
	Name:    pluginName,
	Version: pluginVersion,
	Type:    types.TypeProcess,
	Parameters: []types.ParameterSpec{
		{
			Name:        "file_path",
			Required:    true,
			Description: "Path to file",
		},
	},
}

type MetadataPlugin struct {
	logger   *zap.SugaredLogger
	fileRoot *utils.FileAccess
}

func NewMetadataPlugin(ps types.PluginCall) types.Plugin {
	return &MetadataPlugin{
		logger:   logger.NewPluginLogger(pluginName, ps.JobID),
		fileRoot: utils.NewFileAccess(ps.WorkingPath),
	}
}

func (p *MetadataPlugin) Name() string {
	return pluginName
}

func (p *MetadataPlugin) Type() types.PluginType {
	return types.TypeProcess
}

func (p *MetadataPlugin) Version() string {
	return pluginVersion
}

func (p *MetadataPlugin) Run(ctx context.Context, request *api.Request) (*api.Response, error) {
	filePath := api.GetStringParameter("file_path", request, "")
	if filePath == "" {
		return api.NewFailedResponse("file_path is required"), nil
	}

	p.logger.Infow("metadata started", "file_path", filePath)

	info, err := p.fileRoot.Stat(filePath)
	if err != nil {
		p.logger.Warnw("stat failed", "file_path", filePath, "error", err)
		return api.NewFailedResponse(err.Error()), nil
	}

	results := map[string]any{
		"size":     info.Size(),
		"modified": info.ModTime().Format(time.RFC3339),
		"mode":     info.Mode().String(),
		"is_dir":   info.IsDir(),
	}

	p.logger.Infow("metadata completed", "file_path", filePath, "size", info.Size())
	return api.NewResponseWithResult(results), nil
}
