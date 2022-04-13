package plugin

import (
	"github.com/basenana/go-flow/log"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

const (
	DefaultPluginPath     = "/var/lib/nanafs/plugins"
	DefaultRegisterPeriod = time.Minute * 1
)

var (
	Plugins map[string]Plugin
	logger  log.Logger
)

func init() {
	logger = log.NewLogger("plugin")
	//go parsePlugins()
}

func parsePlugins() {
	ticker := time.NewTicker(DefaultRegisterPeriod)
	defer ticker.Stop()

	for {
		<-ticker.C
		go func() {
			if err := RegisterPlugins(); err != nil {
				logger.Warnf("register plugin error: %v", err)
			}
		}()
	}
}

func RegisterPlugins() error {
	Plugins = make(map[string]Plugin)
	pluginPath := os.Getenv("NANAFS_PLUGIN_PATH")
	if pluginPath == "" {
		pluginPath = DefaultPluginPath
	}
	d, err := ioutil.ReadDir(pluginPath)
	if err != nil {
		return err
	}
	for _, fi := range d {
		pName := filepath.Join(pluginPath, fi.Name())
		p, err := NewPlugin(pName)
		if err != nil {
			logger.Warnf("plugin %s can't be parse", pName)
			continue
		}
		Plugins[p.Name()] = p
	}
	return nil
}
