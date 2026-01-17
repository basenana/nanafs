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

package fileop

import (
	"context"
	"fmt"

	"github.com/basenana/plugin/api"
	"github.com/basenana/plugin/logger"
	"github.com/basenana/plugin/types"
	"github.com/basenana/plugin/utils"
	"go.uber.org/zap"
)

const (
	pluginName    = "fileop"
	pluginVersion = "1.0"
)

var PluginSpec = types.PluginSpec{
	Name:    pluginName,
	Version: pluginVersion,
	Type:    types.TypeProcess,
	Parameters: []types.ParameterSpec{
		{
			Name:        "action",
			Required:    true,
			Description: "Action: cp, mv, rm, rename",
			Options:     []string{"cp", "mv", "rm", "rename"},
		},
		{
			Name:        "src",
			Required:    true,
			Description: "Source path",
		},
		{
			Name:        "dest",
			Required:    false,
			Description: "Destination path (required for cp, mv, rename)",
		},
	},
}

type FileOpPlugin struct {
	logger   *zap.SugaredLogger
	fileRoot *utils.FileAccess
}

func NewFileOpPlugin(ps types.PluginCall) types.Plugin {
	return &FileOpPlugin{
		logger:   logger.NewPluginLogger(pluginName, ps.JobID),
		fileRoot: utils.NewFileAccess(ps.WorkingPath),
	}
}

func (p *FileOpPlugin) Name() string {
	return pluginName
}

func (p *FileOpPlugin) Type() types.PluginType {
	return types.TypeProcess
}

func (p *FileOpPlugin) Version() string {
	return pluginVersion
}

func (p *FileOpPlugin) Run(ctx context.Context, request *api.Request) (*api.Response, error) {
	action := api.GetStringParameter("action", request, "")
	src := api.GetStringParameter("src", request, "")
	dest := api.GetStringParameter("dest", request, "")

	if action == "" {
		return api.NewFailedResponse("action is required"), nil
	}

	if src == "" {
		return api.NewFailedResponse("src is required"), nil
	}

	p.logger.Infow("fileop started", "action", action, "src", src, "dest", dest)

	var err error
	switch action {
	case "cp":
		err = p.fileRoot.Copy(dest, src, 0644)
	case "mv":
		err = p.fileRoot.Rename(src, dest)
	case "rm":
		err = p.fileRoot.Remove(src)
	case "rename":
		if dest == "" {
			return api.NewFailedResponse("dest is required for rename action"), nil
		}
		err = p.fileRoot.Rename(src, dest)
	default:
		return api.NewFailedResponse(fmt.Sprintf("unknown action: %s", action)), nil
	}

	if err != nil {
		p.logger.Warnw("fileop failed", "action", action, "src", src, "dest", dest, "error", err)
		return api.NewFailedResponse(err.Error()), nil
	}

	p.logger.Infow("fileop completed", "action", action, "src", src, "dest", dest)
	return api.NewResponse(), nil
}
