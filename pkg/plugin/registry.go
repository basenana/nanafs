package plugin

import (
	"encoding/json"
	"fmt"
	"github.com/basenana/nanafs/pkg/plugin/adaptors"
	"go.uber.org/zap"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var pluginRegistry *registry

type Registry interface {
	Autoload(stopCh chan struct{})
	//Loaded() bool
}

type registry struct {
	basePath string
	plugins  map[string]Plugin
	species  map[string]Spec
	init     bool
	mux      sync.RWMutex
	logger   *zap.SugaredLogger
}

func (r *registry) get(name string) (Plugin, error) {
	r.mux.RLock()
	p, ok := r.plugins[name]
	r.mux.RUnlock()
	if !ok {
		return nil, fmt.Errorf("plugin %s not found", name)
	}
	return p, nil
}

func (r *registry) Autoload(stopCh chan struct{}) {
	ticker := time.NewTicker(DefaultRegisterPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			func() {
				r.mux.Lock()
				defer r.mux.Unlock()
				d, err := ioutil.ReadDir(r.basePath)
				if err != nil {
					r.logger.Errorw("load plugin config failed", "basePath", r.basePath, "err", err.Error())
					return
				}
				plugNames := make(map[string]struct{})
				for _, fi := range d {
					pName := filepath.Join(r.basePath, fi.Name())
					p, err := parsePluginSpec(pName)
					if err != nil {
						r.logger.Warnf("plugin %s can't be parse", pName)
						continue
					}
					r.loadPlugin(p)
					plugNames[p.Name] = struct{}{}
				}
				for pName := range r.plugins {
					if _, ok := plugNames[pName]; !ok {
						delete(r.plugins, pName)
					}
				}
				r.init = true
			}()
		case <-stopCh:
			r.logger.Infow("stopped")
			return
		}
	}
}

func (r *registry) loadPlugin(spec Spec) {
	var (
		p   Plugin
		err error
	)
	switch spec.Type {
	case PluginLibType:
		p, err = adaptors.NewGoPlugin(spec)
	case PluginBinType:
		p, err = adaptors.NewBinPluginAdaptor(spec)
	case PluginScriptType:
		p, err = adaptors.NewScriptAdaptor(spec)
	}

	if err != nil {
		r.logger.Errorw("load plugin failed", "plugin", spec.Name, "type", spec.Type, "err", err.Error())
		return
	}

	r.plugins[spec.Name] = p
	r.species[spec.Name] = spec
}

func parsePluginSpec(pluginPath string) (Spec, error) {
	f, err := os.Open(pluginPath)
	if err != nil {
		return Spec{}, err
	}
	defer f.Close()

	spec := Spec{}
	if err = json.NewDecoder(f).Decode(&spec); err != nil {
		return Spec{}, err
	}

	if spec.Name == "" {
		return Spec{}, fmt.Errorf("plugin name was empty")
	}
	switch spec.Type {
	case PluginLibType:
	case PluginBinType:
	case PluginScriptType:
	default:
		return Spec{}, fmt.Errorf("plugin type %s no def", spec.Type)
	}

	_, err = os.Stat(spec.Path)
	if err != nil {
		return Spec{}, fmt.Errorf("stat plugin failed: %s", err.Error())
	}

	return spec, nil
}
