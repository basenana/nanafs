package dispatch

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/bio"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"go.uber.org/zap"
)

const (
	maintainTaskIDCompact = "task.maintain.file.compact"
	maintainTaskIDClean   = "task.maintain.file.clean"
)

type maintainAction struct {
	entry      dentry.Manager
	dataStore  storage.Storage
	chunkStore metastore.ChunkStore
	recorder   metastore.ScheduledTaskRecorder
	logger     zap.SugaredLogger
}

func (m *maintainAction) handleEvent(ctx context.Context, evt *types.Event) error {
	//TODO implement me
	panic("implement me")
}

func (m *maintainAction) execute(ctx context.Context, task *types.ScheduledTask) error {
	switch task.TaskID {
	case maintainTaskIDCompact:
		md, ok := task.Event.Data.(*types.Metadata)
		if !ok || md == nil {
			return fmt.Errorf("not metadata struct")
		}
		return m.doChunkCompact(ctx, md)
	default:
		return fmt.Errorf("unknown taskID: %s", task.TaskID)
	}
}

func (m *maintainAction) doChunkCompact(ctx context.Context, md *types.Metadata) error {
	m.logger.Debugw("start compact entry segment", "entry", md.ID)
	if err := bio.CompactChunksData(ctx, md, m.chunkStore, m.dataStore); err != nil {
		m.logger.Errorw("compact entry segment error", "entry", md.ID, "err", err.Error())
		return err
	}
	return nil
}

func (m *maintainAction) doChunkClean(ctx context.Context, md *types.Metadata) error {
	return nil
}

func (m *maintainAction) doUnrefFileClean(ctx context.Context, md *types.Metadata) error {
	return nil
}
