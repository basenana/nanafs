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
	"fmt"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/rule"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/pkg/workflow"
	"github.com/basenana/nanafs/pkg/workflow/jobrun"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"time"
)

const (
	workflowAutoTriggerExecID = "task.workflow.auto_trigger"
)

type workflowExecutor struct {
	entry    dentry.Manager
	manager  workflow.Manager
	recorder metastore.ScheduledTaskRecorder
	logger   *zap.SugaredLogger
}

func (w workflowExecutor) handleEvent(evt *types.EntryEvent) error {
	if evt.Type != events.ActionTypeCreate {
		return nil
	}
	if evt.RefType != "entry" {
		return nil
	}

	time.Sleep(time.Second * 15) // delay for more update after create
	ctx, canF := context.WithTimeout(context.Background(), time.Hour)
	defer canF()
	en, err := w.entry.GetEntry(ctx, evt.RefID)
	if err != nil {
		w.logger.Errorw("[workflowAutoTrigger] get entry failed", "entry", evt.RefID, "err", err)
		return err
	}
	ed, err := w.entry.GetEntryExtendData(ctx, en.ID)
	if err != nil {
		w.logger.Errorw("[workflowAutoTrigger] get entry extend data failed", "entry", evt.RefID, "err", err)
		return err
	}
	labels, err := w.entry.GetEntryLabels(ctx, en.ID)
	if err != nil {
		w.logger.Errorw("[workflowAutoTrigger] get entry labels failed", "entry", evt.RefID, "err", err)
		return err
	}

	wfList, err := w.recorder.ListWorkflow(ctx)
	if err != nil {
		w.logger.Errorw("[workflowAutoTrigger] list workflow failed", "entry", evt.RefID, "err", err)
		return err
	}

	for _, wf := range wfList {
		if !wf.Enable {
			continue
		}

		if !rule.Filter(wf.Rule, en, &ed, &labels) {
			continue
		}

		task := &types.ScheduledTask{
			TaskID:         workflowAutoTriggerExecID,
			Status:         types.ScheduledTaskWait,
			RefType:        evt.RefType,
			RefID:          evt.RefID,
			CreatedTime:    time.Now(),
			ExpirationTime: time.Now().Add(defaultTimeout),
			Event:          *evt,
		}
		if err = w.recorder.SaveTask(ctx, task); err != nil {
			w.logger.Errorw("[workflowAutoTrigger] save task to waiting error", "entry", evt.RefID, "err", err.Error())
			return err
		}
		w.logger.Infow("[workflowAutoTrigger] workflow rule matched, job pre-created", "entry", evt.RefID, "workflow", wf.Id)
		return nil
	}

	return nil
}

func (w workflowExecutor) execute(ctx context.Context, task *types.ScheduledTask) error {
	entry := task.Event.Data
	if dentry.IsFileOpened(entry.ID) {
		return ErrNeedRetry
	}

	en, err := w.entry.GetEntry(ctx, entry.ID)
	if err != nil {
		w.logger.Errorw("[workflowAutoTrigger] query entry error", "entry", entry.ID, "err", err.Error())
		return err
	}
	ed, err := w.entry.GetEntryExtendData(ctx, en.ID)
	if err != nil {
		w.logger.Errorw("[workflowAutoTrigger] get entry extend data failed", "entry", en.ID, "err", err)
		return err
	}
	labels, err := w.entry.GetEntryLabels(ctx, en.ID)
	if err != nil {
		w.logger.Errorw("[workflowAutoTrigger] get entry labels failed", "entry", en.ID, "err", err)
		return err
	}

	wfList, err := w.recorder.ListWorkflow(ctx)
	if err != nil {
		w.logger.Errorw("[workflowAutoTrigger] list workflow failed", "entry", en.ID, "err", err)
		return err
	}

	var job *types.WorkflowJob
	for _, wf := range wfList {
		if !wf.Enable {
			continue
		}

		if !rule.Filter(wf.Rule, en, &ed, &labels) {
			continue
		}

		pendingJob, err := w.recorder.ListWorkflowJob(ctx, types.JobFilter{WorkFlowID: wf.Id, Status: jobrun.InitializingStatus, TargetEntry: en.ID})
		if err != nil {
			w.logger.Errorw("[workflowAutoTrigger] query pending job failed", "entry", en.ID, "workflow", wf.Id, "err", err)
			continue
		}

		if len(pendingJob) > 0 {
			continue
		}

		// trigger workflow
		tgt := types.WorkflowTarget{}
		if types.IsGroup(en.Kind) {
			tgt.ParentEntryID = en.ID
		} else {
			tgt.EntryID = en.ID
			tgt.ParentEntryID = en.ParentID
		}
		job, err = w.manager.TriggerWorkflow(ctx, wf.Id, tgt, workflow.JobAttr{Reason: fmt.Sprintf("event: entry created")})
		if err != nil {
			w.logger.Errorw("[workflowAutoTrigger] workflow trigger failed", "entry", en.ID, "workflow", wf.Id, "err", err)
			continue
		}
		w.logger.Infow("[workflowAutoTrigger] new workflow job", "entry", en.ID, "workflow", wf.Id, "job", job.Id)
	}
	return nil
}

func registerWorkflowExecutor(
	executors map[string]executor,
	entry dentry.Manager,
	wfMgr workflow.Manager,
	recorder metastore.ScheduledTaskRecorder) error {
	e := &workflowExecutor{
		entry:    entry,
		manager:  wfMgr,
		recorder: recorder,
		logger:   logger.NewLogger("workflowExecutor"),
	}
	executors[workflowAutoTriggerExecID] = e
	if _, err := events.Subscribe(events.NamespacedTopic(events.TopicNamespaceEntry, events.ActionTypeCreate), e.handleEvent); err != nil {
		return err
	}
	return nil
}
