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
	"sort"
	"time"

	"go.uber.org/zap"

	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/basenana/nanafs/workflow/jobrun"
)

type workflowExecutor struct {
	recorder metastore.ScheduledTaskRecorder
	logger   *zap.SugaredLogger
}

const (
	succeedJobLiveTime = time.Hour * 24
	failedJobLiveTime  = time.Hour * 24 * 7
	maxJobsPerWorkflow = 100
)

func (w workflowExecutor) cleanUpFinishJobs(ctx context.Context) error {
	allFinishedJobs := make([]*types.WorkflowJob, 0)
	deleted := 0

	succeedJob, err := w.recorder.ListAllNamespaceWorkflowJobs(ctx, types.JobFilter{Status: jobrun.SucceedStatus})
	if err != nil {
		w.logger.Errorw("[cleanUpFinishJobs] list succeed jobs failed", "err", err)
	}
	for _, j := range succeedJob {
		allFinishedJobs = append(allFinishedJobs, j)
		if time.Since(j.FinishAt) < succeedJobLiveTime {
			continue
		}
		err = w.recorder.DeleteWorkflowJobs(ctx, j.Id)
		if err != nil {
			w.logger.Errorw("[cleanUpFinishJobs] delete succeed jobs failed", "job", j.Id, "err", err)
			continue
		}
		deleted++
	}

	failedJob, err := w.recorder.ListAllNamespaceWorkflowJobs(ctx, types.JobFilter{Status: jobrun.FailedStatus})
	if err != nil {
		w.logger.Errorw("[cleanUpFinishJobs] list failed jobs failed", "err", err)
	}
	for _, j := range failedJob {
		allFinishedJobs = append(allFinishedJobs, j)
		if time.Since(j.FinishAt) < failedJobLiveTime {
			continue
		}
		err = w.recorder.DeleteWorkflowJobs(ctx, j.Id)
		if err != nil {
			w.logger.Errorw("[cleanUpFinishJobs] delete error jobs failed", "job", j.Id, "err", err)
			continue
		}
		deleted++
	}

	if len(allFinishedJobs) > 0 {
		jobsByWF := make(map[string][]*types.WorkflowJob)
		for _, j := range allFinishedJobs {
			key := j.Namespace + ":" + j.Workflow
			jobsByWF[key] = append(jobsByWF[key], j)
		}

		for _, jobs := range jobsByWF {
			if len(jobs) <= maxJobsPerWorkflow {
				continue
			}
			sort.Slice(jobs, func(i, j int) bool {
				return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
			})
			excessIDs := make([]string, 0, len(jobs)-maxJobsPerWorkflow)
			for i := maxJobsPerWorkflow; i < len(jobs); i++ {
				excessIDs = append(excessIDs, jobs[i].Id)
			}
			if err := w.recorder.DeleteWorkflowJobs(ctx, excessIDs...); err != nil {
				w.logger.Errorw("[cleanUpFinishJobs] delete excess jobs failed", "err", err)
			} else {
				w.logger.Infof("[cleanUpFinishJobs] cleaned up %d excess jobs", len(excessIDs))
			}
		}
	}

	w.logger.Infof("[cleanUpFinishJobs] finished, deleted %d jobs by age", deleted)
	return nil
}

func registerWorkflowExecutor(
	d *Dispatcher,
	recorder metastore.ScheduledTaskRecorder) error {
	e := &workflowExecutor{
		recorder: recorder,
		logger:   logger.NewLogger("workflowExecutor"),
	}
	d.registerRoutineTask(6, e.cleanUpFinishJobs)
	return nil
}
