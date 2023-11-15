/*
 Copyright 2023 NanaFS Authors.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package dispatch

import (
	"context"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"time"
)

const (
	maintainTaskIDChunkCompact = "task.maintain.chunk.compact"
	maintainTaskIDEntryCleanup = "task.maintain.entry.cleanup"
)

type maintainExecutor struct {
	entry    dentry.Manager
	recorder metastore.ScheduledTaskRecorder
	logger   *zap.SugaredLogger
}

type compactExecutor struct {
	*maintainExecutor
}

func (c *compactExecutor) handleEvent(ctx context.Context, evt *types.EntryEvent) error {
	if evt.Type != events.ActionTypeCompact {
		return nil
	}
	task, err := getWaitingTask(ctx, c.recorder, maintainTaskIDChunkCompact, evt)
	if err != nil {
		c.logger.Errorw("[compactExecutor] list scheduled task error", "entry", evt.RefID, "err", err.Error())
		return err
	}

	if task != nil {
		return nil
	}

	task = &types.ScheduledTask{
		TaskID:         maintainTaskIDChunkCompact,
		Status:         types.ScheduledTaskWait,
		RefType:        evt.RefType,
		RefID:          evt.RefID,
		CreatedTime:    time.Now(),
		ExpirationTime: time.Now().Add(time.Hour),
		Event:          *evt,
	}
	if err = c.recorder.SaveTask(ctx, task); err != nil {
		c.logger.Errorw("[compactExecutor] save task to waiting error", "entry", evt.RefID, "err", err.Error())
		return err
	}
	return nil
}

func (c *compactExecutor) execute(ctx context.Context, task *types.ScheduledTask) error {
	entry := task.Event.Data
	if dentry.IsFileOpened(entry.ID) {
		return ErrNeedRetry
	}

	en, err := c.entry.GetEntry(ctx, entry.ID)
	if err != nil {
		c.logger.Errorw("[compactExecutor] query entry error", "entry", entry.ID, "err", err.Error())
		return err
	}
	c.logger.Debugw("[compactExecutor] start compact entry segment", "entry", entry.ID)
	if err = c.entry.ChunkCompact(ctx, en.ID); err != nil {
		c.logger.Errorw("[compactExecutor] compact entry segment error", "entry", entry.ID, "err", err.Error())
		return err
	}
	return nil
}

type entryCleanExecutor struct {
	*maintainExecutor
}

func (c *entryCleanExecutor) handleEvent(ctx context.Context, evt *types.EntryEvent) error {
	if evt.Type != events.ActionTypeDestroy && evt.Type != events.ActionTypeClose {
		return nil
	}

	entry := evt.Data
	if evt.Type == events.ActionTypeClose && entry.ParentID != 0 {
		return nil
	}

	if types.IsGroup(entry.Kind) {
		return nil
	}

	task, err := getWaitingTask(ctx, c.recorder, maintainTaskIDEntryCleanup, evt)
	if err != nil {
		return err
	}

	if task != nil {
		return nil
	}

	task = &types.ScheduledTask{
		TaskID:         maintainTaskIDEntryCleanup,
		Status:         types.ScheduledTaskWait,
		RefType:        evt.RefType,
		RefID:          evt.RefID,
		CreatedTime:    time.Now(),
		ExpirationTime: time.Now().Add(time.Hour),
		Event:          *evt,
	}
	return c.recorder.SaveTask(ctx, task)
}

func (c *entryCleanExecutor) execute(ctx context.Context, task *types.ScheduledTask) error {
	entry := task.Event.Data
	if dentry.IsFileOpened(entry.ID) {
		return ErrNeedRetry
	}

	en, err := c.entry.GetEntry(ctx, entry.ID)
	if err != nil {
		c.logger.Errorw("[entryCleanExecutor] get entry failed", "entry", entry.ID, "task", task.ID, "err", err)
		return err
	}

	if !types.IsGroup(en.Kind) {
		err = c.entry.CleanEntryData(ctx, en.ID)
		if err != nil {
			c.logger.Errorw("[entryCleanExecutor] get entry failed", "entry", entry.ID, "task", task.ID, "err", err)
			return err
		}
	}

	err = c.entry.DestroyEntry(ctx, en.ID)
	if err != nil {
		c.logger.Errorw("[entryCleanExecutor] get entry failed", "entry", entry.ID, "task", task.ID, "err", err)
		return err
	}
	return nil
}

func registerMaintainExecutor(
	executors map[string]executor,
	entry dentry.Manager,
	recorder metastore.ScheduledTaskRecorder) {
	e := &maintainExecutor{
		entry:    entry,
		recorder: recorder,
		logger:   logger.NewLogger("maintainExecutor"),
	}

	executors[maintainTaskIDChunkCompact] = &compactExecutor{maintainExecutor: e}
	executors[maintainTaskIDEntryCleanup] = &entryCleanExecutor{maintainExecutor: e}
}
