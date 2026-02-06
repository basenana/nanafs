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
	"strings"
	"sync"
	"time"

	"github.com/basenana/go-flow"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/indexer"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/notify"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/basenana/plugin"
	"go.uber.org/zap"
)

type Controller struct {
	runners map[JobID]*runner
	workdir string

	pluginMgr plugin.Manager
	core      core.Core
	store     metastore.Meta
	indexer   indexer.Indexer
	notify    *notify.Notify

	scheduler *Scheduler
	mux       sync.Mutex
	logger    *zap.SugaredLogger
}

func (c *Controller) Start(ctx context.Context) {
	if c.scheduler != nil {
		return
	}

	c.scheduler = NewScheduler(c, defaultWorkers, defaultInterval, types.WorkflowQueueFile)
	c.scheduler.Run(ctx)
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
	if c.scheduler != nil {
		c.scheduler.Stop()
	}

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

	if flow.IsFinishedStatus(event.Flow.Status) {
		c.mux.Lock()
		delete(c.runners, jid)
		c.mux.Unlock()
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

func (c *Controller) addRunner(job *types.WorkflowJob, r *runner) {
	c.mux.Lock()
	c.runners[JobID{namespace: job.Namespace, id: job.Id}] = r
	c.mux.Unlock()
}

func NewJobController(pluginMgr plugin.Manager, fsCore core.Core,
	store metastore.Meta, indexer indexer.Indexer, notify *notify.Notify, workdir string) *Controller {
	ctrl := &Controller{
		pluginMgr: pluginMgr,
		core:      fsCore,
		store:     store,
		indexer:   indexer,
		notify:    notify,
		workdir:   workdir,
		runners:   make(map[JobID]*runner),
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
