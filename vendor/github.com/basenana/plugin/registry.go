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
	"sync"

	"github.com/basenana/plugin/agentic"
	"github.com/basenana/plugin/api"
	"github.com/basenana/plugin/archive"
	"github.com/basenana/plugin/checksum"
	"github.com/basenana/plugin/docloader"
	"github.com/basenana/plugin/fileop"
	"github.com/basenana/plugin/filewrite"
	"github.com/basenana/plugin/fs"
	"github.com/basenana/plugin/logger"
	"github.com/basenana/plugin/metadata"
	"github.com/basenana/plugin/rss"
	"github.com/basenana/plugin/text"
	"github.com/basenana/plugin/types"
	"github.com/basenana/plugin/web"
	"go.uber.org/zap"
)

var (
	ErrNotFound = errors.New("PluginNotFound")
)

type Factory func(ps types.PluginCall) types.Plugin

type Manager interface {
	ListPlugins() []types.PluginSpec
	GetPlugin(name string) (*types.PluginSpec, error)
	Register(spec types.PluginSpec, factory Factory)
	Call(ctx context.Context, ps types.PluginCall, req *api.Request) (resp *api.Response, err error)
}

type manager struct {
	plugins map[string]*pluginInfo
	mux     sync.RWMutex
	logger  *zap.SugaredLogger
}

type pluginInfo struct {
	factory Factory
	spec    types.PluginSpec
	disable bool
	buildIn bool
}

func (m *manager) ListPlugins() []types.PluginSpec {
	var infos = make([]*pluginInfo, 0, len(m.plugins))
	m.mux.Lock()
	for _, p := range m.plugins {
		infos = append(infos, p)
	}
	m.mux.Unlock()

	var result = make([]types.PluginSpec, 0, len(infos))
	for _, i := range infos {
		result = append(result, i.spec)
	}
	return result
}

func (m *manager) GetPlugin(name string) (*types.PluginSpec, error) {
	var info *pluginInfo
	m.mux.Lock()
	for _, p := range m.plugins {
		if p.spec.Name == name {
			info = p
			break
		}
	}
	m.mux.Unlock()
	if info == nil {
		return nil, ErrNotFound
	}
	spec := info.spec
	return &spec, nil
}

func (m *manager) Register(spec types.PluginSpec, factory Factory) {
	m.mux.Lock()
	m.plugins[spec.Name] = &pluginInfo{
		factory: factory,
		spec:    spec,
		buildIn: true,
	}
	m.mux.Unlock()
}

func (m *manager) Call(ctx context.Context, ps types.PluginCall, req *api.Request) (resp *api.Response, err error) {
	var plugin types.Plugin
	plugin, err = m.BuildPlugin(ps)
	if err != nil {
		return nil, err
	}

	runnablePlugin, ok := plugin.(ProcessPlugin)
	if !ok {
		return nil, errors.New("not process plugin")
	}

	if req.Parameter == nil {
		req.Parameter = make(map[string]any)
	}
	return runnablePlugin.Run(ctx, req)
}

func (m *manager) BuildPlugin(ps types.PluginCall) (types.Plugin, error) {
	m.mux.RLock()
	p, ok := m.plugins[ps.PluginName]
	if !ok {
		m.mux.RUnlock()
		m.logger.Warnw("build plugin failed", "plugin", ps.PluginName)
		return nil, ErrNotFound
	}
	m.mux.RUnlock()
	if ps.Params == nil {
		ps.Params = map[string]string{}
	}
	if ps.Config == nil {
		ps.Config = map[string]string{}
	}
	return p.factory(ps), nil
}

func New() Manager {
	m := &manager{
		plugins: map[string]*pluginInfo{},
		logger:  logger.NewLogger("registry"),
	}

	m.Register(archive.PluginSpec, archive.NewArchivePlugin)
	m.Register(agentic.PluginSpec, agentic.NewReactPlugin)
	m.Register(agentic.ResearchPluginSpec, agentic.NewResearchPlugin)
	m.Register(agentic.SummaryPluginSpec, agentic.NewSummaryPlugin)
	m.Register(checksum.PluginSpec, checksum.NewChecksumPlugin)
	m.Register(docloader.PluginSpec, docloader.NewDocLoader)
	m.Register(fileop.PluginSpec, fileop.NewFileOpPlugin)
	m.Register(filewrite.PluginSpec, filewrite.NewFileWritePlugin)
	m.Register(fs.SavePluginSpec, fs.NewSaver)
	m.Register(fs.UpdatePluginSpec, fs.NewUpdater)
	m.Register(metadata.PluginSpec, metadata.NewMetadataPlugin)
	m.Register(rss.RssSourcePluginSpec, rss.NewRssPlugin)
	m.Register(text.PluginSpec, text.NewTextPlugin)
	m.Register(web.WebpackPluginSpec, web.NewWebpackPlugin)

	return m
}
