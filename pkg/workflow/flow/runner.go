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

package flow

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/workflow/fsm"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"sync"
	"time"
)

type Runner interface {
	Start(ctx context.Context) error
	Pause() error
	Resume() error
	Cancel() error
}

func NewRunner(f *Flow, s Storage) Runner {
	return &runner{Flow: f, storage: s, logger: logger.NewLogger("runner").With(zap.String("flow", f.ID))}
}

type runner struct {
	*Flow

	ctx      context.Context
	canF     context.CancelFunc
	fsm      *fsm.FSM
	dag      *DAG
	executor Executor
	stopCh   chan struct{}

	storage Storage
	logger  *zap.SugaredLogger
}

func (r *runner) Start(ctx context.Context) (err error) {
	if IsFinishedStatus(r.Status) {
		return
	}

	r.ctx, r.canF = context.WithCancel(ctx)
	if err = r.initial(); err != nil {
		r.logger.Errorf("flow initial failed: %s", err)
		return err
	}

	r.dag, err = buildDAG(r.Tasks)
	if err != nil {
		r.Status = FailedStatus
		r.logger.Errorf("build dag failed: %s, set flow status to failed: %s", err, r.storage.SaveFlow(r.ctx, r.Flow))
		return err
	}
	r.executor, err = newExecutor(r.Flow.Executor, r.Flow)
	if err != nil {
		r.Status = FailedStatus
		r.logger.Errorf("build executor failed: %s, set flow status to failed: %s", err, r.storage.SaveFlow(r.ctx, r.Flow))
		return err
	}
	r.fsm = buildFlowFSM(r)
	r.stopCh = make(chan struct{})

	if !r.waitingForRunning(ctx) {
		return
	}

	if err = r.executor.Setup(ctx); err != nil {
		r.Status = ErrorStatus
		r.logger.Errorf("setup executor failed: %s, set flow status to error: %s", err, r.storage.SaveFlow(r.ctx, r.Flow))
		return err
	}

	if err = r.pushEvent2FlowFSM(fsm.Event{Type: TriggerEvent, Status: r.GetStatus(), Obj: r}); err != nil {
		r.Status = ErrorStatus
		r.logger.Errorf("trigger flow failed: %s, set flow status to error: %s", err, r.storage.SaveFlow(r.ctx, r.Flow))
		return err
	}

	<-r.stopCh
	r.executor.Teardown(context.TODO())
	return nil
}

func (r *runner) Pause() error {
	if r.Status == RunningStatus {
		return r.pushEvent2FlowFSM(fsm.Event{Type: ExecutePauseEvent, Obj: r.Flow})
	}
	return nil
}

func (r *runner) Resume() error {
	if r.Status == PausedStatus {
		return r.pushEvent2FlowFSM(fsm.Event{Type: ExecuteResumeEvent, Obj: r.Flow})
	}
	return nil
}

func (r *runner) Cancel() error {
	if r.Status == SucceedStatus || r.Status == FailedStatus || r.Status == ErrorStatus {
		return nil
	}
	return r.pushEvent2FlowFSM(fsm.Event{Type: ExecuteCancelEvent, Obj: r.Flow})
}

func (r *runner) handleFlowRun(event fsm.Event) error {
	r.logger.Info("flow ready to run")
	if err := r.storage.SaveFlow(r.ctx, r.Flow); err != nil {
		r.logger.Errorf("save flow status failed: %s", err)
		return err
	}

	go func() {
		var (
			isFinish bool
			err      error
		)
		defer func() {
			r.logger.Info("flow finished")
			close(r.stopCh)
		}()

		for {
			select {
			case <-r.ctx.Done():
				err = r.ctx.Err()
				r.logger.Errorf("flow timeout")
			default:
				isFinish, err = r.flowBatchRun()
			}

			if err != nil {
				if err == context.Canceled {
					r.logger.Errorf("flow canceled")
					return
				}
				_ = r.pushEvent2FlowFSM(fsm.Event{Type: ExecuteErrorEvent, Status: r.GetStatus(), Message: err.Error(), Obj: r})
				return
			}
			if isFinish {
				if !IsFinishedStatus(r.Status) {
					_ = r.pushEvent2FlowFSM(fsm.Event{Type: ExecuteFinishEvent, Status: r.GetStatus(), Message: "finish", Obj: r})
				}
				return
			}
		}
	}()
	return nil
}

func (r *runner) handleFlowPause(event fsm.Event) error {
	r.logger.Info("flow pause")

	if err := r.storage.SaveFlow(r.ctx, r.Flow); err != nil {
		r.logger.Errorf("save flow status failed: %s", err)
		return err
	}
	return nil
}

func (r *runner) handleFlowResume(event fsm.Event) error {
	r.logger.Info("flow resume")

	if err := r.storage.SaveFlow(r.ctx, r.Flow); err != nil {
		r.logger.Errorf("save flow status failed: %s", err)
		return err
	}

	return nil
}

func (r *runner) handleFlowSucceed(event fsm.Event) error {
	r.logger.Info("flow succeed")

	if err := r.storage.SaveFlow(r.ctx, r.Flow); err != nil {
		r.logger.Errorf("save flow status failed: %s", err)
		return err
	}
	r.close("succeed")
	return nil
}

func (r *runner) handleFlowFailed(event fsm.Event) error {
	r.logger.Info("flow failed")

	if err := r.storage.SaveFlow(r.ctx, r.Flow); err != nil {
		r.logger.Errorf("save flow status failed: %s", err)
		return err
	}
	r.close(event.Message)
	return nil
}

func (r *runner) handleFlowCancel(event fsm.Event) error {
	r.logger.Info("flow cancel")
	if err := r.storage.SaveFlow(r.ctx, r.Flow); err != nil {
		r.logger.Errorf("save flow status failed: %s", err)
		return err
	}
	r.close(event.Message)
	return nil
}

func (r *runner) flowBatchRun() (finish bool, err error) {
	if !r.waitingForRunning(r.ctx) {
		return true, nil
	}

	var batch []*Task
	if batch, err = r.nextBatch(); err != nil {
		r.logger.Errorf("make next batch plan error: %s, stop flow.", err)
		return
	}

	if len(batch) == 0 {
		r.logger.Info("got empty batch, close finished flow")
		return true, nil
	}

	batchCtx, batchCanF := context.WithCancel(r.ctx)
	defer batchCanF()
	wg := sync.WaitGroup{}
	for i := range batch {
		wg.Add(1)
		go func(t *Task) {
			defer wg.Done()
			if needCancel := r.taskRun(batchCtx, t); needCancel {
				batchCanF()
			}
		}(batch[i])
	}
	wg.Wait()

	return false, nil
}

func (r *runner) taskRun(ctx context.Context, task *Task) (needCancel bool) {
	var (
		currentTryTimes = 0
		err             error
	)
	r.logger.Infof("task %s started", task.Name)
	if !r.waitingForRunning(ctx) {
		r.logger.Infof("flow was finished, status=%s", r.Status)
		return
	}

	task.Status = RunningStatus
	if err = r.saveTask(task); err != nil {
		r.logger.Errorf("update task status to running failed: %s", err)
		return
	}

	for {
		currentTryTimes += 1
		err = r.executor.DoOperation(ctx, *task, task.OperatorSpec)
		if err == nil {
			r.logger.Infof("task %s succeed", task.Name)
			break
		}
		r.logger.Warnf("do task %s failed: %s, retry=%v", task.Name, err, currentTryTimes < task.RetryOnFailed)
		if currentTryTimes >= task.RetryOnFailed {
			break
		}
	}

	if err != nil {
		msg := fmt.Sprintf("task %s failed: %s", task.Name, err)
		policy := r.ControlPolicy
		task.Status = FailedStatus
		task.Message = msg
		if err = r.saveTask(task); err != nil {
			r.logger.Errorf("update task status to %s failed: %s", task.Status, err)
			return
		}

		switch policy.FailedPolicy {
		case PolicyFastFailed:
			err = r.pushEvent2FlowFSM(fsm.Event{Type: ExecuteFailedEvent, Status: r.GetStatus(), Message: msg, Obj: r})
			needCancel = true
		case PolicyPaused:
			err = r.pushEvent2FlowFSM(fsm.Event{Type: ExecutePauseEvent, Status: r.GetStatus(), Message: msg, Obj: r})
		case PolicyContinue:
			// Do nothing
		}
		if err != nil {
			r.logger.Infof("update flow event failed: %s", err)
		}
		return
	}

	task.Status = SucceedStatus
	if err = r.saveTask(task); err != nil {
		r.logger.Errorf("update task status to %s failed: %s", task.Status, err)
		return
	}
	r.logger.Infof("run task %s finish", task.Name)
	return
}

func (r *runner) initial() (err error) {
	if r.Status == "" {
		r.Status = InitializingStatus
		if err = r.storage.SaveFlow(r.ctx, r.Flow); err != nil {
			r.logger.Errorf("initializing flow status failed: %s")
			return err
		}
	}
	for i := range r.Flow.Tasks {
		task := r.Flow.Tasks[i]
		if task.Status == "" {
			task.Status = InitializingStatus
			if err = r.saveTask(&task); err != nil {
				r.logger.Errorf("initializing task status failed: %s")
				return err
			}
		}
	}
	return
}

func (r *runner) nextBatch() ([]*Task, error) {
	taskTowards := r.dag.nextBatchTasks()

	nextBatch := make([]*Task, 0, len(taskTowards))
	for _, t := range taskTowards {
		for i := range r.Tasks {
			task := r.Tasks[i]
			if task.Name == t.taskName {
				nextBatch = append(nextBatch, &task)
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
			switch r.Status {
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
	r.logger.Debugf("stopping flow: %s", msg)
	r.SetMessage(msg)

	if r.canF != nil {
		r.canF()
	}
}

func (r *runner) pushEvent2FlowFSM(event fsm.Event) error {
	err := r.fsm.Event(event)
	if err != nil {
		r.logger.Infof("push event to flow FSM with event %s error: %s, msg: %s", event.Type, err, event.Message)
		return err
	}
	return nil
}

func (r *runner) saveTask(task *Task) error {
	for i, t := range r.Tasks {
		if t.Name == task.Name {
			r.Tasks[i] = *task
			break
		}
	}

	if err := r.storage.SaveFlow(r.ctx, r.Flow); err != nil {
		return err
	}
	if err := r.storage.SaveTask(r.ctx, r.ID, task); err != nil {
		return err
	}
	if IsFinishedStatus(task.Status) {
		r.dag.updateTaskStatus(task.Name, task.Status)
	}
	return nil
}

func buildFlowFSM(r *runner) *fsm.FSM {
	m := fsm.New(fsm.Option{
		Obj:    r.Flow,
		Logger: r.logger.Named("fsm"),
	})

	m.From([]string{InitializingStatus, RunningStatus}).
		To(RunningStatus).
		When(TriggerEvent).
		Do(r.handleFlowRun)

	m.From([]string{RunningStatus}).
		To(SucceedStatus).
		When(ExecuteFinishEvent).
		Do(r.handleFlowSucceed)

	m.From([]string{InitializingStatus, RunningStatus}).
		To(ErrorStatus).
		When(ExecuteErrorEvent).
		Do(r.handleFlowFailed)

	m.From([]string{InitializingStatus, RunningStatus}).
		To(FailedStatus).
		When(ExecuteFailedEvent).
		Do(r.handleFlowFailed)

	m.From([]string{InitializingStatus, PausedStatus}).
		To(CanceledStatus).
		When(ExecuteCancelEvent).
		Do(r.handleFlowCancel)

	m.From([]string{RunningStatus}).
		To(PausedStatus).
		When(ExecutePauseEvent).
		Do(r.handleFlowPause)

	m.From([]string{PausedStatus}).
		To(RunningStatus).
		When(ExecuteResumeEvent).
		Do(r.handleFlowResume)

	return m
}
