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
	"errors"
	"fmt"
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/utils"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
)

var (
	ErrNotFound = errors.New("PluginNotFound")
)

type Manager struct {
	r *registry
}

func (m *Manager) ListPlugins() []types.PluginSpec {
	infos := m.r.List()
	var result = make([]types.PluginSpec, 0, len(infos))
	for _, i := range infos {
		result = append(result, i.spec)
	}
	return result
}

func (m *Manager) Call(ctx context.Context, job *types.WorkflowJob, ps types.PluginCall, req *pluginapi.Request) (resp *pluginapi.Response, err error) {
	startAt := time.Now()
	defer func() {
		if rErr := utils.Recover(); rErr != nil {
			err = rErr
		}
		processCallTimeUsage.WithLabelValues(ps.PluginName).Observe(time.Since(startAt).Seconds())
	}()
	var plugin Plugin
	plugin, err = m.r.BuildPlugin(job, ps)
	if err != nil {
		return nil, err
	}

	runnablePlugin, ok := plugin.(ProcessPlugin)
	if !ok {
		return nil, fmt.Errorf("not process plugin")
	}
	return runnablePlugin.Run(ctx, req)
}

type Plugin interface {
	Name() string
	Type() types.PluginType
	Version() string
}

func Init(cfg config.Workflow) (*Manager, error) {
	r := &registry{
		cfg:     cfg,
		plugins: map[string]*pluginInfo{},
		logger:  logger.NewLogger("registry"),
	}

	// register build-in plugins
	registerBuildInProcessPlugin(r)
	registerBuildInSourcePlugin(r)

	return &Manager{r: r}, nil
}

type registry struct {
	plugins map[string]*pluginInfo
	cfg     config.Workflow
	mux     sync.RWMutex
	logger  *zap.SugaredLogger
}

func (r *registry) BuildPlugin(job *types.WorkflowJob, ps types.PluginCall) (Plugin, error) {
	r.mux.RLock()
	p, ok := r.plugins[ps.PluginName]
	if !ok {
		r.mux.RUnlock()
		r.logger.Warnw("build plugin failed", "plugin", ps.PluginName)
		return nil, ErrNotFound
	}
	r.mux.RUnlock()
	return p.singleton, nil
}

func (r *registry) Register(pluginName string, spec types.PluginSpec, singleton Plugin) {
	r.mux.Lock()
	r.plugins[pluginName] = &pluginInfo{
		singleton: singleton,
		spec:      spec,
		buildIn:   true,
	}
	r.mux.Unlock()
}

func (r *registry) List() []*pluginInfo {
	var result = make([]*pluginInfo, 0, len(r.plugins))
	r.mux.Lock()
	for _, p := range r.plugins {
		result = append(result, p)
	}
	r.mux.Unlock()
	return result
}

type pluginInfo struct {
	singleton Plugin
	spec      types.PluginSpec
	disable   bool
	buildIn   bool
}
