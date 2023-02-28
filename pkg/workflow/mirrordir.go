package workflow

import (
	"context"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/plugin/common"
	"github.com/basenana/nanafs/pkg/types"
)

const (
	MirrorPluginName    = "workflow"
	MirrorPluginVersion = "1.0"
)

type MirrorPlugin struct {
	Manager
}

func (w *MirrorPlugin) Name() string {
	return MirrorPluginName
}

func (w *MirrorPlugin) Type() types.PluginType {
	return types.TypeMirror
}

func (w *MirrorPlugin) Version() string {
	return MirrorPluginVersion
}

func (w *MirrorPlugin) Run(ctx context.Context, request *common.Request, params map[string]string) (*common.Response, error) {
	//TODO implement me
	panic("implement me")
}

func InitWorkflowMirrorPlugin(mgr Manager) {
	plugin.Register(context.Background(), &MirrorPlugin{Manager: mgr})
	return
}
