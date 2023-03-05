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

package plugin

import (
	"context"
	"github.com/basenana/nanafs/pkg/plugin/common"
	"github.com/basenana/nanafs/pkg/types"
)

type Plugin interface {
	Name() string
	Type() types.PluginType
	Version() string
}

type RunnablePlugin interface {
	Plugin
	Run(ctx context.Context, request *common.Request, params map[string]string) (*common.Response, error)
}

type pluginInfo struct {
	Plugin
	disable bool
	buildIn bool
}
