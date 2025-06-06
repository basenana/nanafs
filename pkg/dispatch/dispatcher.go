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
	"github.com/basenana/nanafs/pkg/core"
	"os"
	"strconv"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/notify"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
)

var taskExecutionInterval = 5 * time.Minute

func init() {
	intervalStr := os.Getenv("SCHED_TASK_EXEC_INTERVAL_SECONDS")
	if intervalStr != "" {
		intervalSec, err := strconv.Atoi(intervalStr)
		if err == nil {
			taskExecutionInterval = time.Duration(intervalSec) * time.Second
		}
	}
}

type executor interface {
	execute(ctx context.Context, task *types.ScheduledTask) error
}

type routineTask func(ctx context.Context) error

type Dispatcher struct {
	core      core.Core
	notify    *notify.Notify
	recorder  metastore.ScheduledTaskRecorder
	executors map[string]executor
	routines  [24][]routineTask
	metricCh  chan prometheus.Metric
	logger    *zap.SugaredLogger
}

func (d *Dispatcher) Run(stopCh chan struct{}) {
	ticker := time.NewTicker(taskExecutionInterval)
	d.logger.Infow("start scheduled task dispatcher", "interval", taskExecutionInterval.String())
	go d.runRoutineTask(stopCh)
	for {
		select {
		case <-stopCh:
			d.logger.Infow("stopped")
			return
		case <-ticker.C:
			d.logger.Debugw("find next runnable tasks")
		case <-idleWakeup:
			d.logger.Debugw("wakeup")
		}

		func() {
			ctx, canF := context.WithTimeout(context.Background(), time.Hour)
			defer canF()
			for taskID, exec := range d.executors {
				tasks, err := d.findRunnableTasks(ctx, taskID)
				if err != nil {
					d.logger.Errorw("find runnable task failed", "taskID", taskID, "err", err)
					taskExecutionErrorCounter.Inc()
					continue
				}

				for i := range tasks {
					err = d.dispatch(ctx, taskID, exec, tasks[i])
					if err != nil {
						d.logger.Errorw("execute task failed", "taskID", taskID, "err", err)
						continue
					}
				}
			}
			if err := d.recorder.DeleteFinishedTask(ctx, taskExecutionInterval*2); err != nil {
				d.logger.Errorw("delete finished task failed", "err", err)
			}
		}()
	}
}

func (d *Dispatcher) dispatch(ctx context.Context, taskID string, exec executor, task *types.ScheduledTask) error {
	defer logTaskExecutionLatency(taskID, time.Now())
	task.Status = types.ScheduledTaskExecuting
	task.ExecutionTime = time.Now()

	ctx = types.WithNamespace(ctx, types.NewNamespace(task.Namespace))
	if err := d.recorder.SaveTask(ctx, task); err != nil {
		taskExecutionErrorCounter.Inc()
		return err
	}

	if err := exec.execute(ctx, task); err != nil {
		task.Status = types.ScheduledTaskFailed
		if err == ErrNeedRetry {
			task.Status = types.ScheduledTaskWait
		}
		task.Result = fmt.Sprintf("refID: %d, msg: %s", task.Event.RefID, err)
		taskFinishStatusCounter.WithLabelValues(taskID, types.ScheduledTaskFailed)
		sentry.CaptureException(err)
		d.logger.Errorw("execute task error", "recordID", task.ID, "taskID", task.TaskID, "err", err,
			"recordNotificationErr", d.notify.RecordWarn(ctx, task.Namespace, fmt.Sprintf("Scheduled task %s failed", task.TaskID), task.Result, "Dispatcher"))
	} else {
		task.Result = "succeed"
		task.Status = types.ScheduledTaskSucceed
		taskFinishStatusCounter.WithLabelValues(taskID, types.ScheduledTaskSucceed)
		d.logger.Debugw("execute task finish", "recordID", task.ID, "taskID", task.TaskID)
	}

	if err := d.recorder.SaveTask(ctx, task); err != nil {
		taskExecutionErrorCounter.Inc()
		return err
	}
	return nil
}

func (d *Dispatcher) findRunnableTasks(ctx context.Context, taskID string) ([]*types.ScheduledTask, error) {
	tasks, err := d.recorder.ListTask(ctx, taskID,
		types.ScheduledTaskFilter{Status: []string{types.ScheduledTaskWait, types.ScheduledTaskExecuting}})
	if err != nil {
		return nil, err
	}

	var (
		runnable     []*types.ScheduledTask
		waitCount    = 0
		runningCount = 0
	)
	for i := range tasks {
		t := tasks[i]
		switch t.Status {
		case types.ScheduledTaskWait:
			if time.Now().After(t.ExecutionTime) {
				runnable = append(runnable, t)
			}
			waitCount += 1
		case types.ScheduledTaskExecuting:
			if time.Now().After(t.ExpirationTime) {
				t.Status = types.ScheduledTaskFailed
				t.Result = "timeout"
				_ = d.recorder.SaveTask(ctx, t)
			}
			runningCount += 1
		}
	}
	taskCurrentRunningGauge.WithLabelValues(taskID, types.ScheduledTaskWait).Set(float64(waitCount))
	taskCurrentRunningGauge.WithLabelValues(taskID, types.ScheduledTaskExecuting).Set(float64(runningCount))
	return runnable, nil
}

func (d *Dispatcher) runRoutineTask(stopCh chan struct{}) {
	var (
		index  = -1
		ticker = time.NewTicker(time.Hour)
	)
	d.logger.Infow("start routine task dispatcher")
	for {
		select {
		case <-stopCh:
			d.logger.Infow("routine task dispatcher stopped")
			return
		case <-ticker.C:
			index += 1
			index %= 24
		}

		tasks := d.routines[index]
		if len(tasks) == 0 {
			continue
		}

		ctx, canF := context.WithTimeout(context.Background(), time.Hour)
		for _, t := range tasks {
			if err := t(ctx); err != nil {
				d.logger.Errorw("routine task failed", "err", err)
			}
		}
		canF()
	}
}

func (d *Dispatcher) registerExecutor(taskID string, e executor) {
	d.executors[taskID] = e
}

func (d *Dispatcher) registerRoutineTask(periodH int, task routineTask) {
	periodH %= 24
	for i := 0; i < 24; i += periodH {
		d.routines[i] = append(d.routines[i], task)
	}
}

func Init(fsCore core.Core, notify *notify.Notify, recorder metastore.ScheduledTaskRecorder) (*Dispatcher, error) {
	d := &Dispatcher{
		core:      fsCore,
		notify:    notify,
		recorder:  recorder,
		executors: map[string]executor{},
		routines:  [24][]routineTask{},
		metricCh:  make(chan prometheus.Metric, 10),
		logger:    logger.NewLogger("dispatcher"),
	}

	if err := registerMaintainExecutor(d, fsCore, recorder); err != nil {
		return nil, err
	}

	if err := registerWorkflowExecutor(d, recorder); err != nil {
		return nil, err
	}

	return d, nil
}
