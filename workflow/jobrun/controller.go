/*
 Copyright 2024 NanaFS Authors.

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
	"github.com/basenana/go-flow"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/document"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/notify"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"sort"
	"strings"
	"sync"
	"time"
)

type Config struct {
	Enable     bool
	JobWorkdir string
}

type Controller struct {
	runners map[string]*flow.Runner
	queue   *GroupJobQueue
	workdir string

	pluginMgr *plugin.Manager
	entryMgr  dentry.Manager
	docMgr    document.Manager
	recorder  metastore.ScheduledTaskRecorder
	notify    *notify.Notify

	isStartUp bool
	config    Config
	mux       sync.Mutex
	logger    *zap.SugaredLogger
}

func (c *Controller) TriggerJob(ctx context.Context, jID string) error {
	job, err := c.recorder.GetWorkflowJob(ctx, jID)
	if err != nil {
		return err
	}
	c.queue.Put(job.Namespace, job.QueueName, job.Id)
	return nil
}

func (c *Controller) Start(ctx context.Context) {
	if c.isStartUp {
		return
	}

	go c.jobWorkQueueIterator(ctx, types.WorkflowQueueFile, 10)
	go c.jobWorkQueueIterator(ctx, types.WorkflowQueuePipe, 50)

	if err := c.rescanRunnableJob(ctx); err != nil {
		c.logger.Errorw("rescanle runable job failed", "err", err)
	}
	c.isStartUp = true

	go func() {
		timer := time.NewTimer(time.Minute * 10)
		c.logger.Infof("start job controller, waiting for next job")
		for {
			select {
			case <-ctx.Done():
				c.logger.Infof("stop find next job")
				return
			case <-timer.C:
				err := c.rescanRunnableJob(ctx)
				if err != nil {
					c.logger.Errorw("find next runnable job failed", "err", err)
				}
			}
		}
	}()
}

func (c *Controller) jobWorkQueueIterator(ctx context.Context, queue string, parallel int) {
	var (
		nextCh     = c.queue.Signal(queue)
		parallelCh = make(chan struct{}, parallel)
	)

	var trigger = func(jid string) {
		parallelCh <- struct{}{}
		go func() {
			defer func() {
				<-parallelCh
				nextCh <- struct{}{}
			}()
			c.handleNextJob(jid)
		}()
	}

	for {
		select {
		case <-ctx.Done():
			return

		case <-nextCh:
			if jid := c.queue.Pop(queue); jid != "" {
				trigger(jid)
			}
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

	f := workflowJob2Flow(c, job)
	ctx = types.WithNamespace(ctx, types.NewNamespace(job.Namespace))
	c.logger.Infow("ns in ctx before start", "ns", types.GetNamespace(ctx).String())
	ctx = utils.NewWorkflowJobContext(ctx, job.Id)

	c.mux.Lock()

	r := flow.NewRunner(f)
	c.runners[jobID] = r
	c.mux.Unlock()
	defer func() {
		c.mux.Lock()
		delete(c.runners, jobID)
		c.mux.Unlock()
	}()

	if job.TimeoutSeconds == 0 {
		job.TimeoutSeconds = 60 * 60 * 3 // 3H
	}
	jobCtx, canF := context.WithTimeout(ctx, time.Duration(job.TimeoutSeconds)*time.Second)
	defer canF()

	c.logger.Infof("trigger flow %s", job.Id)
	err = r.Start(jobCtx)
	if err != nil {
		c.logger.Errorw("start runner failed: job failed", "job", jobID, "err", err)
		_ = c.notify.RecordWarn(jobCtx, fmt.Sprintf("Workflow %s failed", job.Workflow),
			fmt.Sprintf("trigger job %s failed: %s", jobID, err), "JobController")
	}
}

func (c *Controller) rescanRunnableJob(ctx context.Context) error {
	c.mux.Lock()
	defer c.mux.Unlock()

	if !c.isStartUp {
		// all running job
		runningJobs, err := c.recorder.ListWorkflowJob(ctx, types.JobFilter{Status: RunningStatus})
		if err != nil {
			return err
		}
		sort.Slice(runningJobs, func(i, j int) bool {
			return runningJobs[i].CreatedAt.Before(runningJobs[j].CreatedAt)
		})

		for _, j := range runningJobs {
			_, ok := c.runners[j.Id]
			if ok {
				continue
			}
			c.logger.Infow("requeue running job", "job", j.Id, "status", j.Status)
			c.queue.Put(j.Namespace, j.QueueName, j.Id)
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
		c.queue.Put(j.Namespace, j.QueueName, j.Id)
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

func (c *Controller) Handle(event flow.UpdateEvent) {
	ctx := context.Background()
	job, err := c.recorder.GetWorkflowJob(ctx, event.Flow.ID)
	if err != nil {
		c.logger.Errorw("update workflow job status failed, failed to get job",
			"err", err, "job", event.Flow.ID)
		return
	}

	job.Status = event.Flow.Status
	job.Message = event.Flow.Message

	if event.Task != nil {
		for i, step := range job.Steps {
			if step.StepName != event.Task.GetName() {
				continue
			}
			job.Steps[i].Status = event.Task.GetName()
			job.Steps[i].Message = event.Task.GetMessage()
		}
	}

	if err = c.recorder.SaveWorkflowJob(ctx, job); err != nil {
		c.logger.Errorw("update workflow job status failed, failed to save job",
			"err", err, "job", event.Flow.ID)
		return
	}
	c.logger.Infow("update workflow job status finish",
		"job", event.Flow.ID, "status", event.Flow.Status)
}

func NewJobController(pluginMgr *plugin.Manager, entryMgr dentry.Manager, docMgr document.Manager,
	recorder metastore.ScheduledTaskRecorder, notify *notify.Notify, workdir string) *Controller {
	ctrl := &Controller{
		pluginMgr: pluginMgr,
		entryMgr:  entryMgr,
		docMgr:    docMgr,
		recorder:  recorder,
		notify:    notify,
		workdir:   workdir,
		runners:   make(map[string]*flow.Runner),
		queue:     newQueue(),
		logger:    logger.NewLogger("flow"),
	}
	return ctrl
}
