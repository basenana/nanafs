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
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"time"
)

const taskExecutionInterval = 5 * time.Minute

type Dispatcher struct {
	entry     dentry.Manager
	recorder  metastore.ScheduledTaskRecorder
	executors map[string]executor
	logger    *zap.SugaredLogger
}

func (d *Dispatcher) Run(stopCh chan struct{}) {
	ticker := time.NewTicker(taskExecutionInterval)
	for {
		select {
		case <-stopCh:
			d.logger.Infow("stopped")
			return
		case <-ticker.C:
			d.logger.Debugw("find next runnable tasks")
		}

		func() {
			ctx, canF := context.WithTimeout(context.Background(), time.Hour)
			defer canF()
			for taskID, exec := range d.executors {
				tasks, err := d.findRunnableTasks(ctx, taskID)
				if err != nil {
					d.logger.Errorw("find runnable task failed", "taskID", taskID, "err", err)
					continue
				}

				for i := range tasks {
					err = d.dispatch(ctx, exec, tasks[i])
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

func (d *Dispatcher) dispatch(ctx context.Context, exec executor, task *types.ScheduledTask) error {
	execFn := func() error {
		if err := exec.execute(ctx, task); err != nil {
			d.logger.Errorw("execute task error", "recordID", task.ID, "taskID", task.TaskID, "err", err)
			return err
		}
		d.logger.Debugw("execute task finish", "recordID", task.ID, "taskID", task.TaskID)
		return nil
	}

	task.Status = types.ScheduledTaskExecuting
	if err := d.recorder.SaveTask(ctx, task); err != nil {
		return err
	}

	task.Result = "succeed"
	if err := execFn(); err != nil {
		task.Result = fmt.Sprintf("error: %s", err)
	}
	task.Status = types.ScheduledTaskFinish
	if err := d.recorder.SaveTask(ctx, task); err != nil {
		return err
	}
	return nil
}

func (d *Dispatcher) handleEvent(evt *types.Event) {
	ctx, canF := context.WithCancel(context.Background())
	defer canF()

	var err error
	for tID, exec := range d.executors {
		err = exec.handleEvent(ctx, evt)
		if err != nil {
			d.logger.Errorw("handle event error", "taskID", tID, "err", err)
		}
	}
}

func (d *Dispatcher) findRunnableTasks(ctx context.Context, taskID string) ([]*types.ScheduledTask, error) {
	tasks, err := d.recorder.ListTask(ctx, taskID,
		types.ScheduledTaskFilter{Status: []string{types.ScheduledTaskWait, types.ScheduledTaskExecuting}})
	if err != nil {
		return nil, err
	}

	var runnable []*types.ScheduledTask
	for i := range tasks {
		t := tasks[i]
		switch t.Status {
		case types.ScheduledTaskWait:
			if time.Now().After(t.ExecutionTime) {
				runnable = append(runnable, t)
			}
		case types.ScheduledTaskExecuting:
			if time.Now().After(t.ExpirationTime) {
				t.Status = types.ScheduledTaskFinish
				t.Result = "timeout"
				_ = d.recorder.SaveTask(ctx, t)
			}
		}
	}
	return runnable, nil
}

func Init(entry dentry.Manager, recorder metastore.ScheduledTaskRecorder) (*Dispatcher, error) {
	d := &Dispatcher{
		entry:     entry,
		recorder:  recorder,
		executors: map[string]executor{},
		logger:    logger.NewLogger("dispatcher"),
	}
	registerMaintainExecutor(d.executors, entry, recorder)

	if _, err := events.Subscribe(events.TopicAllActions, d.handleEvent); err != nil {
		return nil, err
	}
	return d, nil
}
