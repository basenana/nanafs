/*
 Copyright 2025 NanaFS Authors.

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
	"math/rand"
	"sync"
	"time"

	"github.com/basenana/go-flow"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
)

const (
	defaultWorkers  = 10
	defaultInterval = 5 * time.Second
)

type Scheduler struct {
	ctrl      *Controller
	workers   int
	interval  time.Duration
	queueName string
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

func NewScheduler(ctrl *Controller, workers int, interval time.Duration, queueName string) *Scheduler {
	if workers <= 0 {
		workers = defaultWorkers
	}
	if interval <= 0 {
		interval = defaultInterval
	}
	if queueName == "" {
		queueName = types.WorkflowQueueFile
	}
	return &Scheduler{
		ctrl:      ctrl,
		workers:   workers,
		interval:  interval,
		queueName: queueName,
		stopCh:    make(chan struct{}),
	}
}

func (s *Scheduler) Run(ctx context.Context) {
	for i := 0; i < s.workers; i++ {
		s.wg.Add(1)
		go s.worker(ctx, i)
	}
}

func (s *Scheduler) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}

func (s *Scheduler) worker(ctx context.Context, id int) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.ctrl.logger.Infof("scheduler worker %d stopped", id)
			return
		case <-s.stopCh:
			s.ctrl.logger.Infof("scheduler worker %d stopped", id)
			return
		case <-ticker.C:
			s.pollAndExecute(ctx)
		}
	}
}

func (s *Scheduler) pollAndExecute(ctx context.Context) {
	namespaces, err := s.ctrl.store.GetPendingNamespaces(ctx, s.queueName)
	if err != nil {
		s.ctrl.logger.Errorw("get pending namespaces failed", "err", err)
		return
	}
	if len(namespaces) == 0 {
		return
	}

	ns := namespaces[rand.Intn(len(namespaces))]

	job, err := s.ctrl.store.ClaimNextJob(ctx, s.queueName, ns)
	if err != nil {
		s.ctrl.logger.Errorw("claim next job failed", "namespace", ns, "err", err)
		return
	}
	if job == nil {
		return
	}

	s.executeJob(ctx, job)
}

func (s *Scheduler) executeJob(ctx context.Context, job *types.WorkflowJob) {
	f := workflowJob2Flow(s.ctrl, job)
	ctx = utils.NewWorkflowJobContext(ctx, job.Id)

	r := &runner{
		namespace: job.Namespace,
		workflow:  job.Workflow,
		job:       job.Id,
		runner:    nil,
	}

	if job.TimeoutSeconds == 0 {
		job.TimeoutSeconds = 60 * 60 * 3
	}

	s.ctrl.addRunner(job, r)

	jobCtx, canF := context.WithTimeout(ctx, time.Duration(job.TimeoutSeconds)*time.Second)
	defer canF()

	s.ctrl.logger.Infof("execute job %s.%s", job.Namespace, job.Id)
	r.runner = flow.NewRunner(f)
	err := r.runner.Start(jobCtx)
	if err != nil {
		s.ctrl.logger.Errorw("execute job failed", "job", job.Id, "err", err)
	}
}
