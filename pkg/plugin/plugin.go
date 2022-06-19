package plugin

import (
	"encoding/json"
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	DefaultPluginPath     = "/var/lib/nanafs/plugins"
	DefaultRegisterPeriod = time.Minute * 1
	EnvBasePath           = "NANAFS_PLUGIN_PATH"
)

var pluginRegistry *registry

func RunPluginDaemon(meta storage.MetaStore, config config.Config, stopCh chan struct{}) error {
	base := DefaultPluginPath
	if config.Plugin.BasePath != "" {
		base = config.Plugin.BasePath
	}
	if os.Getenv(EnvBasePath) != "" {
		base = os.Getenv(EnvBasePath)
	}

	pluginRegistry = &registry{
		basePath: base,
		plugins:  map[string]types.Plugin{},
		logger:   logger.NewLogger("pluginRegistry"),
	}
	pluginRegistry.start(stopCh)

	rtm := newPluginRuntime(meta, stopCh)
	go rtm.run()

	return nil
}

type registry struct {
	basePath string
	plugins  map[string]types.Plugin
	species  map[string]types.PluginSpec
	init     bool
	mux      sync.RWMutex
	logger   *zap.SugaredLogger
}

func (r *registry) get(name string) (types.Plugin, error) {
	r.mux.RLock()
	p, ok := r.plugins[name]
	r.mux.RUnlock()
	if !ok {
		return nil, fmt.Errorf("plugin %s not found", name)
	}
	return p, nil
}

func (r *registry) start(stopCh chan struct{}) {
	ticker := time.NewTicker(DefaultRegisterPeriod)
	defer ticker.Stop()

	pluginCh := make(chan types.PluginSpec, 1)

	go func() {
		for spec := range pluginCh {
			r.loadPlugin(spec)
		}
	}()

	for {
		select {
		case <-ticker.C:
			func() {
				r.mux.Lock()
				d, err := ioutil.ReadDir(r.basePath)
				if err != nil {
					r.logger.Errorw("load plugin config failed", "basePath", r.basePath, "err", err.Error())
					return
				}
				for _, fi := range d {
					pName := filepath.Join(r.basePath, fi.Name())
					p, err := parsePluginSpec(pName)
					if err != nil {
						r.logger.Warnf("plugin %s can't be parse", pName)
						continue
					}
					pluginCh <- p
				}
				r.init = true
				defer r.mux.Unlock()
			}()
		case <-stopCh:
			r.logger.Infow("stopped")
			return
		}
	}
}

func (r *registry) loadPlugin(spec types.PluginSpec) {
	var (
		p   types.Plugin
		err error
	)
	switch spec.Type {
	case types.PluginExecType:
		p, err = NewGoPlugin(spec)
	case types.PluginBinType:
		p, err = NewBinPluginAdaptor(spec)
	case types.PluginScriptType:
		p, err = NewScriptAdaptor(spec)
	}

	if err != nil {
		r.logger.Errorw("load plugin failed", "plugin", spec.Name, "type", spec.Type, "err", err.Error())
		return
	}

	r.plugins[spec.Name] = p
	r.species[spec.Name] = spec
}

func LoadPlugin(name string) (types.Plugin, error) {
	return pluginRegistry.get(name)
}

func parsePluginSpec(pluginPath string) (types.PluginSpec, error) {
	f, err := os.Open(pluginPath)
	if err != nil {
		return types.PluginSpec{}, err
	}
	defer f.Close()

	spec := types.PluginSpec{}
	if err = json.NewDecoder(f).Decode(&spec); err != nil {
		return types.PluginSpec{}, err
	}
	return spec, nil
}
