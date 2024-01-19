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
	"math"
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
	queues   map[string]*queue
	rescan   chan int
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
		status := c.putJob2Queue(job)
		c.logger.Infow("append job to workqueue", "job", jID, "status", status)
	}
	return nil
}

func (c *Controller) Start(ctx context.Context) {
	for queueName := range c.queues {
		go c.jobWorkIterator(ctx, c.queues[queueName])
	}

	go func() {
		timer := time.NewTimer(time.Minute * 10)
		c.logger.Infof("start job controller, waiting for next job")
		for {
			select {
			case <-ctx.Done():
				c.logger.Infof("stop find next job")
				return
			case expect := <-c.rescan:
				err := c.findNextRunnableJob(ctx, expect)
				if err != nil {
					c.logger.Errorw("find next runnable job failed", "err", err)
				}
			case <-timer.C:
				select {
				case c.rescan <- defaultWorkQueueSize:
				default:
				}
			}
		}
	}()

	// requeue all running/pending jobs
	c.rescan <- math.MaxInt
}

func (c *Controller) jobWorkIterator(ctx context.Context, q *queue) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			q.HandleNext(c.handleNextJob)
		}
	}
}

func (c *Controller) handleNextJob(jobID string) {
	ctx := context.Background()
	job, err := c.recorder.GetWorkflowJob(ctx, jobID)
	if err != nil {
		c.logger.Errorw("handle next job encounter failed: get workflow job error", "job", jobID, "err", err)
		return
	}

	c.mux.Lock()
	jobCtx := utils.NewWorkflowJobContext(context.Background(), job.Id)
	r := NewRunner(job, runnerDep{recorder: c.recorder, notify: c.notify})
	c.runners[job.Id] = r
	c.mux.Unlock()

	if job.TimeoutSeconds == 0 {
		job.TimeoutSeconds = 60 * 60 * 3 // 3H
	}
	c.logger.Infof("trigger flow %s", job.Id)
	c.startJobRunner(jobCtx, job.Id, time.Duration(job.TimeoutSeconds)*time.Second)
	c.rescan <- 1
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

func (c *Controller) findNextRunnableJob(ctx context.Context, expect int) error {
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
		c.logger.Infow("requeue running job", "job", j.Id, "status", c.putJob2Queue(j))
	}

	pendingJobs, err := c.recorder.ListWorkflowJob(ctx, types.JobFilter{Status: PendingStatus})
	if err != nil {
		return err
	}
	sort.Slice(pendingJobs, func(i, j int) bool {
		return pendingJobs[i].CreatedAt.Before(pendingJobs[j].CreatedAt)
	})

	for _, j := range pendingJobs {
		if expect == 0 {
			return nil
		}
		_, ok := c.runners[j.Id]
		if ok {
			continue
		}
		_, ok = runningTarget[targetHash(j.Target)]
		if ok {
			continue
		}
		if c.putJob2Queue(j) == workQueueStatusAppended {
			expect -= 1
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

func (c *Controller) putJob2Queue(job *types.WorkflowJob) string {
	if job == nil {
		return ""
	}

	queueName := job.QueueName
	if _, ok := c.queues[queueName]; !ok {
		queueName = "default"
	}

	return c.queues[queueName].Put(job.Id)
}

func NewJobController(recorder metastore.ScheduledTaskRecorder, notify *notify.Notify) *Controller {
	ctrl := &Controller{
		runners:  make(map[string]Runner),
		recorder: recorder,
		notify:   notify,
		queues: map[string]*queue{
			"friday":  newQueue(defaultWorkQueueSize, 2),
			"default": newQueue(defaultWorkQueueSize*2, defaultWorkQueueSize),
		},
		rescan: make(chan int, 10),
		logger: logger.NewLogger("flow"),
	}
	return ctrl
}
