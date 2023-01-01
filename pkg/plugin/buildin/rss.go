package buildin

import (
	"context"
	"github.com/basenana/nanafs/pkg/plugin/common"
	"github.com/basenana/nanafs/pkg/types"
)

const (
	RssSourcePluginName    = "rss"
	RssSourcePluginVersion = "1.0"
)

type RssSourcePlugin struct {
}

func (r *RssSourcePlugin) Name() string {
	return RssSourcePluginName
}

func (r *RssSourcePlugin) Type() types.PluginType {
	return types.TypeSource
}

func (r *RssSourcePlugin) Version() string {
	return RssSourcePluginVersion
}

func (r *RssSourcePlugin) Run(ctx context.Context, request *common.Request, params map[string]string) (*common.Response, error) {
	//TODO implement me
	panic("implement me")
}

func InitRssSourcePlugin() *RssSourcePlugin {
	return nil
}
