package plugin

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/plugin/common"
	"github.com/basenana/nanafs/pkg/types"
	"time"
)

const (
	DefaultPluginPath     = "/var/lib/nanafs/plugins"
	DefaultRegisterPeriod = time.Minute * 1
)

var (
	pluginRegistry *registry
)

func Init(config config.Config) error {
	basePath := config.Plugin.BasePath
	if basePath == "" {
		basePath = DefaultPluginPath
	}
	pluginRegistry = newPluginRegistry(basePath)
	return nil
}

func Call(ctx context.Context, ps types.PlugScope, req *common.Request) (resp *common.Response, err error) {
	var plugin Plugin
	plugin, err = pluginRegistry.Get(ctx, ps)
	if err != nil {
		return nil, err
	}

	runnablePlugin, ok := plugin.(RunnablePlugin)
	if !ok {
		return nil, fmt.Errorf("not runnable plugin")
	}
	return runnablePlugin.Run(ctx, req, ps.Parameters)
}
