package plugin

import (
	"encoding/json"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"io/ioutil"
	"os"
	"path/filepath"
	goplugin "plugin"
	"sync"
	"time"
)

const (
	DefaultPluginPath     = "/var/lib/nanafs/plugins"
	DefaultRegisterPeriod = time.Minute * 1
	EnvBasePath           = "NANAFS_PLUGIN_PATH"
)

type registry struct {
	basePath string
	plugins  map[string]types.Plugin
	mux      sync.RWMutex
	logger   *zap.SugaredLogger
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
				defer r.mux.Unlock()
			}()
		case <-stopCh:
			r.logger.Infow("stopped")
			return
		}
	}
}

func (r *registry) loadPlugin(spec types.PluginSpec) {

}

func RunPluginLoader(config config.Config, stopCh chan struct{}) error {
	base := DefaultPluginPath
	if config.Plugin.BasePath != "" {
		base = config.Plugin.BasePath
	}
	if os.Getenv(EnvBasePath) != "" {
		base = os.Getenv(EnvBasePath)
	}

	r := registry{
		basePath: base,
		plugins:  map[string]types.Plugin{},
		logger:   logger.NewLogger("pluginRegistry"),
	}
	r.start(stopCh)

	return nil
}

func NewPlugin(pluginPath string) (types.Plugin, error) {
	p, err := goplugin.Open(pluginPath)
	if err != nil {
		return nil, err
	}
	pl, err := p.Lookup("Plugin")
	if err != nil {
		return nil, err
	}
	i, ok := pl.(types.Plugin)
	if !ok {
		return nil, err
	}
	return i, nil
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
