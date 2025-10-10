/*
   Copyright 2024 Go-Flow Authors

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
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

func NewRunner(f *Flow) *Runner {
	return &Runner{Flow: f}
}

type Runner struct {
	*Flow

	flowCtx    context.Context
	cancelFlow context.CancelFunc
	shotCtx    context.Context
	cancelShot context.CancelFunc

	intervenes sync.Map

	fsm     *FSM
	stopCh  chan struct{}
	started atomic.Bool
}

func (r *Runner) Start(ctx context.Context) (err error) {
	if IsFinishedStatus(r.Status) {
		return
	}

	r.flowCtx, r.cancelFlow = context.WithCancel(ctx)
	if err = r.executor.Setup(ctx); err != nil {
		r.setStatus(ErrorStatus, err.Error())
		return err
	}

	r.fsm = buildFlowFSM(r)
	r.stopCh = make(chan struct{})
	if !r.waitingForRunning(ctx) {
		return
	}

	if err = r.pushEvent2FlowFSM(statusEvent{Type: TriggerEvent}); err != nil {
		r.setStatus(ErrorStatus, err.Error())
		return err
	}

	// waiting all task down
	<-r.stopCh

	return r.executor.Teardown(ctx)
}

func (r *Runner) Pause() error {
	if r.Status == RunningStatus {
		return r.pushEvent2FlowFSM(statusEvent{Type: ExecutePauseEvent})
	}
	return fmt.Errorf("current status is %s", r.Status)
}

func (r *Runner) Resume() error {
	if r.Status == PausedStatus {
		return r.pushEvent2FlowFSM(statusEvent{Type: ExecuteResumeEvent})
	}
	return fmt.Errorf("current status is %s", r.Status)
}

func (r *Runner) Cancel() error {
	if IsFinishedStatus(r.Status) {
		return nil
	}
	return r.pushEvent2FlowFSM(statusEvent{Type: ExecuteCancelEvent})
}

func (r *Runner) handleJobRun(event statusEvent) error {
	if !r.started.CompareAndSwap(false, true) {
		return nil
	}
	r.setStatus(RunningStatus, event.Message)

	go func() {
		defer func() { r.started.Store(false) }()
		r.triggerFlow()
	}()
	return nil
}

func (r *Runner) triggerFlow() {
	var (
		isFinish bool
		err      error
	)

	shotCtx, cancelShot := context.WithCancel(r.flowCtx)
	defer cancelShot()

	r.shotCtx = shotCtx
	r.cancelShot = cancelShot
	for {
		select {
		case <-r.flowCtx.Done():
			err = r.flowCtx.Err()
		default:
		}

		switch r.Status {
		case RunningStatus:
		case PausingStatus:
			_ = r.pushEvent2FlowFSM(statusEvent{Type: ExecutePausedEvent, Message: ""})
			return
		default:
			return
		}

		isFinish, err = r.runNextBatchTask(r.shotCtx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			_ = r.pushEvent2FlowFSM(statusEvent{Type: ExecuteErrorEvent, Message: err.Error()})
			return
		}
		if isFinish {
			if !IsFinishedStatus(r.Status) {
				_ = r.pushEvent2FlowFSM(statusEvent{Type: ExecuteFinishEvent, Message: "finish"})
			}
			return
		}
	}

}

func (r *Runner) handleJobPause(event statusEvent) error {
	r.setStatus(PausingStatus, event.Message)
	if r.cancelShot != nil {
		r.cancelShot()
	}
	return nil
}

func (r *Runner) handleJobPaused(event statusEvent) error {
	r.setStatus(PausedStatus, event.Message)
	return nil
}

func (r *Runner) handleJobSucceed(event statusEvent) error {
	r.setStatus(SucceedStatus, event.Message)
	r.close()
	return nil
}

func (r *Runner) handleJobFailed(event statusEvent) error {
	r.setStatus(FailedStatus, event.Message)
	r.close()
	return nil
}

func (r *Runner) handleJobCancel(event statusEvent) error {
	r.setStatus(CanceledStatus, event.Message)
	r.close()
	return nil
}

func (r *Runner) runNextBatchTask(ctx context.Context) (finish bool, err error) {
	defer func() {
		if panicErr := doRecover(); panicErr != nil {
			err = panicErr
		}
	}()

	var (
		batch     []Task
		taskNames []string
		nextTasks = make(map[string]struct{})
	)
	taskNames, err = r.coordinator.NextBatch(ctx)
	if err != nil {
		return
	}

	for _, taskName := range taskNames {
		nextTasks[taskName] = struct{}{}
	}
	for _, t := range r.tasks {
		if IsFinishedStatus(t.GetStatus()) {
			continue
		}
		if _, ok := nextTasks[t.GetName()]; ok {
			batch = append(batch, t)
		}
	}

	if len(batch) == 0 {
		if r.coordinator.Finished() {
			return true, nil
		}
		_ = r.pushEvent2FlowFSM(statusEvent{Type: ExecutePauseEvent, Message: "some tasks have not been executed yet and cannot be scheduled"})
		return false, nil
	}

	wg := sync.WaitGroup{}
	for i := range batch {
		wg.Add(1)
		go func(t Task) {
			defer wg.Done()
			r.taskRun(ctx, t)
		}(batch[i])
	}
	wg.Wait()

	return false, nil
}

func (r *Runner) taskRun(ctx context.Context, task Task) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	r.setTaskStatue(task.GetName(), RunningStatus, "")
	err := r.executor.Exec(ctx, r.Flow, task)
	if err != nil {
		r.handleTaskFail(ctx, task, err)
		return
	}

	r.setTaskStatue(task.GetName(), SucceedStatus, "")
	return
}

func (r *Runner) handleTaskFail(ctx context.Context, task Task, err error) {
	msg := fmt.Sprintf("task %s failed: %s", task.GetName(), err)

	op := r.coordinator.HandleFail(task, err)
	switch op {
	case FailAndInterrupt:
		r.setTaskStatue(task.GetName(), FailedStatus, msg)
		_ = r.pushEvent2FlowFSM(statusEvent{Type: ExecuteFailedEvent, Message: msg})
	case FailAndPause:
		r.setTaskStatue(task.GetName(), PausedStatus, msg)
		_ = r.pushEvent2FlowFSM(statusEvent{Type: ExecutePauseEvent, Message: msg})
	case FailButContinue:
		r.setTaskStatue(task.GetName(), FailedStatus, msg)
	default:
		r.setTaskStatue(task.GetName(), FailedStatus, msg)
		_ = r.pushEvent2FlowFSM(statusEvent{Type: ExecuteFailedEvent, Message: msg})
	}
}

func (r *Runner) waitingForRunning(ctx context.Context) bool {
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

func (r *Runner) close() {
	if r.cancelFlow != nil {
		r.cancelFlow()
	}
	close(r.stopCh)
}

func (r *Runner) pushEvent2FlowFSM(event statusEvent) error {
	err := r.fsm.Event(event)
	if err != nil {
		return err
	}
	return nil
}

func buildFlowFSM(r *Runner) *FSM {
	m := NewFSM(r.Status)

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

	m.From([]string{InitializingStatus, RunningStatus, PausedStatus}).
		To(CanceledStatus).
		When(ExecuteCancelEvent).
		Do(r.handleJobCancel)

	m.From([]string{RunningStatus}).
		To(PausingStatus).
		When(ExecutePauseEvent).
		Do(r.handleJobPause)

	m.From([]string{PausingStatus}).
		To(PausedStatus).
		When(ExecutePausedEvent).
		Do(r.handleJobPaused)

	m.From([]string{PausedStatus}).
		To(RunningStatus).
		When(ExecuteResumeEvent).
		Do(r.handleJobRun)

	return m
}
