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
	"github.com/basenana/nanafs/utils"
)

type ProcessPlugin interface {
	Plugin
	Run(ctx context.Context, request *pluginapi.Request) (*pluginapi.Response, error)
}

func Call(ctx context.Context, ps types.PlugScope, req *pluginapi.Request) (resp *pluginapi.Response, err error) {
	startAt := time.Now()
	defer func() {
		if rErr := utils.Recover(); rErr != nil {
			err = rErr
		}
		processCallTimeUsage.WithLabelValues(ps.PluginName).Observe(time.Since(startAt).Seconds())
	}()
	var plugin Plugin
	plugin, err = BuildPlugin(ctx, ps)
	if err != nil {
		return nil, err
	}

	runnablePlugin, ok := plugin.(ProcessPlugin)
	if !ok {
		return nil, fmt.Errorf("not process plugin")
	}
	return runnablePlugin.Run(ctx, req)
}

const (
	delayPluginName    = "delay"
	delayPluginVersion = "1.0"
)

type DelayProcessPlugin struct {
	spec  types.PluginSpec
	scope types.PlugScope
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

	pluginParams := d.scope.Parameters
	switch request.Action {

	case "delay":
		delayDurationStr := pluginParams["delay"]
		duration, err := time.ParseDuration(delayDurationStr)
		if err != nil {
			return nil, fmt.Errorf("parse delay duration [%s] failed: %s", delayDurationStr, err)
		}
		until = time.Now().Add(duration)

	case "until":
		var err error
		untilStr := pluginParams["until"]
		until, err = time.Parse(untilStr, time.RFC3339)
		if err != nil {
			return nil, fmt.Errorf("parse delay until [%s] failed: %s", untilStr, err)
		}

	default:
		resp := pluginapi.NewResponse()
		resp.Message = fmt.Sprintf("unknown action: %s", request.Action)
		return resp, nil
	}

	if nowTime.Before(until) {
		timer := time.NewTimer(until.Sub(nowTime))
		defer timer.Stop()
		select {
		case <-timer.C:
			return pluginapi.NewResponseWithResult(map[string]any{"delay_finish_at": time.Now().String()}), nil
		case <-ctx.Done():
			return &pluginapi.Response{IsSucceed: false, Message: ctx.Err().Error()}, nil
		}
	}

	return pluginapi.NewResponseWithResult(nil), nil
}

func registerBuildInProcessPlugin(svc buildin.Services, r *registry) {
	r.Register(
		delayPluginName,
		types.PluginSpec{Name: delayPluginName, Version: delayPluginVersion, Type: types.TypeProcess, Parameters: map[string]string{}},
		func(ctx context.Context, spec types.PluginSpec, scope types.PlugScope) (Plugin, error) {
			return &DelayProcessPlugin{spec: spec, scope: scope}, nil
		},
	)

	r.Register(
		docloader.PluginName,
		types.PluginSpec{Name: docloader.PluginName, Version: docloader.PluginVersion, Type: types.TypeProcess, Parameters: map[string]string{}},
		func(ctx context.Context, spec types.PluginSpec, scope types.PlugScope) (Plugin, error) {
			return docloader.NewDocLoader(spec, scope), nil
		},
	)

	r.Register(
		buildin.IngestPluginName,
		types.PluginSpec{Name: buildin.IngestPluginName, Version: buildin.IngestPluginVersion, Type: types.TypeProcess, Parameters: map[string]string{}},
		func(ctx context.Context, spec types.PluginSpec, scope types.PlugScope) (Plugin, error) {
			return buildin.NewIngestPlugin(spec, scope, svc)
		},
	)

	r.Register(
		buildin.SummaryPluginName,
		types.PluginSpec{Name: buildin.SummaryPluginName, Version: buildin.SummaryPluginVersion, Type: types.TypeProcess, Parameters: map[string]string{}},
		func(ctx context.Context, spec types.PluginSpec, scope types.PlugScope) (Plugin, error) {
			return buildin.NewSummaryPlugin(spec, scope, svc)
		},
	)

	r.Register(
		buildin.KeywordsPluginName,
		types.PluginSpec{Name: buildin.KeywordsPluginName, Version: buildin.KeywordsPluginVersion, Type: types.TypeProcess, Parameters: map[string]string{}},
		func(ctx context.Context, spec types.PluginSpec, scope types.PlugScope) (Plugin, error) {
			return buildin.NewKeyWordsPlugin(spec, scope, svc)
		},
	)

	r.Register(
		buildin.DocMetaPluginName,
		types.PluginSpec{Name: buildin.DocMetaPluginName, Version: buildin.DocMetaPluginVersion, Type: types.TypeProcess, Parameters: map[string]string{}},
		func(ctx context.Context, spec types.PluginSpec, scope types.PlugScope) (Plugin, error) {
			return buildin.NewDocMetaPlugin(spec, scope, svc)
		},
	)

	r.Register(
		buildin.WebpackPluginName,
		types.PluginSpec{Name: buildin.WebpackPluginName, Version: buildin.WebpackPluginVersion, Type: types.TypeProcess, Parameters: map[string]string{}},
		func(ctx context.Context, spec types.PluginSpec, scope types.PlugScope) (Plugin, error) {
			return buildin.NewWebpackPlugin(spec, scope, svc)
		},
	)
}
