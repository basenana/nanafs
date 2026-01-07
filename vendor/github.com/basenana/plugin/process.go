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

	"github.com/basenana/plugin/api"
	"github.com/basenana/plugin/logger"
	"github.com/basenana/plugin/types"
	"go.uber.org/zap"
)

type ProcessPlugin interface {
	types.Plugin
	Run(ctx context.Context, request *api.Request) (*api.Response, error)
}

const (
	delayPluginName    = "delay"
	delayPluginVersion = "1.0"
)

var DelayProcessPluginSpec = types.PluginSpec{
	Name:    delayPluginName,
	Version: delayPluginVersion,
	Type:    types.TypeProcess,
}

type DelayProcessPlugin struct {
	logger *zap.SugaredLogger
}

var _ ProcessPlugin = &DelayProcessPlugin{}

func NewDelayProcessPlugin(ps types.PluginCall) types.Plugin {
	return &DelayProcessPlugin{
		logger: logger.NewPluginLogger(delayPluginName, ps.JobID),
	}
}

func (d *DelayProcessPlugin) Name() string {
	return delayPluginName
}

func (d *DelayProcessPlugin) Type() types.PluginType {
	return types.TypeProcess
}

func (d *DelayProcessPlugin) Version() string {
	return delayPluginVersion
}

func (d *DelayProcessPlugin) Run(ctx context.Context, request *api.Request) (*api.Response, error) {
	var (
		until            time.Time
		delayDurationStr = api.GetStringParameter("delay", request, "")
		untilStr         = api.GetStringParameter("until", request, "")
	)

	switch {

	case delayDurationStr != "":
		duration, err := time.ParseDuration(delayDurationStr)
		if err != nil {
			d.logger.Warnw("parse delay duration failed", "duration", delayDurationStr, "error", err)
			return nil, fmt.Errorf("parse delay duration [%s] failed: %s", delayDurationStr, err)
		}
		until = time.Now().Add(duration)

	case untilStr != "":
		var err error
		until, err = time.Parse(time.RFC3339, untilStr)
		if err != nil {
			d.logger.Warnw("parse delay until failed", "until", untilStr, "error", err)
			return nil, fmt.Errorf("parse delay until [%s] failed: %s", untilStr, err)
		}

	default:
		return api.NewFailedResponse(fmt.Sprintf("unknown action")), nil
	}

	d.logger.Infow("delay started", "until", until)

	now := time.Now()
	if now.Before(until) {
		timer := time.NewTimer(until.Sub(now))
		defer timer.Stop()
		select {
		case <-timer.C:
			d.logger.Infow("delay completed")
			return api.NewResponse(), nil
		case <-ctx.Done():
			return api.NewFailedResponse(ctx.Err().Error()), nil
		}
	}

	return api.NewResponse(), nil
}
