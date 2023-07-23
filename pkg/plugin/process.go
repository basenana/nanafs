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
	"fmt"
	"github.com/basenana/nanafs/pkg/plugin/stub"
	"github.com/basenana/nanafs/pkg/types"
	"time"
)

type ProcessPlugin interface {
	Plugin
	Run(ctx context.Context, request *stub.Request, params map[string]string) (*stub.Response, error)
}

func Call(ctx context.Context, ps types.PlugScope, req *stub.Request) (resp *stub.Response, err error) {
	var plugin Plugin
	plugin, err = BuildPlugin(ctx, ps)
	if err != nil {
		return nil, err
	}

	runnablePlugin, ok := plugin.(ProcessPlugin)
	if !ok {
		return nil, fmt.Errorf("not process plugin")
	}
	return runnablePlugin.Run(ctx, req, ps.Parameters)
}

type DummyProcessPlugin struct{}

var _ ProcessPlugin = &DummyProcessPlugin{}

func (d *DummyProcessPlugin) Name() string {
	return "dummy-process-plugin"
}

func (d *DummyProcessPlugin) Type() types.PluginType {
	return types.TypeProcess
}

func (d *DummyProcessPlugin) Version() string {
	return "1.0"
}

func (d *DummyProcessPlugin) Run(ctx context.Context, request *stub.Request, params map[string]string) (*stub.Response, error) {
	time.Sleep(time.Second * 2)
	return &stub.Response{IsSucceed: true}, nil
}
