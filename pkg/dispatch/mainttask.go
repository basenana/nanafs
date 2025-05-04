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
	"github.com/basenana/nanafs/pkg/core"
	"github.com/hyponet/eventbus"
	"time"

	"go.uber.org/zap"

	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
)

const (
	maintainTaskIDChunkCompact = "task.maintain.chunk.compact"
	maintainTaskIDEntryCleanup = "task.maintain.entry.cleanup"
)

type maintainExecutor struct {
	core     core.Core
	recorder metastore.ScheduledTaskRecorder
	logger   *zap.SugaredLogger
}

type compactExecutor struct {
	*maintainExecutor
}

func (c *compactExecutor) handleEvent(evt *types.Event) error {
	if evt.Type != events.ActionTypeCompact {
		return nil
	}

	ctx, canF := context.WithTimeout(context.Background(), time.Hour)
	defer canF()

	task, err := getWaitingTask(ctx, c.recorder, maintainTaskIDChunkCompact, evt)
	if err != nil {
		c.logger.Errorw("[compactExecutor] list scheduled task error", "entry", evt.RefID, "err", err.Error())
		return err
	}

	if task != nil {
		return nil
	}

	task = &types.ScheduledTask{
		Namespace:      evt.Namespace,
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

	en, err := c.core.GetEntry(ctx, task.Namespace, entry.ID)
	if err != nil {
		c.logger.Errorw("[compactExecutor] query entry error", "entry", entry.ID, "err", err.Error())
		return err
	}
	c.logger.Debugw("[compactExecutor] start compact entry segment", "entry", entry.ID)
	if err = c.core.ChunkCompact(ctx, en.ID); err != nil {
		c.logger.Errorw("[compactExecutor] compact entry segment error", "entry", entry.ID, "err", err.Error())
		return err
	}
	return nil
}

type entryCleanExecutor struct {
	*maintainExecutor
}

func (c *entryCleanExecutor) handleEvent(evt *types.Event) error {
	if evt.Type != events.ActionTypeDestroy {
		return nil
	}

	entry := evt.Data
	if evt.Type == events.ActionTypeClose && entry.ParentID != 0 {
		return nil
	}

	ctx, canF := context.WithTimeout(context.Background(), time.Hour)
	defer canF()
	task, err := getWaitingTask(ctx, c.recorder, maintainTaskIDEntryCleanup, evt)
	if err != nil {
		return err
	}

	if task != nil {
		return nil
	}

	task = &types.ScheduledTask{
		Namespace:      evt.Namespace,
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

	en, err := c.core.GetEntry(ctx, task.Namespace, entry.ID)
	if err != nil {
		c.logger.Errorw("[entryCleanExecutor] get entry failed", "entry", entry.ID, "task", task.ID, "err", err)
		return err
	}

	if !en.IsGroup {
		err = c.core.CleanEntryData(ctx, en.ID)
		if err != nil {
			c.logger.Errorw("[entryCleanExecutor] get entry failed", "entry", entry.ID, "task", task.ID, "err", err)
			return err
		}
	}

	err = c.core.DestroyEntry(ctx, en.ID)
	if err != nil {
		c.logger.Errorw("[entryCleanExecutor] get entry failed", "entry", entry.ID, "task", task.ID, "err", err)
		return err
	}
	return nil
}

func registerMaintainExecutor(
	d *Dispatcher,
	fsCore core.Core,
	recorder metastore.ScheduledTaskRecorder) error {
	e := &maintainExecutor{core: fsCore, recorder: recorder, logger: logger.NewLogger("maintainExecutor")}
	ce := &compactExecutor{maintainExecutor: e}
	ee := &entryCleanExecutor{maintainExecutor: e}

	d.registerExecutor(maintainTaskIDChunkCompact, ce)
	d.registerExecutor(maintainTaskIDEntryCleanup, ee)
	eventbus.Subscribe(events.NamespacedTopic(events.TopicNamespaceFile, events.ActionTypeCompact), ce.handleEvent)
	eventbus.Subscribe(events.NamespacedTopic(events.TopicNamespaceEntry, events.ActionTypeDestroy), ee.handleEvent)
	return nil
}
