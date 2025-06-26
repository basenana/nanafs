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

package adaptors

import (
	"context"
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
)

const (
	AdaptorTypeScriptPlugin        = "script"
	AdaptorTypeScriptPluginVersion = "1.0"
)

type ScriptPluginAdaptor struct {
	spec   types.PluginSpec
	scope  types.PluginCall
	logger *zap.SugaredLogger
}

func (s *ScriptPluginAdaptor) Name() string { return AdaptorTypeScriptPlugin }

func (s *ScriptPluginAdaptor) Type() types.PluginType { return types.TypeProcess }

func (s *ScriptPluginAdaptor) Version() string { return AdaptorTypeScriptPluginVersion }

func (s *ScriptPluginAdaptor) Run(ctx context.Context, request *pluginapi.Request) (*pluginapi.Response, error) {
	//TODO implement me
	panic("implement me")
}

func NewScriptPluginAdaptor(spec types.PluginSpec, scope types.PluginCall) (*ScriptPluginAdaptor, error) {
	return &ScriptPluginAdaptor{spec: spec, scope: scope, logger: logger.NewLogger("scriptPluginAdaptor")}, nil
}
