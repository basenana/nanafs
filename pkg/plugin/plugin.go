package plugin

import (
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/utils/logger"
	"os"
	"time"
)

const (
	DefaultPluginPath     = "/var/lib/nanafs/plugins"
	DefaultRegisterPeriod = time.Minute * 1
	EnvBasePath           = "NANAFS_PLUGIN_PATH"
)

func Init(meta storage.MetaStore, config config.Config, stopCh chan struct{}) error {
	base := DefaultPluginPath
	if config.Plugin.BasePath != "" {
		base = config.Plugin.BasePath
	}
	if os.Getenv(EnvBasePath) != "" {
		base = os.Getenv(EnvBasePath)
	}

	rt = &runtime{
		basePath:        base,
		meta:            meta,
		pluginProcesses: map[string]*process{},
		logger:          logger.NewLogger("pluginRuntime"),
	}

	go rt.run(stopCh)

	return nil
}

func LoadPlugin(name string) (Plugin, error) {
	return pluginRegistry.get(name)
}
