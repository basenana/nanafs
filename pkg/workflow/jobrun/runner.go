/*
   Copyright 2022 Go-Flow Authors

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

package jobrun

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/pkg/workflow/fsm"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"sync"
	"time"
)

const (
	localExecName = "local"
)

type Runner interface {
	Start(ctx context.Context) error
	Pause() error
	Resume() error
	Cancel() error
}

func NewRunner(j *types.WorkflowJob, recorder metastore.ScheduledTaskRecorder) Runner {
	return &runner{job: j, recorder: recorder, logger: logger.NewLogger("jobrun").With(zap.String("job", j.Id))}
}

type runner struct {
	job      *types.WorkflowJob
	ctx      context.Context
	canF     context.CancelFunc
	fsm      *fsm.FSM
	dag      *DAG
	executor Executor
	stopCh   chan struct{}
	started  bool
	mux      sync.Mutex

	recorder metastore.ScheduledTaskRecorder
	logger   *zap.SugaredLogger
}

func (r *runner) Start(ctx context.Context) (err error) {
	if IsFinishedStatus(r.job.Status) {
		return
	}

	r.ctx, r.canF = context.WithCancel(ctx)
	if err = r.initial(); err != nil {
		r.logger.Errorf("job initial failed: %s", err)
		return err
	}

	defer func() {
		if deferErr := r.recorder.SaveWorkflowJob(context.TODO(), r.job); deferErr != nil {
			r.logger.Errorw("save job to metabase failed", "err", deferErr)
		}
	}()

	r.dag, err = buildDAG(r.job.Steps)
	if err != nil {
		r.job.Status = FailedStatus
		r.logger.Errorw("build dag failed", "err", err)
		return err
	}
	r.executor, err = newExecutor(localExecName, r.job)
	if err != nil {
		r.job.Status = FailedStatus
		r.logger.Errorw("build executor failed", "err", err)
		return err
	}
	r.fsm = buildFlowFSM(r)
	r.stopCh = make(chan struct{})

	if !r.waitingForRunning(ctx) {
		return
	}

	if err = r.executor.Setup(ctx); err != nil {
		r.job.Status = ErrorStatus
		r.logger.Errorw("setup executor failed", "err", err)
		return err
	}

	if err = r.pushEvent2FlowFSM(fsm.Event{Type: TriggerEvent, Status: r.job.Status, Obj: r.job}); err != nil {
		r.job.Status = ErrorStatus
		r.logger.Errorw("trigger job failed", "err", err)
		return err
	}

	<-r.stopCh
	r.executor.Teardown(context.TODO())
	return nil
}

func (r *runner) Pause() error {
	if r.job.Status == RunningStatus {
		return r.pushEvent2FlowFSM(fsm.Event{Type: ExecutePauseEvent, Obj: r.job})
	}
	return nil
}

func (r *runner) Resume() error {
	if r.job.Status == PausedStatus {
		return r.pushEvent2FlowFSM(fsm.Event{Type: ExecuteResumeEvent, Obj: r.job})
	}
	return nil
}

func (r *runner) Cancel() error {
	if r.job.Status == SucceedStatus || r.job.Status == FailedStatus || r.job.Status == ErrorStatus {
		return nil
	}
	return r.pushEvent2FlowFSM(fsm.Event{Type: ExecuteCancelEvent, Obj: r.job})
}

func (r *runner) handleJobRun(event fsm.Event) error {
	r.logger.Info("job ready to run")
	if err := r.recorder.SaveWorkflowJob(r.ctx, r.job); err != nil {
		r.logger.Errorf("save job status failed: %s", err)
		return err
	}

	r.mux.Lock()
	defer r.mux.Unlock()
	if r.started {
		return nil
	}
	r.started = true

	go func() {
		var (
			isFinish bool
			err      error
		)
		defer func() {
			r.logger.Info("job finished")
			close(r.stopCh)
		}()

		for {
			select {
			case <-r.ctx.Done():
				err = r.ctx.Err()
				r.logger.Errorf("job timeout")
			default:
				isFinish, err = r.jobBatchRun()
			}

			if err != nil {
				if err == context.Canceled {
					r.logger.Errorf("job canceled")
					return
				}
				_ = r.pushEvent2FlowFSM(fsm.Event{Type: ExecuteErrorEvent, Status: r.job.Status, Message: err.Error(), Obj: r.job})
				return
			}
			if isFinish {
				if !IsFinishedStatus(r.job.Status) {
					_ = r.pushEvent2FlowFSM(fsm.Event{Type: ExecuteFinishEvent, Status: r.job.Status, Message: "finish", Obj: r.job})
				}
				return
			}
		}
	}()
	return nil
}

func (r *runner) handleJobPause(event fsm.Event) error {
	r.logger.Info("job pause")

	if err := r.recorder.SaveWorkflowJob(r.ctx, r.job); err != nil {
		r.logger.Errorf("save job status failed: %s", err)
		return err
	}
	return nil
}

func (r *runner) handleJobResume(event fsm.Event) error {
	r.logger.Info("job resume")

	if err := r.recorder.SaveWorkflowJob(r.ctx, r.job); err != nil {
		r.logger.Errorf("save job status failed: %s", err)
		return err
	}

	return nil
}

func (r *runner) handleJobSucceed(event fsm.Event) error {
	r.logger.Info("job succeed")

	if err := r.recorder.SaveWorkflowJob(r.ctx, r.job); err != nil {
		r.logger.Errorf("save job status failed: %s", err)
		return err
	}
	r.close("succeed")
	return nil
}

func (r *runner) handleJobFailed(event fsm.Event) error {
	r.logger.Info("job failed")

	if err := r.recorder.SaveWorkflowJob(r.ctx, r.job); err != nil {
		r.logger.Errorf("save job status failed: %s", err)
		return err
	}
	r.close(event.Message)
	return nil
}

func (r *runner) handleJobCancel(event fsm.Event) error {
	r.logger.Info("job cancel")
	if err := r.recorder.SaveWorkflowJob(r.ctx, r.job); err != nil {
		r.logger.Errorf("save job status failed: %s", err)
		return err
	}
	r.close(event.Message)
	return nil
}

func (r *runner) jobBatchRun() (finish bool, err error) {
	if !r.waitingForRunning(r.ctx) {
		return true, nil
	}

	var batch []*types.WorkflowJobStep
	if batch, err = r.nextBatch(); err != nil {
		r.logger.Errorf("make next batch plan error: %s, stop job.", err)
		return
	}

	if len(batch) == 0 {
		r.logger.Info("got empty batch, close finished job")
		return true, nil
	}

	batchCtx, batchCanF := context.WithCancel(r.ctx)
	defer batchCanF()
	wg := sync.WaitGroup{}
	for i := range batch {
		wg.Add(1)
		go func(t *types.WorkflowJobStep) {
			defer wg.Done()
			if needCancel := r.stepRun(batchCtx, t); needCancel {
				batchCanF()
			}
		}(batch[i])
	}
	wg.Wait()

	return false, nil
}

func (r *runner) stepRun(ctx context.Context, step *types.WorkflowJobStep) (needCancel bool) {
	var (
		currentTryTimes = 0
		err             error
	)
	r.logger.Infof("step %s started", step.StepName)
	if !r.waitingForRunning(ctx) {
		r.logger.Infof("job was finished, status=%s", r.job.Status)
		return
	}

	step.Status = RunningStatus
	if err = r.updateStepStatus(step); err != nil {
		r.logger.Errorf("update step status to running failed: %s", err)
		return
	}

	currentTryTimes += 1
	err = r.executor.DoOperation(ctx, *step)
	if err != nil {
		r.logger.Warnf("do step %s failed: %s", step.StepName, err)

		msg := fmt.Sprintf("step %s failed: %s", step.StepName, err)
		step.Status = FailedStatus
		step.Message = msg
		if err = r.updateStepStatus(step); err != nil {
			r.logger.Errorf("update step status to %s failed: %s", step.Status, err)
			return
		}

		needCancel = true
		err = r.pushEvent2FlowFSM(fsm.Event{Type: ExecuteFailedEvent, Status: r.job.Status, Message: msg, Obj: r.job})
		if err != nil {
			r.logger.Errorw("update job event failed", "err", err)
		}
		return
	}

	r.logger.Infof("step %s succeed", step.StepName)
	step.Status = SucceedStatus
	if err = r.updateStepStatus(step); err != nil {
		r.logger.Errorf("update step status to %s failed: %s", step.Status, err)
		return
	}
	return
}

func (r *runner) initial() (err error) {
	if r.job.Status == "" {
		r.job.Status = InitializingStatus
		if err = r.recorder.SaveWorkflowJob(r.ctx, r.job); err != nil {
			r.logger.Errorf("initializing job status failed: %s")
			return err
		}
	}
	for i := range r.job.Status {
		step := r.job.Steps[i]
		if step.Status == "" {
			step.Status = InitializingStatus
		}
	}
	if err = r.recorder.SaveWorkflowJob(r.ctx, r.job); err != nil {
		r.logger.Errorf("initializing step status failed: %s")
		return err
	}
	return
}

func (r *runner) nextBatch() ([]*types.WorkflowJobStep, error) {
	taskTowards := r.dag.nextBatchTasks()

	nextBatch := make([]*types.WorkflowJobStep, 0, len(taskTowards))
	for _, t := range taskTowards {
		for i := range r.job.Steps {
			step := r.job.Steps[i]
			if step.StepName == t.stepName {
				nextBatch = append(nextBatch, &step)
				break
			}
		}
	}
	return nextBatch, nil
}

func (r *runner) waitingForRunning(ctx context.Context) bool {
	for {
		select {
		case <-ctx.Done():
			return false
		default:
			switch r.job.Status {
			case SucceedStatus, FailedStatus, CanceledStatus, ErrorStatus:
				return false
			case InitializingStatus, RunningStatus:
				return true
			default:
				time.Sleep(time.Second * 15)
			}
		}
	}
}

func (r *runner) close(msg string) {
	r.logger.Debugf("stopping job: %s", msg)
	r.job.Message = msg

	if r.canF != nil {
		r.canF()
	}
}

func (r *runner) pushEvent2FlowFSM(event fsm.Event) error {
	err := r.fsm.Event(event)
	if err != nil {
		r.logger.Infow("push event to job FSM", "event", event.Type, "message", event.Message, "err", err)
		return err
	}
	return nil
}

func (r *runner) updateStepStatus(name, status string) error {

	found := false
	for i := range r.job.Steps {
		if r.job.Steps[i].StepName == name {
			r.job.Steps[i].Status = status
			found = true
		}
	}

	if !found {
		return nil
	}

	if err := r.recorder.SaveWorkflowJob(r.ctx, r.job); err != nil {
		return err
	}
	if IsFinishedStatus(status) {
		r.dag.updateTaskStatus(name, status)
	}
	return nil
}

func buildFlowFSM(r *runner) *fsm.FSM {
	m := fsm.New(fsm.Option{
		Obj:    r.job,
		Logger: r.logger.Named("fsm"),
	})

	m.From([]string{InitializingStatus, RunningStatus}).
		To(RunningStatus).
		When(TriggerEvent).
		Do(r.handleJobRun)

	m.From([]string{RunningStatus}).
		To(SucceedStatus).
		When(ExecuteFinishEvent).
		Do(r.handleJobSucceed)

	m.From([]string{InitializingStatus, RunningStatus}).
		To(ErrorStatus).
		When(ExecuteErrorEvent).
		Do(r.handleJobFailed)

	m.From([]string{InitializingStatus, RunningStatus}).
		To(FailedStatus).
		When(ExecuteFailedEvent).
		Do(r.handleJobFailed)

	m.From([]string{InitializingStatus, PausedStatus}).
		To(CanceledStatus).
		When(ExecuteCancelEvent).
		Do(r.handleJobCancel)

	m.From([]string{RunningStatus}).
		To(PausedStatus).
		When(ExecutePauseEvent).
		Do(r.handleJobPause)

	m.From([]string{PausedStatus}).
		To(RunningStatus).
		When(ExecuteResumeEvent).
		Do(r.handleJobResume)

	return m
}
