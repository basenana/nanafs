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
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"strings"
	"sync"
)

type Controller struct {
	runners  map[string]Runner
	mux      sync.Mutex
	recorder metastore.ScheduledTaskRecorder
	notify   *notify.Notify
	logger   *zap.SugaredLogger
}

func (c *Controller) TriggerJob(ctx context.Context, jID string) error {
	job, err := c.recorder.GetWorkflowJob(ctx, jID)
	if err != nil {
		return err
	}
	r := NewRunner(job, c.recorder)
	c.mux.Lock()
	c.runners[jID] = r
	c.mux.Unlock()

	c.logger.Infof("trigger flow %s", jID)
	go c.startJobRunner(ctx, jID)
	return nil
}

func (c *Controller) startJobRunner(ctx context.Context, jID string) {
	c.mux.Lock()
	r, ok := c.runners[jID]
	c.mux.Unlock()
	if !ok {
		c.logger.Errorf("start runner failed, err: runner %s not found", jID)
		return
	}

	defer func() {
		c.mux.Lock()
		delete(c.runners, jID)
		c.mux.Unlock()
	}()

	err := r.Start(ctx)
	if err != nil {
		c.logger.Errorf("start runner failed, err: %s", err)
		_ = c.notify.RecordWarn(context.TODO(), "WorkflowJobFailed", fmt.Sprintf("start runner error: %s", err), "JobController")
	}

	job, err := c.recorder.GetWorkflowJob(ctx, jID)
	if err != nil {
		c.logger.Errorf("query workflow job %s failed, err: %s", jID, err)
		return
	}

	if IsFinishedStatus(job.Status) && job.Status != SucceedStatus || job.Status != CanceledStatus {
		_ = c.notify.RecordWarn(context.TODO(), "WorkflowJobFailed",
			fmt.Sprintf("job %s status: %s message: %s", jID, job.Status, job.Message), "JobController")
	}
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
		logger:   logger.NewLogger("flow"),
	}
}
