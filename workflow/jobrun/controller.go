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
	"github.com/basenana/nanafs/pkg/core"
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
	runners map[JobID]*runner
	queue   *GroupJobQueue
	workdir string

	pluginMgr plugin.Manager
	core      core.Core
	store     metastore.Meta
	notify    *notify.Notify

	isStartUp bool
	config    Config
	mux       sync.Mutex
	logger    *zap.SugaredLogger
}

func (c *Controller) TriggerJob(ctx context.Context, namespace, jID string) error {
	job, err := c.store.GetWorkflowJob(ctx, namespace, jID)
	if err != nil {
		return err
	}
	c.queue.Put(namespace, job.QueueName, job.Id)
	return nil
}

func (c *Controller) Start(ctx context.Context) {
	if c.isStartUp {
		return
	}

	go c.jobWorkQueueIterator(ctx, types.WorkflowQueueFile, 10)

	if err := c.rescanRunnableJob(ctx); err != nil {
		c.logger.Errorw("rescanle runable job failed", "err", err)
	}
	c.isStartUp = true

	go func() {
		timer := time.NewTicker(time.Minute * 10)
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

	var trigger = func(namespace, jid string) {
		parallelCh <- struct{}{}
		go func() {
			defer func() {
				<-parallelCh
				nextCh <- struct{}{}
			}()
			c.handleNextJob(namespace, jid)
		}()
	}

	for {
		select {
		case <-ctx.Done():
			return

		case <-nextCh:
			if jid := c.queue.Pop(queue); jid != nil {
				trigger(jid.namespace, jid.id)
			}
		}
	}
}

func (c *Controller) handleNextJob(namespace, jobID string) {
	ctx := context.Background()
	job, err := c.store.GetWorkflowJob(ctx, namespace, jobID)
	if err != nil {
		c.logger.Errorw("handle next job encounter failed: get workflow job error", "job", jobID, "err", err)
		return
	}

	f := workflowJob2Flow(c, job)
	c.logger.Infow("ns in ctx before start", "namespace", job.Namespace, "job", jobID)
	ctx = utils.NewWorkflowJobContext(ctx, job.Id)

	c.mux.Lock()
	jid := JobID{namespace: namespace, id: jobID}
	r := &runner{
		namespace: namespace,
		workflow:  job.Workflow,
		job:       jobID,
		runner:    flow.NewRunner(f),
	}
	c.runners[jid] = r
	c.mux.Unlock()
	defer func() {
		c.mux.Lock()
		delete(c.runners, jid)
		c.mux.Unlock()
	}()

	if job.TimeoutSeconds == 0 {
		job.TimeoutSeconds = 60 * 60 * 3 // 3H
	}

	jobCtx, canF := context.WithTimeout(ctx, time.Duration(job.TimeoutSeconds)*time.Second)
	defer canF()

	c.logger.Infof("trigger flow %s %s", namespace, job.Id)
	err = r.runner.Start(jobCtx)
	if err != nil {
		c.logger.Errorw("start runner failed: job failed", "job", jobID, "err", err)
		_ = c.notify.RecordWarn(jobCtx, job.Namespace, fmt.Sprintf("Workflow %s failed", job.Workflow),
			fmt.Sprintf("trigger job %s failed: %s", jobID, err), "JobController")
	}
}

func (c *Controller) rescanRunnableJob(ctx context.Context) error {
	if !c.isStartUp {
		// all running job
		runningJobs, err := c.store.ListAllNamespaceWorkflowJobs(ctx, types.JobFilter{Status: RunningStatus})
		if err != nil {
			return err
		}
		sort.Slice(runningJobs, func(i, j int) bool {
			return runningJobs[i].CreatedAt.Before(runningJobs[j].CreatedAt)
		})

		for _, j := range runningJobs {
			existR := c.getRunner(j.Workflow, j.Id)
			if existR != nil {
				continue
			}
			c.logger.Infow("requeue running job", "job", j.Id, "status", j.Status)
			c.queue.Put(j.Namespace, j.QueueName, j.Id)
		}
	}

	pendingJobs, err := c.store.ListAllNamespaceWorkflowJobs(ctx, types.JobFilter{Status: InitializingStatus})
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

func (c *Controller) PauseJob(namespace, jID string) error {
	r := c.getRunner(namespace, jID)
	if r == nil {
		return types.ErrNotFound
	}
	c.logger.Infof("pause flow %s", jID)
	return r.runner.Pause()
}

func (c *Controller) CancelJob(namespace, jID string) error {
	r := c.getRunner(namespace, jID)
	if r == nil {
		return types.ErrNotFound
	}
	return r.runner.Cancel()
}

func (c *Controller) ResumeJob(namespace, jID string) error {
	r := c.getRunner(namespace, jID)
	if r == nil {
		return types.ErrNotFound
	}
	return r.runner.Resume()
}

func (c *Controller) Shutdown() error {
	c.mux.Lock()
	defer c.mux.Unlock()

	failedFlows := make([]string, 0)
	for jid, r := range c.runners {
		if err := r.runner.Cancel(); err != nil {
			failedFlows = append(failedFlows, jid.id)
		}
	}
	if len(failedFlows) > 0 {
		return fmt.Errorf("cancel flows failed: %s", strings.Join(failedFlows, ","))
	}
	return nil
}

func (c *Controller) Handle(event flow.UpdateEvent) {
	ctx := context.Background()
	jid := NewJobID(event.Flow.ID)
	job, err := c.store.GetWorkflowJob(ctx, jid.namespace, jid.id)
	if err != nil {
		c.logger.Errorw("update workflow job status failed, failed to get job",
			"err", err, "namespace", jid.namespace, "job", jid.id)
		return
	}

	if job.StartAt.IsZero() {
		job.StartAt = time.Now()
	}

	if flow.IsFinishedStatus(event.Flow.Status) {
		job.FinishAt = time.Now()
	}

	job.Status = event.Flow.Status
	job.Message = event.Flow.Message

	if event.Task != nil {
		for i, step := range job.Nodes {
			if step.Name != event.Task.GetName() {
				continue
			}
			job.Nodes[i].Status = event.Task.GetStatus()
			job.Nodes[i].Message = event.Task.GetMessage()
		}
	}

	if err = c.store.SaveWorkflowJob(ctx, job.Namespace, job); err != nil {
		c.logger.Errorw("update workflow job status failed, failed to save job",
			"err", err, "job", event.Flow.ID)
		return
	}
	c.logger.Infow("update workflow job status",
		"job", event.Flow.ID, "status", event.Flow.Status, "message", event.Flow.Message)
}

func (c *Controller) getRunner(namespace, jobiD string) *runner {
	c.mux.Lock()
	r := c.runners[JobID{namespace: namespace, id: jobiD}]
	c.mux.Unlock()
	return r
}

func NewJobController(pluginMgr plugin.Manager, fsCore core.Core,
	store metastore.Meta, notify *notify.Notify, workdir string) *Controller {
	ctrl := &Controller{
		pluginMgr: pluginMgr,
		core:      fsCore,
		store:     store,
		notify:    notify,
		workdir:   workdir,
		runners:   make(map[JobID]*runner),
		queue:     newQueue(),
		logger:    logger.NewLogger("flow"),
	}
	return ctrl
}

type runner struct {
	namespace string
	workflow  string
	job       string
	runner    *flow.Runner
}
