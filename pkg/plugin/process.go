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
	"time"

	"github.com/basenana/nanafs/pkg/plugin/buildin"
	"github.com/basenana/nanafs/pkg/plugin/buildin/docloader"
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
)

type ProcessPlugin interface {
	Plugin
	Run(ctx context.Context, request *pluginapi.Request) (*pluginapi.Response, error)
}

const (
	delayPluginName    = "delay"
	delayPluginVersion = "1.0"
)

var DelayProcessPluginSpec = types.PluginSpec{
	Name:       delayPluginName,
	Version:    delayPluginVersion,
	Type:       types.TypeProcess,
	Parameters: make(map[string]string),
	Customization: []types.PluginConfig{
		{Key: "delay", Default: ""},
		{Key: "until", Default: ""},
	},
}

type DelayProcessPlugin struct {
	scope types.PluginCall
}

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

func (d *DelayProcessPlugin) Run(ctx context.Context, request *pluginapi.Request) (*pluginapi.Response, error) {
	var (
		until   time.Time
		nowTime = time.Now()
	)

	switch request.Action {

	case "delay":
		delayDurationStr := pluginapi.GetParameter("delay", request, DelayProcessPluginSpec, d.scope)
		duration, err := time.ParseDuration(delayDurationStr)
		if err != nil {
			return nil, fmt.Errorf("parse delay duration [%s] failed: %s", delayDurationStr, err)
		}
		until = time.Now().Add(duration)

	case "until":
		var err error
		untilStr := pluginapi.GetParameter("until", request, DelayProcessPluginSpec, d.scope)
		until, err = time.Parse(untilStr, time.RFC3339)
		if err != nil {
			return nil, fmt.Errorf("parse delay until [%s] failed: %s", untilStr, err)
		}

	default:
		return pluginapi.NewFailedResponse(fmt.Sprintf("unknown action: %s", request.Action)), nil
	}

	if nowTime.Before(until) {
		timer := time.NewTimer(until.Sub(nowTime))
		defer timer.Stop()
		select {
		case <-timer.C:
			return pluginapi.NewResponse(), nil
		case <-ctx.Done():
			return pluginapi.NewFailedResponse(ctx.Err().Error()), nil
		}
	}

	return pluginapi.NewResponse(), nil
}

func registerBuildInProcessPlugin(r *registry) {
	r.Register(
		delayPluginName,
		DelayProcessPluginSpec,
		func(job *types.WorkflowJob, scope types.PluginCall) (Plugin, error) {
			return &DelayProcessPlugin{scope: scope}, nil
		},
	)

	r.Register(
		docloader.PluginName,
		docloader.PluginSpec,
		func(job *types.WorkflowJob, scope types.PluginCall) (Plugin, error) {
			return docloader.NewDocLoader(job, scope), nil
		},
	)

	r.Register(
		buildin.WebpackPluginName,
		buildin.WebpackPluginSpec,
		func(job *types.WorkflowJob, scope types.PluginCall) (Plugin, error) {
			return buildin.NewWebpackPlugin(job, scope)
		},
	)
}
