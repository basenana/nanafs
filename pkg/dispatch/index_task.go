/*
 Copyright 2024 NanaFS Authors.

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
	"sync"
	"time"

	"github.com/hyponet/eventbus"
	"go.uber.org/zap"

	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/indexer"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/basenana/nanafs/workflow"
)

const (
	taskIDIndexReindex      = "task.index.reindex"
	taskIDUpdateDocumentURI = "task.index.update_uri"
)

type indexExecutor struct {
	recorder  metastore.ScheduledTaskRecorder
	workflow  workflow.Workflow
	metastore metastore.Meta
	indexer   indexer.Indexer
	mux       sync.Mutex
	logger    *zap.SugaredLogger
}

func (i *indexExecutor) handleEvent(evt *types.Event) error {
	if evt.Type != events.ActionTypeIndex {
		return nil
	}

	if evt.Data.URI == "" {
		i.logger.Warn("[indexExecutor] event is missing URI")
		return nil
	}

	i.mux.Lock()
	defer i.mux.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()

	task, err := getWaitingTask(ctx, i.recorder, taskIDIndexReindex, evt)
	if err != nil {
		i.logger.Errorw("[indexExecutor] list scheduled task error", "entry", evt.RefID, "err", err.Error())
		return err
	}
	if task != nil {
		return nil
	}

	task = &types.ScheduledTask{
		Namespace:      evt.Namespace,
		TaskID:         taskIDIndexReindex,
		Status:         types.ScheduledTaskWait,
		RefType:        evt.RefType,
		RefID:          evt.RefID,
		CreatedTime:    time.Now(),
		ExpirationTime: time.Now().Add(time.Hour),
		Event:          *evt,
	}
	if err = i.recorder.SaveTask(ctx, task); err != nil {
		i.logger.Errorw("[indexExecutor] save task to waiting error", "entry", evt.RefID, "err", err.Error())
		return err
	}
	return nil
}

func (i *indexExecutor) execute(ctx context.Context, task *types.ScheduledTask) error {
	entryURI := task.Event.Data.URI
	docloaderWFID := workflow.BuildInWorkflowID(task.Namespace, "docloader")
	_, err := i.workflow.TriggerWorkflow(
		ctx,
		task.Namespace,
		docloaderWFID,
		types.WorkflowTarget{Entries: []string{entryURI}},
		workflow.JobAttr{Reason: "index version mismatch"},
	)
	if err != nil {
		i.logger.Errorw("[indexExecutor] trigger workflow failed", "uri", entryURI, "err", err.Error())
		return err
	}
	return nil
}

type documentURIUpdateExecutor struct {
	recorder  metastore.ScheduledTaskRecorder
	metastore metastore.Meta
	indexer   indexer.Indexer
	logger    *zap.SugaredLogger
}

func (u *documentURIUpdateExecutor) handleEvent(evt *types.Event) error {
	if evt.Type != events.ActionTypeChangeParent {
		return nil
	}

	ctx, canF := context.WithTimeout(context.Background(), time.Hour)
	defer canF()

	task, err := getWaitingTask(ctx, u.recorder, taskIDUpdateDocumentURI, evt)
	if err != nil {
		u.logger.Errorw("[documentURIUpdateExecutor] list scheduled task error", "entry", evt.RefID, "err", err.Error())
		return err
	}

	if task != nil {
		return nil
	}

	task = &types.ScheduledTask{
		Namespace:      evt.Namespace,
		TaskID:         taskIDUpdateDocumentURI,
		Status:         types.ScheduledTaskWait,
		RefType:        evt.RefType,
		RefID:          evt.RefID,
		CreatedTime:    time.Now(),
		ExpirationTime: time.Now().Add(time.Hour),
		Event:          *evt,
	}
	return u.recorder.SaveTask(ctx, task)
}

func (u *documentURIUpdateExecutor) execute(ctx context.Context, task *types.ScheduledTask) error {
	newURI := task.Event.Data.URI
	entryID := task.Event.RefID

	en, err := u.metastore.GetEntry(ctx, task.Namespace, entryID)
	if err != nil {
		u.logger.Errorw("[documentURIUpdateExecutor] get entry failed", "entry", entryID, "err", err)
		return err
	}

	if en.IsGroup {
		if err = u.indexer.UpdateChildrenURI(ctx, task.Namespace, entryID, newURI); err != nil {
			u.logger.Errorw("[documentURIUpdateExecutor] update child documents failed", "group", entryID, "err", err)
		}
		return nil
	}

	if err = u.indexer.UpdateURI(ctx, task.Namespace, entryID, newURI); err != nil {
		u.logger.Errorw("[documentURIUpdateExecutor] update document uri failed", "entry", entryID, "err", err)
	}

	return nil
}

func registerIndexExecutors(d *Dispatcher, recorder metastore.ScheduledTaskRecorder, meta metastore.Meta, wf workflow.Workflow, idx indexer.Indexer) error {
	e := &indexExecutor{
		recorder:  recorder,
		workflow:  wf,
		metastore: meta,
		indexer:   idx,
		logger:    logger.NewLogger("indexExecutor"),
	}
	d.registerExecutor(taskIDIndexReindex, e)
	eventbus.Subscribe(
		events.NamespacedTopic(events.TopicNamespaceEntry, events.ActionTypeIndex),
		e.handleEvent,
	)

	ue := &documentURIUpdateExecutor{
		recorder:  recorder,
		metastore: meta,
		indexer:   idx,
		logger:    logger.NewLogger("documentURIUpdateExecutor"),
	}
	d.registerExecutor(taskIDUpdateDocumentURI, ue)
	eventbus.Subscribe(
		events.NamespacedTopic(events.TopicNamespaceEntry, events.ActionTypeChangeParent),
		ue.handleEvent,
	)

	return nil
}
