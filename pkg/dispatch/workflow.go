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

	"go.uber.org/zap"

	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/document"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/basenana/nanafs/workflow/jobrun"
)

type workflowExecutor struct {
	entry    dentry.Manager
	doc      document.Manager
	recorder metastore.ScheduledTaskRecorder
	logger   *zap.SugaredLogger
}

const (
	succeedJobLiveTime = time.Hour * 24
	failedJobLiveTime  = time.Hour * 24 * 7
)

func (w workflowExecutor) cleanUpFinishJobs(ctx context.Context) error {
	needCleanUp, deleted := 0, 0
	succeedJob, err := w.recorder.ListAllNamespaceWorkflowJobs(ctx, types.JobFilter{Status: jobrun.SucceedStatus})
	if err != nil {
		w.logger.Errorw("[cleanUpFinishJobs] list succeed jobs failed", "err", err)
	}
	for _, j := range succeedJob {
		if time.Since(j.FinishAt) < succeedJobLiveTime {
			continue
		}
		needCleanUp += 1
		err = w.recorder.DeleteWorkflowJobs(ctx, j.Id)
		if err != nil {
			w.logger.Errorw("[cleanUpFinishJobs] delete succeed jobs failed", "job", j.Id, "err", err)
			continue
		}
		deleted += 1
	}

	failedJob, err := w.recorder.ListAllNamespaceWorkflowJobs(ctx, types.JobFilter{Status: jobrun.FailedStatus})
	if err != nil {
		w.logger.Errorw("[cleanUpFinishJobs] list succeed jobs failed", "err", err)
	}
	for _, j := range failedJob {
		if time.Since(j.FinishAt) < failedJobLiveTime {
			continue
		}
		needCleanUp += 1
		err = w.recorder.DeleteWorkflowJobs(ctx, j.Id)
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
	recorder metastore.ScheduledTaskRecorder) error {
	e := &workflowExecutor{
		entry:    entry,
		doc:      doc,
		recorder: recorder,
		logger:   logger.NewLogger("workflowExecutor"),
	}
	d.registerRoutineTask(6, e.cleanUpFinishJobs)
	return nil
}
