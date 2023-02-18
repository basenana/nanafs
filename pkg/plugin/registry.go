package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/plugin/adaptors"
	"github.com/basenana/nanafs/pkg/plugin/buildin"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	ErrNotFound = errors.New("PluginNotFound")
)

type pluginInfo struct {
	Plugin
	disable bool
	buildIn bool
}

type registry struct {
	basePath string
	plugins  map[string]*pluginInfo
	mux      sync.RWMutex
	logger   *zap.SugaredLogger
}

func newPluginRegistry(cfg config.Plugin) *registry {
	if cfg.BasePath == "" {
		cfg.BasePath = DefaultPluginPath
	}

	r := &registry{
		basePath: cfg.BasePath,
		plugins:  map[string]*pluginInfo{},
		logger:   logger.NewLogger("pluginRegistry"),
	}

	if cfg.DummyPlugins {
		r.loadDummyPlugins()
	}

	if r.basePath != "" {
		go r.AutoReload(context.Background(), DefaultRegisterPeriod)
	}

	return r
}

func (r *registry) Get(ctx context.Context, ps types.PlugScope) (Plugin, error) {
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

func (r *registry) Reload(ctx context.Context) error {
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

func (r *registry) AutoReload(ctx context.Context, duration time.Duration) {
	wfPlugin := buildin.InitWorkflowMirrorPlugin()
	rssPlugin := buildin.InitRssSourcePlugin()

	r.mux.Lock()
	r.plugins[buildin.WorkflowMirrorPluginName] = &pluginInfo{
		Plugin:  wfPlugin,
		buildIn: true,
	}
	r.plugins[buildin.RssSourcePluginName] = &pluginInfo{
		Plugin:  rssPlugin,
		buildIn: true,
	}
	r.mux.Unlock()

	ticker := time.NewTicker(duration)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			func() {
				reloadCtx, canF := context.WithTimeout(context.Background(), duration)
				defer canF()
				_ = r.Reload(reloadCtx)
			}()
		case <-ctx.Done():
			return
		}
	}
}

func (r *registry) loadPluginWithSpec(spec types.PluginSpec) {
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

func (r *registry) loadDummyPlugins() {
	// register dummy plugin
	dummySourcePlugin := buildin.InitDummySourcePlugin()
	dummyProcessPlugin := buildin.InitDummyProcessPlugin()
	dummyMirrorPlugin := buildin.InitDummyMirrorPlugin()

	r.mux.Lock()
	r.plugins[dummySourcePlugin.Name()] = &pluginInfo{
		Plugin:  dummySourcePlugin,
		buildIn: true,
	}
	r.plugins[dummyProcessPlugin.Name()] = &pluginInfo{
		Plugin:  dummyProcessPlugin,
		buildIn: true,
	}
	r.plugins[dummyMirrorPlugin.Name()] = &pluginInfo{
		Plugin:  dummyMirrorPlugin,
		buildIn: true,
	}
	r.mux.Unlock()

}

func readPluginSpec(basePath, pluginSpecFile string) (types.PluginSpec, error) {
	pluginPath := filepath.Join(basePath, pluginSpecFile)
	f, err := os.Open(pluginPath)
	if err != nil {
		return types.PluginSpec{}, err
	}
	defer f.Close()

	spec := types.PluginSpec{}
	if err = json.NewDecoder(f).Decode(&spec); err != nil {
		return types.PluginSpec{}, err
	}

	if spec.Name == "" {
		return types.PluginSpec{}, fmt.Errorf("plugin name was empty")
	}
	switch spec.Type {
	case adaptors.ExecTypeGoPlugin:
	case adaptors.ExecTypeBin:
	case adaptors.ExecTypeScript:
	default:
		return types.PluginSpec{}, fmt.Errorf("plugin type %s no def", spec.Type)
	}

	if spec.Path != "" {
		_, err = os.Stat(spec.Path)
		if err != nil {
			return types.PluginSpec{}, fmt.Errorf("stat plugin failed: %s", err.Error())
		}
	}

	return spec, nil
}
