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

package workflow

import (
	"context"
	"errors"
	flowcontroller "github.com/basenana/go-flow/controller"
	goflowctrl "github.com/basenana/go-flow/controller"
	"github.com/basenana/go-flow/flow"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"sync"
	"time"
)

var (
	ErrJobNotFound = errors.New("job not found")
)

const (
	fetchPendingJobInterval = time.Minute * 5
)

type Runner struct {
	ctrl *flowcontroller.FlowController

	stopCh   chan struct{}
	recorder metastore.ScheduledTaskRecorder
	config   config.Workflow
	logger   *zap.SugaredLogger

	sync.RWMutex
}

func InitWorkflowRunner(recorder metastore.ScheduledTaskRecorder) (*Runner, error) {
	runner := &Runner{
		recorder: recorder,
		logger:   logger.NewLogger("workflowRuntime"),
	}

	opt := goflowctrl.Option{Storage: &storageWrapper{recorder: recorder, logger: runner.logger}}
	flowCtrl, err := goflowctrl.NewFlowController(opt)
	if err != nil {
		return nil, err
	}
	if err = flowCtrl.Register(&Job{}); err != nil {
		return nil, err
	}
	runner.ctrl = flowCtrl

	return runner, nil
}

func (r *Runner) Start(stopCh chan struct{}) error {
	t := time.NewTicker(fetchPendingJobInterval)

	retryJobs, err := r.recorder.ListWorkflowJob(context.Background(), types.JobFilter{Status: flow.RunningStatus})
	if err != nil {
		r.logger.Errorw("list need retry jobs failed", "err", err)
		return err
	}

	for i := range retryJobs {
		r.triggerJob(context.Background(), retryJobs[i])
	}

	go func() {
		for {
			select {
			case <-stopCh:
				r.logger.Info("stopped")
				return
			case <-t.C:
				r.logger.Debug("fetch pending jobs")
			}

			pendingJobs, err := r.recorder.ListWorkflowJob(context.Background(), types.JobFilter{Status: string(flow.CreatingStatus)})
			if err != nil {
				r.logger.Errorw("list pending jobs failed", "err", err)
				continue
			}
			for i := range pendingJobs {
				r.triggerJob(context.Background(), pendingJobs[i])
			}
		}
	}()
	return nil
}

func (r *Runner) PauseFlow(flowId string) error {
	return r.ctrl.PauseFlow(flow.FID(flowId))
}

func (r *Runner) ResumeFlow(flowId string) error {
	return r.ctrl.ResumeFlow(flow.FID(flowId))
}

func (r *Runner) CancelFlow(flowId string) error {
	return r.ctrl.CancelFlow(flow.FID(flowId))
}

func (r *Runner) triggerJob(ctx context.Context, job *types.WorkflowJob) {
	if job.Target.EntryID != nil {
		jobs, err := r.recorder.ListWorkflowJob(ctx, types.JobFilter{TargetEntry: *job.Target.EntryID, Status: flow.RunningStatus})
		if err != nil {
			r.logger.Errorw("query target entry running job failed", "err", err)
			return
		}
		if len(jobs) > 0 {
			return
		}
	}

	if err := r.ctrl.TriggerFlow(ctx, flow.FID(job.Id)); err != nil {
		r.logger.Errorw("trigger job flow failed", "job", job.Id, "err", err)
	}
}
