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
	"github.com/basenana/go-flow/flow"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/pkg/workflow"
	"go.uber.org/zap"
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
	if evt.RefType != "object" {
		return nil
	}

	_, err := w.entry.GetEntry(ctx, evt.RefID)
	if err != nil {
		w.logger.Errorw("[workflowAction] get entry failed", "entry", evt.RefID, "err", err)
		return err
	}

	wfList, err := w.recorder.ListWorkflow(ctx)
	if err != nil {
		w.logger.Errorw("[workflowAction] list workflow failed", "entry", evt.RefID, "err", err)
		return err
	}

	var job *types.WorkflowJob
	for _, wf := range wfList {
		if !wf.Enable {
			continue
		}
		// TODO
		//if !en.RuleMatched(ctx, wf.Rule) {
		//	continue
		//}

		pendingJob, err := w.recorder.ListWorkflowJob(ctx, types.JobFilter{WorkFlowID: wf.Id, Status: flow.InitializingStatus, TargetEntry: evt.RefID})
		if err != nil {
			w.logger.Errorw("[workflowAction] query pending job failed", "entry", evt.RefID, "workflow", wf.Id, "err", err)
			continue
		}
		if len(pendingJob) > 0 {
			continue
		}

		// trigger workflow
		job, err = w.manager.TriggerWorkflow(ctx, wf.Id, evt.RefID, workflow.JobAttr{Reason: fmt.Sprintf("event: %s", evt.Type)})
		if err != nil {
			w.logger.Errorw("[workflowAction] workflow trigger failed", "entry", evt.RefID, "workflow", wf.Id, "err", err)
			continue
		}
		w.logger.Infow("[workflowAction] new workflow job", "entry", evt.RefID, "workflow", wf.Id, "job", job.Id)
	}

	return nil
}

func (w workflowAction) execute(ctx context.Context, task *types.ScheduledTask) error {
	return nil
}
