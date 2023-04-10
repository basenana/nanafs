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
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/plugin/adaptors"
	"github.com/basenana/nanafs/pkg/plugin/buildin"
	"github.com/basenana/nanafs/pkg/plugin/common"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"io/ioutil"
	"sync"
	"time"
)

const (
	DefaultPluginPath     = "/var/lib/nanafs/plugins"
	DefaultRegisterPeriod = time.Minute * 1
)

var (
	pluginEntries *plugins
	ErrNotFound   = errors.New("PluginNotFound")
)

func Init(cfg config.Config, recorderGetter metastore.PluginRecorderGetter) error {
	if cfg.Plugin.BasePath == "" {
		cfg.Plugin.BasePath = DefaultPluginPath
	}

	r := &plugins{
		basePath: cfg.Plugin.BasePath,
		plugins:  map[string]*pluginInfo{},
		recorder: recorderGetter,
		logger:   logger.NewLogger("plugins"),
	}

	if cfg.Plugin.DummyPlugins {
		loadDummyPlugins(r)
	}

	if r.basePath != "" {
		go r.AutoReload(context.Background(), DefaultRegisterPeriod)
	}

	pluginEntries = r
	return nil
}

func Register(ctx context.Context, plugin Plugin) {
	pluginEntries.Register(ctx, plugin.Name(), plugin)
}

func Call(ctx context.Context, ps types.PlugScope, req *common.Request) (resp *common.Response, err error) {
	var plugin Plugin
	plugin, err = pluginEntries.Get(ctx, ps)
	if err != nil {
		return nil, err
	}

	runnablePlugin, ok := plugin.(RunnablePlugin)
	if !ok {
		return nil, fmt.Errorf("not runnable plugin")
	}
	return runnablePlugin.Run(ctx, req, ps.Parameters)
}

type plugins struct {
	basePath string
	plugins  map[string]*pluginInfo
	mux      sync.RWMutex
	recorder metastore.PluginRecorderGetter
	logger   *zap.SugaredLogger
}

func (r *plugins) Get(ctx context.Context, ps types.PlugScope) (Plugin, error) {
	r.mux.RLock()
	defer r.mux.RUnlock()
	p, ok := r.plugins[ps.PluginName]
	if !ok {
		return nil, ErrNotFound
	}
	if p.Type() != ps.PluginType {
		return nil, ErrNotFound
	}
	return p.Plugin, nil
}

func (r *plugins) Register(ctx context.Context, pluginName string, plugin Plugin) {
	r.mux.Lock()
	r.plugins[pluginName] = &pluginInfo{
		Plugin:  plugin,
		buildIn: true,
	}
	r.mux.Unlock()
}

func (r *plugins) reload(ctx context.Context) error {
	r.mux.Lock()

	d, err := ioutil.ReadDir(r.basePath)
	if err != nil {
		r.logger.Errorw("load plugin config failed", "basePath", r.basePath, "err", err.Error())
		r.mux.Unlock()
		return err
	}

	plugNames := make(map[string]struct{})
	for _, fi := range d {
		pluginSpec, err := readPluginSpec(r.basePath, fi.Name())
		if err != nil {
			r.logger.Warnf("plugin spec %s can't be parse: %s", fi.Name(), err.Error())
			continue
		}
		r.loadPluginWithSpec(pluginSpec)
		plugNames[pluginSpec.Name] = struct{}{}
	}

	r.mux.Lock()
	for pName := range r.plugins {
		if _, ok := plugNames[pName]; !ok {
			delete(r.plugins, pName)
		}
	}
	r.mux.Unlock()
	return nil
}

func (r *plugins) AutoReload(ctx context.Context, duration time.Duration) {
	rssPlugin := buildin.InitRssSourcePlugin()
	r.Register(ctx, buildin.RssSourcePluginName, rssPlugin)

	ticker := time.NewTicker(duration)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			func() {
				reloadCtx, canF := context.WithTimeout(context.Background(), duration)
				defer canF()
				_ = r.reload(reloadCtx)
			}()
		case <-ctx.Done():
			return
		}
	}
}

func (r *plugins) loadPluginWithSpec(spec types.PluginSpec) {
	var (
		p   Plugin
		err error
	)
	switch spec.Type {
	case adaptors.ExecTypeGoPlugin:
		p, err = adaptors.NewGoPluginAdaptor(spec)
	case adaptors.ExecTypeBin:
		p, err = adaptors.NewBinPluginAdaptor(spec)
	case adaptors.ExecTypeScript:
		p, err = adaptors.NewScriptAdaptor(spec)
	default:
		r.logger.Warnf("load plugin %s failed: unknown plugin type %s", spec.Name, spec.Type)
	}

	if err != nil {
		r.logger.Errorw("load plugin failed", "plugin", spec.Name, "type", spec.Type, "err", err.Error())
		return
	}

	r.mux.Lock()
	r.plugins[spec.Name] = &pluginInfo{
		Plugin:  p,
		disable: false,
	}
	r.mux.Unlock()
}
