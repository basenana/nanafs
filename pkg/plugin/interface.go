package plugin

import (
	"context"
	"github.com/basenana/nanafs/pkg/plugin/common"
	"github.com/basenana/nanafs/pkg/types"
)

type Plugin interface {
	Name() string
	Type() types.PluginType
	Version() string
}

type RunnablePlugin interface {
	Plugin
	Run(ctx context.Context, request *common.Request, params map[string]string) (*common.Response, error)
}
