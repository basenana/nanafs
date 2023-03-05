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

package workflow

import (
	"context"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/plugin/common"
	"github.com/basenana/nanafs/pkg/types"
)

const (
	MirrorPluginName    = "workflow"
	MirrorPluginVersion = "1.0"
)

type MirrorPlugin struct {
	Manager
}

func (w *MirrorPlugin) Name() string {
	return MirrorPluginName
}

func (w *MirrorPlugin) Type() types.PluginType {
	return types.TypeMirror
}

func (w *MirrorPlugin) Version() string {
	return MirrorPluginVersion
}

func (w *MirrorPlugin) Run(ctx context.Context, request *common.Request, params map[string]string) (*common.Response, error) {
	//TODO implement me
	panic("implement me")
}

func InitWorkflowMirrorPlugin(mgr Manager) {
	plugin.Register(context.Background(), &MirrorPlugin{Manager: mgr})
	return
}
