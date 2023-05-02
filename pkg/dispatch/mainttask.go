package dispatch

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/bio"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"time"
)

const (
	maintainTaskIDChunkCompact = "task.maintain.chunk.compact"
	maintainTaskIDEntryClean   = "task.maintain.entry.clean"
)

type maintainExecutor struct {
	entry      dentry.Manager
	dataStore  storage.Storage
	chunkStore metastore.ChunkStore
	recorder   metastore.ScheduledTaskRecorder
	logger     *zap.SugaredLogger
}

type compactExecutor struct {
	*maintainExecutor
}

func (c *compactExecutor) handleEvent(ctx context.Context, evt *types.Event) error {
	switch evt.Type {
	case events.ActionTypeOpen:
		return c.handleOpenEvent(ctx, evt)
	case events.ActionTypeClose:
		return c.handleCloseEvent(ctx, evt)
	}
	return nil
}

func (c *compactExecutor) handleOpenEvent(ctx context.Context, evt *types.Event) error {
	return nil
}

func (c *compactExecutor) handleCloseEvent(ctx context.Context, evt *types.Event) error {
	if !dentry.IsFileOpened(evt.RefID) {
		return nil
	}

	tasks, err := c.recorder.ListTask(ctx, maintainTaskIDChunkCompact, types.ScheduledTaskFilter{RefType: evt.RefType, RefID: evt.RefID})
	if err != nil {
		c.logger.Errorw("[compactExecutor] list scheduled task error", "entry", evt.RefID, "err", err.Error())
		return err
	}

	var (
		needRunTask *types.ScheduledTask
		needCheck   = true
	)
	for i := range tasks {
		t := tasks[i]
		if t.Status == types.ScheduledTaskFinish {
			continue
		}
		needCheck = false
		if t.Status == types.ScheduledTaskInitial {
			t.Status = types.ScheduledTaskWait
			t.ExecutionTime = time.Now()
			t.ExpirationTime = time.Now().Add(time.Hour)
			needRunTask = t
			break
		}
	}

	if needRunTask != nil {
		if err = c.recorder.SaveTask(ctx, needRunTask); err != nil {
			c.logger.Errorw("[compactExecutor] save task to waiting error", "entry", evt.RefID, "err", err.Error())
			return err
		}
	}

	if !needCheck {
		return nil
	}

	chunkCount := make(map[int64]int)
	segments, err := c.chunkStore.ListSegments(ctx, evt.RefID, 0, true)
	if err != nil {
		c.logger.Errorw("[compactExecutor] list entry all segment error", "entry", evt.RefID, "err", err.Error())
		return err
	}

	needCompact := false
	for _, seg := range segments {
		if needCompact {
			break
		}
		chunkCount[seg.ChunkID] += 1
		if chunkCount[seg.ChunkID] > 1 {
			needCompact = true
		}
	}

	return c.recorder.SaveTask(ctx, &types.ScheduledTask{
		TaskID:         maintainTaskIDChunkCompact,
		Status:         types.ScheduledTaskWait,
		CreatedTime:    time.Now(),
		ExecutionTime:  time.Now(),
		ExpirationTime: time.Now().Add(time.Hour),
		Event:          *evt,
	})
}

func (c *compactExecutor) execute(ctx context.Context, task *types.ScheduledTask) error {
	md, ok := task.Event.Data.(*types.Metadata)
	if !ok || md == nil {
		return fmt.Errorf("not metadata struct")
	}
	c.logger.Debugw("[compactExecutor] start compact entry segment", "entry", md.ID)
	if err := bio.CompactChunksData(ctx, md, c.chunkStore, c.dataStore); err != nil {
		c.logger.Errorw("[compactExecutor] compact entry segment error", "entry", md.ID, "err", err.Error())
		return err
	}
	return nil
}

type entryCleanExecutor struct {
	*maintainExecutor
}

func (c *entryCleanExecutor) handleEvent(ctx context.Context, evt *types.Event) error {
	if evt.Type != events.ActionTypeDestroy && evt.Type != events.ActionTypeClose {
		return nil
	}

	md, ok := evt.Data.(*types.Metadata)
	if !ok {
		c.logger.Errorw("[entryCleanExecutor] get metadata from event error", "entry", evt.RefID)
		return fmt.Errorf("can not get metdata")
	}
	if evt.Type == events.ActionTypeClose && md.ParentID != 0 {
		return nil
	}

	return c.recorder.SaveTask(ctx, &types.ScheduledTask{
		TaskID:         maintainTaskIDEntryClean,
		Status:         types.ScheduledTaskWait,
		CreatedTime:    time.Now(),
		ExecutionTime:  time.Now(),
		ExpirationTime: time.Now().Add(time.Hour),
		Event:          *evt,
	})
}

func (c *entryCleanExecutor) execute(ctx context.Context, task *types.ScheduledTask) error {
	evt := task.Event
	en, err := c.entry.GetEntry(ctx, evt.RefID)
	if err != nil {
		c.logger.Errorw("[entryCleanExecutor] get entry failed", "entry", evt.RefID, "task", task.ID, "err", err)
		return err
	}

	err = c.entry.CleanEntryData(ctx, en)
	if err != nil {
		c.logger.Errorw("[entryCleanExecutor] get entry failed", "entry", evt.RefID, "task", task.ID, "err", err)
		return err
	}

	_, err = c.entry.DestroyEntry(ctx, en)
	if err != nil {
		c.logger.Errorw("[entryCleanExecutor] get entry failed", "entry", evt.RefID, "task", task.ID, "err", err)
		return err
	}
	return nil
}

func registerMaintainExecutor(
	executors map[string]executor,
	recorder metastore.ScheduledTaskRecorder,
	chunkStore metastore.ChunkStore,
	dataStore storage.Storage) {
	e := &maintainExecutor{
		dataStore:  dataStore,
		chunkStore: chunkStore,
		recorder:   recorder,
		logger:     logger.NewLogger("maintainExecutor"),
	}

	executors[maintainTaskIDChunkCompact] = &compactExecutor{maintainExecutor: e}
	executors[maintainTaskIDEntryClean] = &entryCleanExecutor{maintainExecutor: e}
}
