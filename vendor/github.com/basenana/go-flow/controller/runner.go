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

package controller

import (
	"context"
	"fmt"
	"github.com/basenana/go-flow/flow"
	"github.com/basenana/go-flow/fsm"
	"github.com/basenana/go-flow/log"
	"github.com/basenana/go-flow/storage"
	"reflect"
	"sync"
	"time"
)

type runner struct {
	flow.Flow

	ctx    *flow.Context
	stopCh chan struct{}
	fsm    *fsm.FSM

	batch     []runningTask
	batchCtx  *flow.Context
	batchCanF context.CancelFunc

	storage storage.Interface
	logger  log.Logger
}

func (r *runner) Start(ctx *flow.Context) error {
	r.ctx = ctx
	r.SetStatus(flow.InitializingStatus)
	if err := r.storage.SaveFlow(r.Flow); err != nil {
		r.logger.Errorf("save flow status failed: %s", err.Error())
		return err
	}

	r.fsm = buildFlowFSM(r)

	setupCtx := &flow.Context{
		Context: ctx.Context,
		Logger:  ctx.Logger.With("setup"),
		FlowId:  r.ID(),
	}
	if err := r.Setup(setupCtx); err != nil || !setupCtx.IsSucceed {
		if err == nil {
			err = fmt.Errorf("flow setup failed: %s", setupCtx.Message)
		}
		setupCtx.Logger.Errorf("flow setup failed: %s", err.Error())
		_ = r.pushEvent2FlowFSM(fsm.Event{Type: flow.ExecuteErrorEvent, Status: r.GetStatus(), Message: "flow setup failed", Obj: r})
		return err
	}

	r.logger.Info("flow ready to run")
	if err := r.pushEvent2FlowFSM(fsm.Event{Type: flow.TriggerEvent, Status: r.GetStatus(), Obj: r}); err != nil {
		return err
	}

	if err := r.storage.SaveFlow(r.Flow); err != nil {
		r.logger.Errorf("save flow status failed: %s", err.Error())
	}
	return r.flowRun()
}

func (r *runner) Pause(event fsm.Event) error {
	event.Type = flow.ExecutePauseEvent
	event.Obj = r
	return r.pushEvent2FlowFSM(event)
}

func (r *runner) Resume(event fsm.Event) error {
	event.Type = flow.ExecuteResumeEvent
	event.Obj = r
	return r.pushEvent2FlowFSM(event)
}

func (r *runner) Cancel(event fsm.Event) error {
	event.Type = flow.ExecuteCancelEvent
	event.Obj = r
	return r.pushEvent2FlowFSM(event)
}

func (r *runner) flowRun() error {
	go func() {
		r.logger.Info("flow start")

		var err error
		for {
			select {
			case <-r.ctx.Done():
				r.logger.Errorf("flow timeout")
				_ = r.pushEvent2FlowFSM(fsm.Event{Type: flow.ExecuteErrorEvent, Status: r.GetStatus(), Message: "timeout", Obj: r})
				return
			case <-r.stopCh:
				r.logger.Warn("flow closed")
				return
			default:
				r.logger.Info("run next batch")
			}

		StatusCheckLoop:
			for {
				switch r.GetStatus() {
				case flow.RunningStatus:
					break StatusCheckLoop
				case flow.FailedStatus:
					r.logger.Warn("flow closed")
					return
				}
				time.Sleep(15 * time.Second)
			}

			r.logger.Info("current not paused")
			needRetry := false
			for _, task := range r.batch {
				if task.GetStatus() == flow.SucceedStatus {
					continue
				}
				needRetry = true
			}

			if !needRetry {
				if err = r.makeNextBatch(); err != nil {
					r.logger.Errorf("make next batch plan error: %s, stop flow.", err.Error())
					_ = r.pushEvent2FlowFSM(fsm.Event{Type: flow.ExecuteErrorEvent, Status: r.GetStatus(), Message: err.Error(), Obj: r})
					return
				}
				if len(r.batch) == 0 {
					r.logger.Info("got empty batch, close finished flow")
					_ = r.pushEvent2FlowFSM(fsm.Event{Type: flow.ExecuteFinishEvent, Status: r.GetStatus(), Message: "finish", Obj: r})
					break
				}
			} else {
				r.logger.Warn("retry current batch")
			}

			batchCtx, canF := context.WithCancel(r.ctx.Context)
			r.batchCtx = &flow.Context{
				Context:  batchCtx,
				Logger:   r.logger,
				FlowId:   r.ID(),
				MaxRetry: r.ctx.MaxRetry,
			}
			r.batchCanF = canF

			if err = r.runBatchTasks(); err != nil {
				r.logger.Warnf("run batch failed: %s", err.Error())
				_ = r.pushEvent2FlowFSM(fsm.Event{Type: flow.ExecuteErrorEvent, Status: r.GetStatus(), Message: err.Error(), Obj: r})
				return
			}
		}
		r.logger.Info("flow finish")
	}()

	return nil
}

func (r *runner) makeNextBatch() error {
	var (
		nextTasks []flow.Task
		meta      *storage.FlowMeta
		err       error
	)

	meta, err = r.storage.GetFlowMeta(r.Flow.ID())
	if err != nil {
		return fmt.Errorf("query flow meta error: %s", err.Error())
	}
	for len(nextTasks) == 0 {
		nextTasks, err = r.NextBatch(r.ctx)
		if err != nil {
			r.logger.Errorf("got next batch error: %s", err.Error())
			return err
		}

		if len(nextTasks) == 0 {
			// all task finish
			return nil
		}

		newBatch := make([]runningTask, 0, len(nextTasks))
		for i, task := range nextTasks {

			oldTaskStatus, queryTaskErr := meta.QueryTaskStatus(task.Name())
			if queryTaskErr != nil {
				if queryTaskErr == storage.NotFound {
					// new task
					newBatch = append(newBatch, runningTask{
						Task: nextTasks[i],
						FSM:  buildFlowTaskFSM(r, nextTasks[i]),
					})
					continue
				}
				return fmt.Errorf("query task status %s error: %s", task.Name(), err.Error())
			}

			r.logger.Infof("loaded old task record, status is %s(isFinish=%v)", oldTaskStatus, IsFinishedStatus(oldTaskStatus))
			if IsFinishedStatus(oldTaskStatus) {
				continue
			}

			newBatch = append(newBatch, runningTask{
				Task: nextTasks[i],
				FSM:  buildFlowTaskFSM(r, nextTasks[i]),
			})
		}
		r.batch = newBatch
	}
	r.logger.Infof("got new batch, contain %d tasks", len(r.batch))
	return nil
}

func (r *runner) handleFlowRun(event fsm.Event) error {
	hooks := r.GetHooks()
	if hook, ok := hooks[flow.WhenTrigger]; ok {
		if err := hook(r.ctx, r.Flow, nil); err != nil {
			r.logger.Errorf("run flow trigger hook error: %s", err.Error())
			return err
		}
	}
	return nil
}

func (r *runner) handleFlowPause(event fsm.Event) error {
	r.logger.Info("flow pause")

	pauseTasksErr := NewErrors()
	for _, t := range r.batch {
		if t.GetStatus() == flow.RunningStatus {
			if err := t.Event(fsm.Event{Type: flow.TaskExecutePauseEvent, Status: t.GetStatus(), Obj: t.Task}); err != nil {
				pauseTasksErr = append(pauseTasksErr, err)
			}
		}
	}
	if pauseTasksErr.IsError() {
		return pauseTasksErr
	}

	hooks := r.GetHooks()
	if hook, ok := hooks[flow.WhenExecutePause]; ok {
		if err := hook(r.ctx, r.Flow, nil); err != nil {
			r.logger.Errorf("run flow pause hook error: %s", err.Error())
			return err
		}
	}

	if err := r.storage.SaveFlow(r.Flow); err != nil {
		r.logger.Errorf("save flow status failed: %s", err.Error())
		return err
	}
	return nil
}

func (r *runner) handleFlowResume(event fsm.Event) error {
	r.logger.Info("flow resume")

	resumeTasksErr := NewErrors()
	for _, t := range r.batch {
		if t.GetStatus() == flow.PausedStatus {
			if err := t.Event(fsm.Event{Type: flow.TaskExecuteResumeEvent, Status: t.GetStatus(), Obj: t.Task}); err != nil {
				resumeTasksErr = append(resumeTasksErr, err)
			}
		}
	}
	if resumeTasksErr.IsError() {
		return resumeTasksErr
	}

	hooks := r.GetHooks()
	if hook, ok := hooks[flow.WhenExecuteResume]; ok {
		if err := hook(r.ctx, r.Flow, nil); err != nil {
			r.logger.Errorf("run flow resume hook error: %s", err.Error())
			return err
		}
	}

	if err := r.storage.SaveFlow(r.Flow); err != nil {
		r.logger.Errorf("save flow status failed: %s", err.Error())
		return err
	}

	return nil
}

func (r *runner) handleFlowSucceed(event fsm.Event) error {
	r.logger.Info("flow succeed")
	r.stop("succeed")

	hooks := r.GetHooks()
	if hook, ok := hooks[flow.WhenExecuteSucceed]; ok {
		if err := hook(r.ctx, r.Flow, nil); err != nil {
			r.logger.Errorf("run flow succeed hook error: %s", err.Error())
			return err
		}
	}

	if err := r.storage.SaveFlow(r.Flow); err != nil {
		r.logger.Errorf("save flow status failed: %s", err.Error())
		return err
	}
	return nil
}

func (r *runner) handleFlowFailed(event fsm.Event) error {
	r.logger.Info("flow failed")
	r.stop(event.Message)

	hooks := r.GetHooks()
	if hook, ok := hooks[flow.WhenExecuteFailed]; ok {
		if err := hook(r.ctx, r.Flow, nil); err != nil {
			r.logger.Errorf("run flow failed hook error: %s", err.Error())
			return err
		}
	}

	if err := r.storage.SaveFlow(r.Flow); err != nil {
		r.logger.Errorf("save flow status failed: %s", err.Error())
		return err
	}
	return nil
}

func (r *runner) handleFlowCancel(event fsm.Event) error {
	r.logger.Info("flow cancel")
	if r.batchCanF != nil {
		r.batchCanF()
	}

	cancelTasksErr := NewErrors()
	for _, t := range r.batch {
		switch t.GetStatus() {
		case flow.InitializingStatus, flow.RunningStatus, flow.PausedStatus:
			r.logger.Infof("cancel %s task %s", t.GetStatus(), t.Name())
			if err := t.Event(fsm.Event{Type: flow.TaskExecuteCancelEvent, Status: t.GetStatus(), Obj: t.Task}); err != nil {
				cancelTasksErr = append(cancelTasksErr, err)
			}
		}
	}
	if cancelTasksErr.IsError() {
		return cancelTasksErr
	}

	r.stop(event.Message)

	hooks := r.GetHooks()
	if hook, ok := hooks[flow.WhenExecuteCancel]; ok {
		if err := hook(r.ctx, r.Flow, nil); err != nil {
			r.logger.Errorf("run flow cancel hook error: %s", err.Error())
			return err
		}
	}

	if err := r.storage.SaveFlow(r.Flow); err != nil {
		r.logger.Errorf("save flow status failed: %s", err.Error())
		return err
	}

	return nil
}

func (r *runner) runBatchTasks() error {
	var (
		wg         = sync.WaitGroup{}
		taskErrors = NewErrors()
	)

	triggerTask := func(task runningTask) error {
		r.logger.Infof("trigger task %s", task.Name())
		if IsFinishedStatus(task.GetStatus()) {
			r.logger.Warnf("task %s was finished(%s)", task.Name(), task.GetStatus())
			return nil
		}

		select {
		case <-r.ctx.Done():
			r.logger.Infof("flow was closed, skip task %s trigger", task.Name())
			return nil
		default:
			if err := task.Event(fsm.Event{Type: flow.TaskTriggerEvent, Status: task.GetStatus(), Obj: task.Task}); err != nil {
				switch r.GetStatus() {
				case flow.CanceledStatus, flow.FailedStatus:
					return nil
				default:
					return err
				}
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				if runTaskErr := r.taskRun(task); runTaskErr != nil {
					taskErrors = append(taskErrors, runTaskErr)
				}
			}()
			return nil
		}
	}

	for i, task := range r.batch {
		// batch retry skip already succeed task
		if task.GetStatus() == flow.SucceedStatus {
			continue
		}

		task.SetStatus(flow.InitializingStatus)
		setupCtx := &flow.Context{
			Context: r.ctx.Context,
			Logger:  r.logger.With(fmt.Sprintf("task.%s.setup", task.Name())),
			FlowId:  r.ID(),
		}
		if err := task.Setup(setupCtx); err != nil || !setupCtx.IsSucceed {
			var msg string
			if err != nil {
				msg = fmt.Sprintf("task setup failed: %s", err.Error())
			} else {
				msg = setupCtx.Message
			}

			if updateTaskStatsErr := task.Event(fsm.Event{Type: flow.TaskExecuteErrorEvent, Status: task.GetStatus(), Obj: task.Task}); updateTaskStatsErr != nil {
				setupCtx.Logger.Errorf("update task status failed: %s", updateTaskStatsErr.Error())
			}

			setupCtx.Logger.Warnf("context not got succeed: %s", msg)
			return fmt.Errorf("task setup failed: %s", msg)
		}

		if err := triggerTask(r.batch[i]); err != nil {
			return err
		}
	}

	// wait all task finish
	r.logger.Infof("start waiting all task finish")
	wg.Wait()

	if len(taskErrors) == 0 {
		r.batch = nil
		return nil
	}

	return fmt.Errorf("%s, and %d others", taskErrors[0], len(taskErrors))
}

func (r *runner) taskRun(task runningTask) error {
	var (
		currentTryTimes  = 0
		defaultRetryTime = 3
		err              error
	)
	ctx := &flow.Context{
		Context:  r.batchCtx.Context,
		Logger:   r.logger.With("task"),
		FlowId:   r.batchCtx.FlowId,
		MaxRetry: defaultRetryTime,
		TaskName: task.Name(),
	}
	r.logger.Infof("task %s started", task.Name())
	for {
		currentTryTimes += 1
		err = task.Task.Do(ctx)
		if err == nil {
			r.logger.Infof("task %s succeed", task.Name())
			break
		}
		r.logger.Warnf("do task %s failed: %s", task.Name(), err.Error())
		if currentTryTimes >= ctx.MaxRetry {
			r.logger.Infof("task %s can not retry", task.Name())
			break
		}
	}

	if !ctx.IsSucceed {
		msg := fmt.Sprintf("task %s failed: %s", task.Name(), ctx.Message)
		policy := r.GetControlPolicy()

		updateStatsErrors := NewErrors()
		switch policy.FailedPolicy {
		case flow.PolicyFastFailed:
			err = r.pushEvent2FlowFSM(fsm.Event{Type: flow.ExecuteErrorEvent, Status: r.GetStatus(), Message: msg, Obj: r})
		case flow.PolicyPaused:
			err = r.pushEvent2FlowFSM(fsm.Event{Type: flow.ExecutePauseEvent, Status: r.GetStatus(), Message: msg, Obj: r})
		default:
			err = r.pushEvent2FlowFSM(fsm.Event{Type: flow.ExecutePauseEvent, Status: r.GetStatus(), Message: msg, Obj: r})
		}

		if err != nil {
			updateStatsErrors = append(updateStatsErrors, err)
		}
		if err = task.Event(fsm.Event{Type: flow.TaskExecuteErrorEvent, Status: task.GetStatus(), Message: ctx.Message, Obj: task.Task}); err != nil {
			updateStatsErrors = append(updateStatsErrors, err)
		}

		if updateStatsErrors.IsError() {
			return updateStatsErrors
		}
		return nil
	}

	r.logger.Infof("run task %s finish", task.Name())
	return task.Event(fsm.Event{Type: flow.TaskExecuteFinishEvent, Status: task.GetStatus(), Obj: task.Task})
}

func (r *runner) handleTaskRun(event fsm.Event) error {
	task, ok := event.Obj.(flow.Task)
	if !ok || reflect.ValueOf(task).Kind() != reflect.Ptr {
		return fmt.Errorf("task %s obj not ptr", task.Name())
	}

	hooks := r.GetHooks()
	if hook, ok := hooks[flow.WhenTaskTrigger]; ok {
		if err := hook(r.ctx, r.Flow, task); err != nil {
			r.logger.Errorf("run task trigger hook error: %s", err.Error())
			return err
		}
	}

	if err := r.storage.SaveTask(r.ID(), task); err != nil {
		r.logger.Errorf("save task status failed: %s", err.Error())
		return err
	}
	return nil
}

func (r *runner) handleTaskSucceed(event fsm.Event) error {
	task := event.Obj.(flow.Task)
	r.logger.Infof("task %s succeed", task.Name())
	task.Teardown(r.batchCtx)

	hooks := r.GetHooks()
	if hook, ok := hooks[flow.WhenTaskExecuteSucceed]; ok {
		if err := hook(r.ctx, r.Flow, task); err != nil {
			r.logger.Errorf("run task succeed hook error: %s", err.Error())
			return err
		}
	}

	if err := r.storage.SaveTask(r.ID(), task); err != nil {
		r.logger.Errorf("save task status failed: %s", err.Error())
		return err
	}
	return nil
}

func (r *runner) handleTaskFailed(event fsm.Event) (err error) {
	task := event.Obj.(flow.Task)
	r.logger.Infof("task %s failed: %s", task.Name(), event.Message)
	task.Teardown(r.batchCtx)

	hooks := r.GetHooks()
	if hook, ok := hooks[flow.WhenTaskExecuteFailed]; ok {
		if err = hook(r.ctx, r.Flow, task); err != nil {
			r.logger.Errorf("run task failed hook error: %s", err.Error())
			return err
		}
	}

	if err = r.storage.SaveTask(r.ID(), task); err != nil {
		r.logger.Errorf("save task status failed: %s", err.Error())
		return err
	}
	return nil
}

func (r *runner) handleTaskCanceled(event fsm.Event) error {
	task := event.Obj.(flow.Task)
	task.Teardown(r.batchCtx)
	r.logger.Infof("task %s canceled", task.Name())

	hooks := r.GetHooks()
	if hook, ok := hooks[flow.WhenTaskExecuteCancel]; ok {
		if err := hook(r.ctx, r.Flow, task); err != nil {
			r.logger.Errorf("run task cancel hook error: %s", err.Error())
			return err
		}
	}

	if err := r.storage.SaveTask(r.ID(), task); err != nil {
		r.logger.Errorf("save task status failed: %s", err.Error())
		return err
	}
	return nil
}

func (r *runner) handleTaskPaused(event fsm.Event) error {
	task := event.Obj.(flow.Task)
	r.logger.Infof("task %s paused", task.Name())

	hooks := r.GetHooks()
	if hook, ok := hooks[flow.WhenTaskExecutePause]; ok {
		if err := hook(r.ctx, r.Flow, task); err != nil {
			r.logger.Errorf("run task cancel hook error: %s", err.Error())
			return err
		}
	}

	if err := r.storage.SaveTask(r.ID(), task); err != nil {
		r.logger.Errorf("save task status failed: %s", err.Error())
		return err
	}
	return nil
}

func (r *runner) handleTaskResume(event fsm.Event) error {
	task := event.Obj.(flow.Task)
	r.logger.Infof("task %s resume", task.Name())

	hooks := r.GetHooks()
	if hook, ok := hooks[flow.WhenTaskExecuteResume]; ok {
		if err := hook(r.ctx, r.Flow, task); err != nil {
			r.logger.Errorf("run task resume hook error: %s", err.Error())
			return err
		}
	}

	if err := r.storage.SaveTask(r.ID(), task); err != nil {
		r.logger.Errorf("save task status failed: %s", err.Error())
		return err
	}

	return nil
}
func (r *runner) stop(msg string) {
	r.logger.Debugf("stopping flow: %s", msg)
	r.SetMessage(msg)
	close(r.stopCh)

	func() {
		defer func() {
			if err := recover(); err != nil {
				r.logger.Errorf("teardown panic: %v", err)
			}
		}()
		r.Teardown(&flow.Context{
			Context: r.ctx.Context,
			Logger:  r.logger.With("teardown"),
			FlowId:  r.ID(),
		})
	}()
}

func (r *runner) pushEvent2FlowFSM(event fsm.Event) error {
	err := r.fsm.Event(event)
	if err != nil {
		r.logger.Errorf("push event to flow FSM with event %s error: %s, msg: %s", event.Type, err.Error(), event.Message)
		return err
	}
	return nil
}

type runningTask struct {
	flow.Task
	*fsm.FSM
}

func buildFlowFSM(r *runner) *fsm.FSM {
	m := fsm.New(fsm.Option{
		Obj:    r.Flow,
		Logger: r.logger.With("fsm"),
	})

	m.From([]fsm.Status{flow.InitializingStatus}).
		To(flow.RunningStatus).
		When(flow.TriggerEvent).
		Do(r.handleFlowRun)

	m.From([]fsm.Status{flow.RunningStatus}).
		To(flow.SucceedStatus).
		When(flow.ExecuteFinishEvent).
		Do(r.handleFlowSucceed)

	m.From([]fsm.Status{flow.InitializingStatus, flow.RunningStatus}).
		To(flow.FailedStatus).
		When(flow.ExecuteErrorEvent).
		Do(r.handleFlowFailed)

	m.From([]fsm.Status{flow.InitializingStatus, flow.PausedStatus}).
		To(flow.CanceledStatus).
		When(flow.ExecuteCancelEvent).
		Do(r.handleFlowCancel)

	m.From([]fsm.Status{flow.RunningStatus}).
		To(flow.PausedStatus).
		When(flow.ExecutePauseEvent).
		Do(r.handleFlowPause)

	m.From([]fsm.Status{flow.PausedStatus}).
		To(flow.RunningStatus).
		When(flow.ExecuteResumeEvent).
		Do(r.handleFlowResume)

	return m
}

func buildFlowTaskFSM(r *runner, t flow.Task) *fsm.FSM {
	m := fsm.New(fsm.Option{
		Obj:    t,
		Logger: r.logger.With(fmt.Sprintf("task.%s.fsm", t.Name())),
	})

	m.From([]fsm.Status{flow.InitializingStatus}).
		To(flow.RunningStatus).
		When(flow.TaskTriggerEvent).
		Do(r.handleTaskRun)

	m.From([]fsm.Status{flow.RunningStatus}).
		To(flow.SucceedStatus).
		When(flow.TaskExecuteFinishEvent).
		Do(r.handleTaskSucceed)

	m.From([]fsm.Status{flow.RunningStatus}).
		To(flow.PausedStatus).
		When(flow.TaskExecutePauseEvent).
		Do(r.handleTaskPaused)

	m.From([]fsm.Status{flow.PausedStatus}).
		To(flow.RunningStatus).
		When(flow.TaskExecuteResumeEvent).
		Do(r.handleTaskResume)

	m.From([]fsm.Status{flow.InitializingStatus, flow.RunningStatus, flow.PausedStatus}).
		To(flow.CanceledStatus).
		When(flow.TaskExecuteCancelEvent).
		Do(r.handleTaskCanceled)

	m.From([]fsm.Status{flow.InitializingStatus, flow.RunningStatus}).
		To(flow.FailedStatus).
		When(flow.TaskExecuteErrorEvent).
		Do(r.handleTaskFailed)

	return m
}
