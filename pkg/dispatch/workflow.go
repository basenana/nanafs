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
	"github.com/basenana/nanafs/pkg/document"
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/rule"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/pkg/workflow"
	"github.com/basenana/nanafs/pkg/workflow/jobrun"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"time"
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

func (w workflowExecutor) handleEntryEvent(evt *types.EntryEvent) error {
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
func (w workflowExecutor) handleDocEvent(evt *types.EntryEvent) error {
	if evt.Type != events.ActionTypeCreate {
		return nil
	}
	if evt.RefType != "document" {
		return nil
	}
	ctx := context.Background()
	doc, err := w.doc.GetDocument(ctx, evt.Data.ID)
	if err != nil {
		w.logger.Errorw("[handleDocEvent] query document failed", "document", evt.Data.ID, "err", err)
		return err
	}

	if doc.OID == 0 {
		return nil
	}
	entry, err := w.entry.GetEntry(ctx, doc.OID)
	if err != nil {
		w.logger.Errorw("[handleDocEvent] query document entry failed", "document", doc.ID, "entry", doc.OID, "err", err)
		return err
	}

	ed, err := w.entry.GetEntryExtendData(ctx, entry.ParentID)
	if err != nil {
		err = fmt.Errorf("get parent entry extend data error: %s", err)
		w.logger.Errorw("[handleDocEvent] query document parent entry ext data failed", "document", doc.ID, "parent", entry.ParentID, "err", err)
		return err
	}

	properties := make(map[string]string)
	if ed.Properties.Fields != nil {
		for k, v := range ed.Properties.Fields {
			val := v.Value
			if v.Encoded {
				val, err = utils.DecodeBase64String(val)
				if err != nil {
					w.logger.Warnw("[handleDocEvent] decode extend property value failed", "key", k)
					continue
				}
			}
			properties[k] = val
		}
	}

	if properties[propertyKeyFridayEnabled] != "true" {
		return nil
	}

	job, err := w.manager.TriggerWorkflow(ctx, workflow.BuildInWorkflowSummary,
		types.WorkflowTarget{EntryID: entry.ID, ParentEntryID: entry.ParentID},
		workflow.JobAttr{Reason: "event: auto summary"})
	if err != nil {
		w.logger.Infow("[handleDocEvent] document has enable friday auto summary, but trigger failed", "document", doc.ID, "err", err)
		return err
	}
	w.logger.Infow("[handleDocEvent] document has enable friday auto summary", "document", doc.ID, "job", job.Id)
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
	if _, err := events.Subscribe(
		events.NamespacedTopic(events.TopicNamespaceDocument, events.ActionTypeCreate), e.handleDocEvent); err != nil {
		return err
	}
	return nil
}
