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
	"time"

	"go.uber.org/zap"

	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/document"
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/rule"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/pkg/workflow"
	"github.com/basenana/nanafs/pkg/workflow/jobrun"
	"github.com/basenana/nanafs/utils/logger"
)

const (
	workflowAutoTriggerExecID = "task.workflow.auto_trigger"
	propertyKeyFridayEnabled  = "org.basenana.plugin.friday/enabled"
)

type workflowExecutor struct {
	entry    dentry.Manager
	doc      document.Manager
	manager  workflow.Manager
	recorder metastore.ScheduledTaskRecorder
	logger   *zap.SugaredLogger
}

func (w workflowExecutor) handleEntryEvent(evt *types.Event) error {
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

	ctx = types.WithNamespace(ctx, types.NewNamespace(en.Namespace))
	properties, err := w.entry.ListEntryProperty(ctx, en.ID)
	if err != nil {
		w.logger.Errorw("[workflowAutoTrigger] get entry properties failed", "entry", evt.RefID, "err", err)
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
	if en.Namespace != types.GlobalNamespaceValue {
		globalWfList, err := w.recorder.ListGlobalWorkflow(ctx)
		if err != nil {
			w.logger.Errorw("[workflowAutoTrigger] list global workflow failed", "entry", en.ID, "err", err)
			return err
		}
		wfList = append(wfList, globalWfList...)
	}

	for _, wf := range wfList {
		if wf.Namespace != types.GlobalNamespaceValue && wf.Namespace != en.Namespace {
			continue
		}
		if !wf.Enable {
			continue
		}

		if !rule.Filter(wf.Rule, en, &properties, &labels) {
			continue
		}

		task := &types.ScheduledTask{
			TaskID:         workflowAutoTriggerExecID,
			Status:         types.ScheduledTaskWait,
			Namespace:      en.Namespace,
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
		quickIdleWakeup()
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
	ctx = types.WithNamespace(ctx, types.NewNamespace(en.Namespace))
	properties, err := w.entry.ListEntryProperty(ctx, en.ID)
	if err != nil {
		w.logger.Errorw("[workflowAutoTrigger] get entry properties failed", "entry", en.ID, "err", err)
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

	if en.Namespace != types.GlobalNamespaceValue {
		globalWfList, err := w.recorder.ListGlobalWorkflow(ctx)
		if err != nil {
			w.logger.Errorw("[workflowAutoTrigger] list global workflow failed", "entry", en.ID, "err", err)
			return err
		}
		wfList = append(wfList, globalWfList...)
	}

	var job *types.WorkflowJob
	for _, wf := range wfList {
		if wf.Namespace != types.GlobalNamespaceValue && wf.Namespace != en.Namespace {
			continue
		}

		if !wf.Enable {
			continue
		}

		if !rule.Filter(wf.Rule, en, &properties, &labels) {
			continue
		}

		sameTargetJob, err := w.recorder.ListWorkflowJob(ctx, types.JobFilter{WorkFlowID: wf.Id, TargetEntry: en.ID})
		if err != nil {
			w.logger.Errorw("[workflowAutoTrigger] query same target job failed", "entry", en.ID, "workflow", wf.Id, "err", err)
			continue
		}

		needSkip := false
		for _, j := range sameTargetJob {
			if j.Status == jobrun.InitializingStatus || j.Status == jobrun.PendingStatus || j.Status == jobrun.RunningStatus {
				w.logger.Debugw("[workflowAutoTrigger] found same target job, skip this event", "entry", en.ID, "job", j.Id)
				needSkip = true
				break
			}
		}
		if needSkip {
			continue
		}

		// trigger workflow
		tgt := types.WorkflowTarget{}
		if en.IsGroup {
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

const (
	succeedJobLiveTime = time.Hour * 24
	failedJobLiveTime  = time.Hour * 24 * 7
)

func (w workflowExecutor) cleanUpFinishJobs(ctx context.Context) error {
	needCleanUp, deleted := 0, 0
	succeedJob, err := w.recorder.ListWorkflowJob(ctx, types.JobFilter{Status: jobrun.SucceedStatus})
	if err != nil {
		w.logger.Errorw("[cleanUpFinishJobs] list succeed jobs failed", "err", err)
	}
	for _, j := range succeedJob {
		if time.Since(j.FinishAt) < succeedJobLiveTime {
			continue
		}
		needCleanUp += 1
		err = w.recorder.DeleteWorkflowJob(ctx, j.Id)
		if err != nil {
			w.logger.Errorw("[cleanUpFinishJobs] delete succeed jobs failed", "job", j.Id, "err", err)
			continue
		}
		deleted += 1
	}

	failedJob, err := w.recorder.ListWorkflowJob(ctx, types.JobFilter{Status: jobrun.FailedStatus})
	if err != nil {
		w.logger.Errorw("[cleanUpFinishJobs] list succeed jobs failed", "err", err)
	}
	for _, j := range failedJob {
		if time.Since(j.FinishAt) < failedJobLiveTime {
			continue
		}
		needCleanUp += 1
		err = w.recorder.DeleteWorkflowJob(ctx, j.Id)
		if err != nil {
			w.logger.Errorw("[cleanUpFinishJobs] delete error jobs failed", "job", j.Id, "err", err)
			continue
		}
		deleted += 1
	}

	if needCleanUp > 0 {
		w.logger.Infof("[cleanUpFinishJobs] finish, need cleanup %d jobs, deleted %d", needCleanUp, deleted)
	}
	return nil
}

func registerWorkflowExecutor(
	d *Dispatcher,
	entry dentry.Manager,
	doc document.Manager,
	wfMgr workflow.Manager,
	recorder metastore.ScheduledTaskRecorder) error {
	e := &workflowExecutor{
		entry:    entry,
		doc:      doc,
		manager:  wfMgr,
		recorder: recorder,
		logger:   logger.NewLogger("workflowExecutor"),
	}
	d.registerExecutor(workflowAutoTriggerExecID, e)
	d.registerRoutineTask(6, e.cleanUpFinishJobs)
	if _, err := events.Subscribe(
		events.NamespacedTopic(events.TopicNamespaceEntry, events.ActionTypeCreate), e.handleEntryEvent); err != nil {
		return err
	}
	return nil
}
