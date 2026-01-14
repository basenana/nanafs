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
	"time"

	"github.com/basenana/nanafs/pkg/core"
	"github.com/hyponet/eventbus"

	"go.uber.org/zap"

	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
)

const (
	maintainTaskIDChunkCompact = "task.maintain.chunk.compact"
	maintainTaskIDEntryCleanup = "task.maintain.entry.cleanup"
)

const orphanEntryTimeout = 12 * time.Hour

type maintainExecutor struct {
	core      core.Core
	recorder  metastore.ScheduledTaskRecorder
	metastore metastore.Meta
	logger    *zap.SugaredLogger
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
	if core.IsFileOpened(entry.ID) {
		return ErrNeedRetry
	}

	en, err := c.core.GetEntry(ctx, task.Namespace, entry.ID)
	if err != nil {
		c.logger.Errorw("[compactExecutor] query entry error", "entry", entry.ID, "err", err.Error())
		return err
	}
	c.logger.Debugw("[compactExecutor] start compact entry segment", "entry", entry.ID)
	if err = c.core.ChunkCompact(ctx, task.Namespace, en.ID); err != nil {
		c.logger.Errorw("[compactExecutor] compact entry segment error", "entry", entry.ID, "err", err.Error())
		return err
	}
	return nil
}

type entryCleanExecutor struct {
	*maintainExecutor
	orphanThreshold time.Duration
}

func (c *entryCleanExecutor) scanOrphanEntriesTask(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	cutoff := time.Now().Add(-c.orphanThreshold)
	entries, err := c.metastore.ScanOrphanEntries(ctx, cutoff)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		c.logger.Errorw("[orphanScanner] scan orphan entries failed", "err", err.Error())
		return nil // Don't return error to prevent retry
	}

	for _, en := range entries {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := c.createCleanupTask(ctx, en); err != nil {
			c.logger.Errorw("[orphanScanner] create cleanup task failed", "entry", en.ID, "err", err.Error())
		}
	}
	return nil
}

func (c *entryCleanExecutor) createCleanupTask(ctx context.Context, en *types.Entry) error {
	// Check if task already exists
	filter := types.ScheduledTaskFilter{
		RefType: "entry",
		RefID:   en.ID,
		Status:  []string{types.ScheduledTaskWait, types.ScheduledTaskExecuting},
	}
	tasks, err := c.recorder.ListTask(ctx, maintainTaskIDEntryCleanup, filter)
	if err != nil {
		return err
	}
	if len(tasks) > 0 {
		return nil // Task already exists
	}

	// Create new event for the entry
	evt := &types.Event{
		Type:      events.ActionTypeRemove,
		Namespace: en.Namespace,
		RefType:   "entry",
		RefID:     en.ID,
		Data: types.EventData{
			ID:      en.ID,
			Kind:    en.Kind,
			IsGroup: en.IsGroup,
		},
		Time: time.Now(),
	}

	task := &types.ScheduledTask{
		Namespace:      en.Namespace,
		TaskID:         maintainTaskIDEntryCleanup,
		Status:         types.ScheduledTaskWait,
		RefType:        "entry",
		RefID:          en.ID,
		CreatedTime:    time.Now(),
		ExpirationTime: time.Now().Add(time.Hour),
		Event:          *evt,
	}
	return c.recorder.SaveTask(ctx, task)
}

func (c *entryCleanExecutor) handleEvent(evt *types.Event) error {
	if evt.Type != events.ActionTypeRemove {
		return nil
	}

	ctx, canF := context.WithTimeout(context.Background(), time.Hour)
	defer canF()
	task, err := getWaitingTask(ctx, c.recorder, maintainTaskIDEntryCleanup, evt)
	if err != nil {
		c.logger.Errorw("[entryCleanExecutor] list scheduled task error", "entry", evt.RefID, "err", err.Error())
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
	if core.IsFileOpened(entry.ID) {
		return ErrNeedRetry
	}

	en, err := c.metastore.GetEntry(ctx, task.Namespace, entry.ID)
	if err != nil {
		c.logger.Errorw("[entryCleanExecutor] get entry failed", "entry", entry.ID, "task", task.ID, "err", err)
		return err
	}

	if !en.IsGroup {
		err = c.core.CleanEntryData(ctx, task.Namespace, en.ID)
		if err != nil {
			c.logger.Errorw("[entryCleanExecutor] clean entry data failed", "entry", entry.ID, "task", task.ID, "err", err)
			return err
		}
	}

	err = c.core.DestroyEntry(ctx, task.Namespace, en.ID)
	if err != nil {
		c.logger.Errorw("[entryCleanExecutor] destroy entry failed", "entry", entry.ID, "task", task.ID, "err", err)
		return err
	}
	return nil
}

func registerMaintainExecutor(
	d *Dispatcher,
	fsCore core.Core,
	meta metastore.Meta,
	recorder metastore.ScheduledTaskRecorder) error {
	e := &maintainExecutor{core: fsCore, recorder: recorder, metastore: meta, logger: logger.NewLogger("maintainExecutor")}
	ce := &compactExecutor{maintainExecutor: e}
	ee := &entryCleanExecutor{maintainExecutor: e, orphanThreshold: orphanEntryTimeout}

	d.registerExecutor(maintainTaskIDChunkCompact, ce)
	d.registerExecutor(maintainTaskIDEntryCleanup, ee)
	eventbus.Subscribe(events.NamespacedTopic(events.TopicNamespaceFile, events.ActionTypeCompact), ce.handleEvent)
	eventbus.Subscribe(events.NamespacedTopic(events.TopicNamespaceEntry, events.ActionTypeRemove), ee.handleEvent)

	// Register orphan scanner: run every 6 hours, with initial scan
	d.registerRoutineTask(6, ee.scanOrphanEntriesTask)
	return nil
}
