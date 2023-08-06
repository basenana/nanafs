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

const (
	delayPluginName    = "delay"
	delayPluginVersion = "1.0"
)

type DelayProcessPlugin struct{}

var _ ProcessPlugin = &DelayProcessPlugin{}

func (d *DelayProcessPlugin) Name() string {
	return delayPluginName
}

func (d *DelayProcessPlugin) Type() types.PluginType {
	return types.TypeProcess
}

func (d *DelayProcessPlugin) Version() string {
	return delayPluginVersion
}

func (d *DelayProcessPlugin) Run(ctx context.Context, request *stub.Request, params map[string]string) (*stub.Response, error) {
	var (
		until   time.Time
		nowTime = time.Now()
	)
	switch request.Action {

	case "delay":
		delayDurationStr := params["delay"]
		duration, err := time.ParseDuration(delayDurationStr)
		if err != nil {
			return nil, fmt.Errorf("parse delay duration [%s] failed: %s", delayDurationStr, err)
		}
		until = time.Now().Add(duration)

	case "until":
		var err error
		untilStr := params["until"]
		until, err = time.Parse(untilStr, time.RFC3339)
		if err != nil {
			return nil, fmt.Errorf("parse delay until [%s] failed: %s", untilStr, err)
		}

	default:
		resp := stub.NewResponse()
		resp.Message = fmt.Sprintf("unknown action: %s", request.Action)
		return resp, nil
	}

	if nowTime.Before(until) {
		timer := time.NewTimer(until.Sub(nowTime))
		defer timer.Stop()
		select {
		case <-timer.C:
			return &stub.Response{IsSucceed: true}, nil
		case <-ctx.Done():
			return &stub.Response{IsSucceed: false, Message: ctx.Err().Error()}, nil
		}
	}

	return &stub.Response{IsSucceed: true}, nil
}

func registerDelayPlugin(r *registry) {
	r.Register(
		delayPluginName,
		types.PluginSpec{Name: delayPluginName, Version: delayPluginVersion, Type: types.TypeProcess, Parameters: map[string]string{}},
		func(ctx context.Context, spec types.PluginSpec, scope types.PlugScope) (Plugin, error) {
			return &DelayProcessPlugin{}, nil
		},
	)
}
