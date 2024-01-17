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
	"github.com/basenana/nanafs/pkg/notify"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultWorkQueueSize = 5
)

type Controller struct {
	runners  map[string]Runner
	mux      sync.Mutex
	recorder metastore.ScheduledTaskRecorder
	signal   chan string
	queue    *queue
	notify   *notify.Notify
	logger   *zap.SugaredLogger
}

func (c *Controller) TriggerJob(ctx context.Context, jID string) error {
	job, err := c.recorder.GetWorkflowJob(ctx, jID)
	if err != nil {
		return err
	}
	if job.Status == InitializingStatus {
		job.Status = PendingStatus
		err = c.recorder.SaveWorkflowJob(ctx, job)
		if err != nil {
			c.logger.Errorw("set job status to pending failed", "err", err)
			return err
		}
	}

	if job.Status == PendingStatus || job.Status == RunningStatus {
		if c.queue.Put(jID) {
			c.logger.Warnw("trigger job blocked", "job", jID)
		}
	}
	return nil
}

func (c *Controller) Start(ctx context.Context) {
	go c.jobWorkIterator(ctx)

	go func() {
		timer := time.NewTimer(time.Minute * 10)
		c.logger.Infof("start job controller, waiting for next job")
		for {
			select {
			case <-ctx.Done():
				c.logger.Infof("stop find next job")
				return
			case <-timer.C:
				err := c.findNextRunnableJob(ctx)
				if err != nil {
					c.logger.Errorw("find next runnable job failed", "err", err)
				}
			}
		}
	}()
}

func (c *Controller) jobWorkIterator(ctx context.Context) {
	var (
		job           *types.WorkflowJob
		err           error
		queueAccounts = map[string]chan struct{}{
			"friday":  make(chan struct{}, 1),
			"default": make(chan struct{}, defaultWorkQueueSize),
		}
	)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			jobID := c.queue.Next()
			job, err = c.recorder.GetWorkflowJob(ctx, jobID)
			if err != nil {
				c.logger.Errorw("get workflow job error", "job", jobID, "err", err)
				continue
			}
		}
		queueName := job.QueueName
		if _, ok := queueAccounts[queueName]; !ok {
			queueName = "default"
		}

		queueAccounts[queueName] <- struct{}{}
		c.mux.Lock()
		jobCtx := utils.NewWorkflowJobContext(context.Background(), job.Id)
		r := NewRunner(job, runnerDep{recorder: c.recorder, notify: c.notify})
		c.runners[job.Id] = r
		c.mux.Unlock()

		if job.TimeoutSeconds == 0 {
			job.TimeoutSeconds = 60 * 60 * 3 // 3H
		}
		go func(jobCtx context.Context, job *types.WorkflowJob, queueName string) {
			c.logger.Infof("trigger flow %s", job.Id)
			c.startJobRunner(jobCtx, job.Id, time.Duration(job.TimeoutSeconds)*time.Second)
			<-queueAccounts[queueName]
		}(jobCtx, job, queueName)
	}
}

func (c *Controller) startJobRunner(ctx context.Context, jID string, timeout time.Duration) {
	c.mux.Lock()
	r, ok := c.runners[jID]
	c.mux.Unlock()
	if !ok {
		c.logger.Errorw("start runner failed, err: runner not found", "job", jID)
		return
	}

	defer func() {
		c.mux.Lock()
		delete(c.runners, jID)
		c.mux.Unlock()
	}()

	ctx, canF := context.WithTimeout(ctx, timeout)
	defer canF()

	job, err := c.recorder.GetWorkflowJob(ctx, jID)
	if err != nil {
		c.logger.Errorw("start runner failed: query jobs error", "job", jID, "err", err)
		return
	}

	wf, err := c.recorder.GetWorkflow(ctx, job.Workflow)
	if err != nil {
		c.logger.Errorw("start runner failed: query workflow error", "job", jID, "workflow", job.Workflow, "err", err)
		return
	}

	err = r.Start(ctx)
	if err != nil {
		c.logger.Errorw("start runner failed: job failed", "job", jID, "err", err)
		_ = c.notify.RecordWarn(context.TODO(), fmt.Sprintf("Workflow %s failed", wf.Name),
			fmt.Sprintf("trigger job %s failed: %s", jID, err), "JobController")
	}
}

func (c *Controller) findNextRunnableJob(ctx context.Context) error {
	c.mux.Lock()
	defer c.mux.Unlock()

	runningJobs, err := c.recorder.ListWorkflowJob(ctx, types.JobFilter{Status: RunningStatus})
	if err != nil {
		return err
	}
	sort.Slice(runningJobs, func(i, j int) bool {
		return runningJobs[i].CreatedAt.Before(runningJobs[j].CreatedAt)
	})

	runningTarget := map[string]struct{}{}
	for _, j := range runningJobs {
		runningTarget[targetHash(j.Target)] = struct{}{}
		_, ok := c.runners[j.Id]
		if ok {
			continue
		}
		if c.queue.Put(j.Id) {
			return nil
		}
	}

	pendingJobs, err := c.recorder.ListWorkflowJob(ctx, types.JobFilter{Status: PendingStatus})
	if err != nil {
		return err
	}
	sort.Slice(pendingJobs, func(i, j int) bool {
		return pendingJobs[i].CreatedAt.Before(pendingJobs[j].CreatedAt)
	})

	for _, j := range pendingJobs {
		_, ok := c.runners[j.Id]
		if ok {
			continue
		}
		_, ok = runningTarget[targetHash(j.Target)]
		if ok {
			continue
		}
		if c.queue.Put(j.Id) {
			return nil
		}
	}

	return nil
}

func (c *Controller) PauseJob(jID string) error {
	r, ok := c.runners[jID]
	if !ok {
		return fmt.Errorf("flow %s not found", jID)
	}
	c.logger.Infof("pause flow %s", jID)
	return r.Pause()
}

func (c *Controller) CancelJob(jID string) error {
	r, ok := c.runners[jID]
	if !ok {
		return fmt.Errorf("flow %s not found", jID)
	}
	return r.Cancel()
}

func (c *Controller) ResumeJob(jID string) error {
	r, ok := c.runners[jID]
	if !ok {
		return fmt.Errorf("flow %s not found", jID)
	}
	return r.Resume()
}

func (c *Controller) Shutdown() error {
	c.mux.Lock()
	defer c.mux.Unlock()

	failedFlows := make([]string, 0)
	for fId, r := range c.runners {
		if err := r.Cancel(); err != nil {
			failedFlows = append(failedFlows, fId)
		}
	}
	if len(failedFlows) > 0 {
		return fmt.Errorf("cancel flows failed: %s", strings.Join(failedFlows, ","))
	}
	return nil
}

func NewJobController(recorder metastore.ScheduledTaskRecorder, notify *notify.Notify) *Controller {
	return &Controller{
		runners:  make(map[string]Runner),
		recorder: recorder,
		notify:   notify,
		queue:    newQueue(),
		logger:   logger.NewLogger("flow"),
	}
}
