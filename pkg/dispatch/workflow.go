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
	"github.com/basenana/nanafs/pkg/workflow"
	"go.uber.org/zap"
	"time"
)

const (
	workflowTaskIDTrigger = "task.workflow.trigger"
)

type workflowAction struct {
	entry    dentry.Manager
	manager  workflow.Manager
	recorder metastore.ScheduledTaskRecorder
	logger   *zap.SugaredLogger
}

func (w workflowAction) handleEvent(ctx context.Context, evt *types.Event) error {
	if evt.Type != events.ActionTypeClose || !dentry.IsFileOpened(evt.RefID) {
		return nil
	}

	task, err := getWaitingTask(ctx, w.recorder, workflowTaskIDTrigger, evt)
	if err != nil {
		w.logger.Errorw("[workflowAction] list scheduled task error", "entry", evt.RefID, "err", err.Error())
		return err
	}

	if task != nil {
		return nil
	}

	en, err := w.entry.GetEntry(ctx, evt.RefID)
	if err != nil {
		w.logger.Errorw("[workflowAction] get entry failed", "entry", evt.RefID, "err", err)
		return err
	}

	wfList, err := w.recorder.ListWorkflow(ctx)
	if err != nil {
		w.logger.Errorw("[workflowAction] list workflow failed", "entry", evt.RefID, "err", err)
		return err
	}

	for _, wf := range wfList {
		if !wf.Enable {
			continue
		}
		if !en.RuleMatched(ctx, wf.Rule) {
			continue
		}

		// trigger workflow
		task = &types.ScheduledTask{
			TaskID:         workflowTaskIDTrigger,
			Status:         types.ScheduledTaskInitial,
			RefType:        evt.RefType,
			RefID:          evt.RefID,
			CreatedTime:    time.Now(),
			ExecutionTime:  time.Now(),
			ExpirationTime: time.Now().Add(time.Hour),
			Event:          *evt,
		}
		err = w.recorder.SaveTask(ctx, task)
		if err != nil {
			w.logger.Errorw("[workflowAction] workflow trigger failed", "entry", evt.RefID, "workflow", wf.Id, "err", err)
			continue
		}
	}

	return nil
}

func (w workflowAction) execute(ctx context.Context, task *types.ScheduledTask) error {
	//TODO implement me
	panic("implement me")
}
