package dispatch

import (
	"context"
	"github.com/basenana/nanafs/pkg/types"
)

type workflowAction struct {
}

func (w workflowAction) handleEvent(ctx context.Context, evt *types.Event) error {
	//TODO implement me
	panic("implement me")
}

func (w workflowAction) execute(ctx context.Context, task *types.ScheduledTask) error {
	//TODO implement me
	panic("implement me")
}
