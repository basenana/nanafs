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
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/basenana/nanafs/workflow"
)

const (
	taskIDIndexReindex = "task.index.reindex"
)

type indexExecutor struct {
	recorder  metastore.ScheduledTaskRecorder
	workflow  workflow.Workflow
	metastore metastore.Meta
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

func registerIndexExecutor(d *Dispatcher, recorder metastore.ScheduledTaskRecorder, meta metastore.Meta, wf workflow.Workflow) error {
	e := &indexExecutor{
		recorder:  recorder,
		workflow:  wf,
		metastore: meta,
		logger:    logger.NewLogger("indexExecutor"),
	}
	d.registerExecutor(taskIDIndexReindex, e)
	eventbus.Subscribe(
		events.NamespacedTopic(events.TopicNamespaceEntry, events.ActionTypeIndex),
		e.handleEvent,
	)
	return nil
}
