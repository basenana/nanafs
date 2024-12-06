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
	"github.com/basenana/nanafs/pkg/plugin/buildin"
	"io/ioutil"
	"strings"
	"sync"

	"go.uber.org/zap"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
)

var (
	pluginRegistry *registry
	ErrNotFound    = errors.New("PluginNotFound")
)

type Plugin interface {
	Name() string
	Type() types.PluginType
	Version() string
}

type Builder func(ctx context.Context, spec types.PluginSpec, scope types.PlugScope) (Plugin, error)

func Register(spec types.PluginSpec, builder Builder) {
	if pluginRegistry == nil {
		return
	}
	pluginRegistry.Register(spec.Name, spec, builder)
}

func BuildPlugin(ctx context.Context, ps types.PlugScope) (Plugin, error) {
	if pluginRegistry == nil {
		return nil, fmt.Errorf("plugin not init")
	}
	return pluginRegistry.BuildPlugin(ctx, ps)
}

func Init(svc buildin.Services, cfg config.Loader) error {
	r := &registry{
		cfg:     cfg,
		plugins: map[string]*pluginInfo{},
		logger:  logger.NewLogger("registry"),
	}

	// register build-in plugins
	registerBuildInProcessPlugin(svc, r)
	registerBuildInSourcePlugin(r)

	pluginRegistry = r
	return r.load(context.TODO())
}

func MustShutdown() {
	return
}

type registry struct {
	plugins map[string]*pluginInfo
	cfg     config.Loader
	mux     sync.RWMutex
	logger  *zap.SugaredLogger
}

func (r *registry) BuildPlugin(ctx context.Context, ps types.PlugScope) (Plugin, error) {
	r.mux.RLock()
	p, ok := r.plugins[ps.PluginName]
	if !ok {
		r.mux.RUnlock()
		r.logger.Warnw("build plugin failed", "plugin", ps.PluginName)
		return nil, ErrNotFound
	}
	r.mux.RUnlock()
	return p.build(ctx, p.spec, ps)
}

func (r *registry) Register(pluginName string, spec types.PluginSpec, builder Builder) {
	r.mux.Lock()
	r.plugins[pluginName] = &pluginInfo{
		build:   builder,
		spec:    spec,
		buildIn: true,
	}
	r.mux.Unlock()
}

func (r *registry) load(ctx context.Context) error {
	definePath, err := r.cfg.GetSystemConfig(ctx, config.PluginConfigGroup, "define_path").String()
	if err != nil && !errors.Is(err, config.ErrNotConfigured) {
		return err
	}
	if definePath == "" {
		return nil
	}

	d, err := ioutil.ReadDir(definePath)
	if err != nil {
		return err
	}

	var (
		needDelete = map[string]struct{}{}
		needAdd    = map[string]*pluginInfo{}
	)
	for _, fi := range d {
		if fi.IsDir() {
			continue
		}
		if strings.HasSuffix(fi.Name(), ".json") {
			pluginSpec, builder, err := readPluginSpec(definePath, fi.Name())
			if err != nil {
				r.logger.Warnf("plugin spec %s can't be parse: %s", fi.Name(), err)
				continue
			}
			needAdd[pluginSpec.Name] = &pluginInfo{build: builder, spec: pluginSpec, disable: false, buildIn: false}
		}
	}

	r.mux.Lock()
	for pName := range r.plugins {
		info, ok := needAdd[pName]
		if !ok {
			needDelete[pName] = struct{}{}
			continue
		}
		r.plugins[pName] = info
		delete(needAdd, pName)
	}

	for pName := range needAdd {
		r.plugins[pName] = needAdd[pName]
	}
	r.mux.Unlock()
	return nil
}

type pluginInfo struct {
	build   Builder
	spec    types.PluginSpec
	disable bool
	buildIn bool
}
