package dispatch

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"go.uber.org/zap"
	"time"
)

type Dispatcher struct {
	entry     dentry.Manager
	recorder  metastore.ScheduledTaskRecorder
	executors map[string]executor
	logger    zap.SugaredLogger
}

func (d *Dispatcher) Run(stopCh chan struct{}) {
	ticker := time.NewTicker(time.Minute * 15)
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
					err = exec.execute(ctx, tasks[i])
					if err != nil {
						d.logger.Errorw("execute task failed", "taskID", taskID, "err", err)
						continue
					}
				}
			}
		}()
	}
}

func (d *Dispatcher) dispatch(ctx context.Context, task *types.ScheduledTask) error {
	exec := d.executors[task.TaskID]
	if exec == nil {
		return fmt.Errorf("task id %s has no executor registered", task.TaskID)
	}
	if err := exec.execute(ctx, task); err != nil {
		d.logger.Infow("execute task error", "recordID", task.ID, "taskID", task.TaskID)
		return err
	}
	d.logger.Infow("execute task finish", "recordID", task.ID, "taskID", task.TaskID)
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
	tasks, err := d.recorder.ListTask(ctx, taskID, types.ScheduledTaskFilter{})
	if err != nil {
		return nil, err
	}

	var runnable []*types.ScheduledTask
	for i := range tasks {
		t := tasks[i]
		if t.Status != types.ScheduledTaskWait {
			continue
		}
		if time.Now().After(t.ExecutionTime) {
			runnable = append(runnable, t)
		}
	}
	return runnable, nil
}

func Init(
	entry dentry.Manager,
	recorder metastore.ScheduledTaskRecorder,
	chunkStore metastore.ChunkStore,
	dataStore storage.Storage) (*Dispatcher, error) {
	d := &Dispatcher{
		entry:    entry,
		recorder: recorder,
	}
	if _, err := events.Subscribe(events.TopicAllActions, d.handleEvent); err != nil {
		return nil, err
	}
	return d, nil
}
