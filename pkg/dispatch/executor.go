package dispatch

import (
	"context"
	"github.com/basenana/nanafs/pkg/types"
)

type executor interface {
	handleEvent(ctx context.Context, evt *types.Event) error
	execute(ctx context.Context, task *types.ScheduledTask) error
}
