package buildin

import (
	"context"
	"github.com/basenana/nanafs/pkg/plugin/common"
	"github.com/basenana/nanafs/pkg/types"
)

const (
	WorkflowMirrorPluginName    = "workflow"
	WorkflowMirrorPluginVersion = "1.0"
)

type WorkflowMirrorPlugin struct {
}

func (w *WorkflowMirrorPlugin) Name() string {
	return WorkflowMirrorPluginName
}

func (w *WorkflowMirrorPlugin) Type() types.PluginType {
	return types.TypeMirror
}

func (w *WorkflowMirrorPlugin) Version() string {
	return WorkflowMirrorPluginVersion
}

func (w *WorkflowMirrorPlugin) Run(ctx context.Context, request *common.Request, params map[string]string) (*common.Response, error) {
	//TODO implement me
	panic("implement me")
}

func InitWorkflowMirrorPlugin() *WorkflowMirrorPlugin {
	return nil
}
