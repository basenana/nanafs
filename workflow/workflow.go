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

package workflow

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/plugin"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/notify"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/basenana/nanafs/workflow/jobrun"
)

type Workflow interface {
	Start(ctx context.Context)

	ListWorkflows(ctx context.Context, namespace string) ([]*types.Workflow, error)
	GetWorkflow(ctx context.Context, namespace string, wfId string) (*types.Workflow, error)
	CreateWorkflow(ctx context.Context, namespace string, spec *types.Workflow) (*types.Workflow, error)
	UpdateWorkflow(ctx context.Context, namespace string, spec *types.Workflow) (*types.Workflow, error)
	DeleteWorkflow(ctx context.Context, namespace string, wfId string) error
	ListJobs(ctx context.Context, namespace string, wfId string) ([]*types.WorkflowJob, error)
	GetJob(ctx context.Context, namespace string, wfId string, jobID string) (*types.WorkflowJob, error)

	TriggerWorkflow(ctx context.Context, namespace string, wfId string, tgt types.WorkflowTarget, attr JobAttr) (*types.WorkflowJob, error)
	PauseWorkflowJob(ctx context.Context, namespace string, jobId string) error
	ResumeWorkflowJob(ctx context.Context, namespace string, jobId string) error
	CancelWorkflowJob(ctx context.Context, namespace string, jobId string) error
}

type manager struct {
	ctrl   *jobrun.Controller
	core   core.Core
	notify *notify.Notify
	meta   metastore.Meta
	config config.Workflow
	hooks  *hooks
	logger *zap.SugaredLogger
}

var _ Workflow = &manager{}

func New(fsCore core.Core, notify *notify.Notify, meta metastore.Meta, cfg config.Workflow) (Workflow, error) {
	wfLogger = logger.NewLogger("workflow")

	if cfg.JobWorkdir == "" {
		cfg.JobWorkdir = genDefaultJobRootWorkdir()
		wfLogger.Warnw("using default job root workdir", "jobWorkdir", cfg.JobWorkdir)
	}

	if err := initWorkflowJobRootWorkdir(cfg.JobWorkdir); err != nil {
		return nil, fmt.Errorf("init workflow job root workdir error: %s", err)
	}

	pluginMgr, err := plugin.Init(cfg)
	if err != nil {
		return nil, fmt.Errorf("init plugin failed %w", err)
	}
	flowCtrl := jobrun.NewJobController(pluginMgr, fsCore, meta, notify, cfg.JobWorkdir)
	mgr := &manager{ctrl: flowCtrl, core: fsCore, meta: meta, config: cfg, logger: wfLogger}
	mgr.hooks = initHooks(mgr)

	return mgr, nil
}
func (m *manager) Start(ctx context.Context) {
	m.ctrl.Start(ctx)
	m.hooks.start(ctx)
}

func (m *manager) ListWorkflows(ctx context.Context, namespace string) ([]*types.Workflow, error) {
	result, err := m.meta.ListWorkflows(ctx, namespace)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (m *manager) GetWorkflow(ctx context.Context, namespace string, wfId string) (*types.Workflow, error) {
	return m.meta.GetWorkflow(ctx, namespace, wfId)
}

func (m *manager) CreateWorkflow(ctx context.Context, namespace string, workflow *types.Workflow) (*types.Workflow, error) {
	if workflow.Name == "" {
		return nil, fmt.Errorf("workflow name is empty")
	}
	workflow = initWorkflow(namespace, workflow)
	err := validateWorkflowSpec(workflow)
	if err != nil {
		return nil, err
	}
	workflow.System = false
	if err = m.meta.SaveWorkflow(ctx, namespace, workflow); err != nil {
		return nil, err
	}

	m.hooks.handleWorkflowUpdate(workflow)
	return workflow, nil
}

func (m *manager) UpdateWorkflow(ctx context.Context, namespace string, workflow *types.Workflow) (*types.Workflow, error) {
	workflow.Namespace = namespace
	err := validateWorkflowSpec(workflow)
	if err != nil {
		return nil, err
	}
	workflow.UpdatedAt = time.Now()
	if err = m.meta.SaveWorkflow(ctx, namespace, workflow); err != nil {
		return nil, err
	}

	m.hooks.handleWorkflowUpdate(workflow)
	return workflow, nil
}

func (m *manager) DeleteWorkflow(ctx context.Context, namespace string, wfId string) error {
	jobs, err := m.ListJobs(ctx, namespace, wfId)
	if err != nil {
		return err
	}

	runningJobs := make([]string, 0)
	for _, j := range jobs {
		if j.Status == jobrun.PausedStatus || j.Status == jobrun.RunningStatus {
			runningJobs = append(runningJobs, j.Id)
		}
	}
	if len(runningJobs) > 0 {
		return fmt.Errorf("has running jobs: [%s]", strings.Join(runningJobs, ","))
	}

	err = m.meta.DeleteWorkflow(ctx, namespace, wfId)
	if err != nil {
		return err
	}
	m.hooks.handleWorkflowDelete(namespace, wfId)
	return nil
}

func (m *manager) GetJob(ctx context.Context, namespace string, wfId string, jobID string) (*types.WorkflowJob, error) {
	result, err := m.meta.GetWorkflowJob(ctx, namespace, jobID)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (m *manager) ListJobs(ctx context.Context, namespace string, wfId string) ([]*types.WorkflowJob, error) {
	result, err := m.meta.ListWorkflowJobs(ctx, namespace, types.JobFilter{WorkFlowID: wfId})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (m *manager) TriggerWorkflow(ctx context.Context, namespace string, wfId string, tgt types.WorkflowTarget, attr JobAttr) (*types.WorkflowJob, error) {
	workflow, err := m.GetWorkflow(ctx, namespace, wfId)
	if err != nil {
		return nil, err
	}

	m.logger.Infow("receive workflow", "workflow", workflow.Name, "entryID", tgt)
	job, err := assembleWorkflowJob(workflow, tgt)
	if err != nil {
		m.logger.Errorw("assemble job failed", "workflow", workflow.Name, "err", err)
		return nil, err
	}

	if attr.Timeout == 0 {
		attr.Timeout = defaultJobTimeout
	}
	job.TimeoutSeconds = int(attr.Timeout.Seconds())
	job.TriggerReason = attr.Reason

	err = m.meta.SaveWorkflowJob(ctx, namespace, job)
	if err != nil {
		return nil, err
	}

	if err = m.ctrl.TriggerJob(ctx, job.Namespace, job.Id); err != nil {
		m.logger.Errorw("trigger job flow failed", "job", job.Id, "err", err)
		return nil, err
	}
	return job, nil
}

func (m *manager) PauseWorkflowJob(ctx context.Context, namespace string, jobId string) error {
	job, err := m.meta.GetWorkflowJob(ctx, namespace, jobId)
	if err != nil {
		return err
	}
	if job.Status != jobrun.RunningStatus {
		return fmt.Errorf("pausing is not supported in non-running state")
	}
	return m.ctrl.PauseJob(namespace, jobId)
}

func (m *manager) ResumeWorkflowJob(ctx context.Context, namespace string, jobId string) error {
	job, err := m.meta.GetWorkflowJob(ctx, namespace, jobId)
	if err != nil {
		return err
	}
	if job.Status != jobrun.PausedStatus {
		return fmt.Errorf("resuming is not supported in non-paused state")
	}
	return m.ctrl.ResumeJob(namespace, jobId)
}

func (m *manager) CancelWorkflowJob(ctx context.Context, namespace string, jobId string) error {
	job, err := m.meta.GetWorkflowJob(ctx, namespace, jobId)
	if err != nil {
		return err
	}
	if !job.FinishAt.IsZero() {
		return fmt.Errorf("canceling is not supported in finished state")
	}
	return m.ctrl.CancelJob(namespace, jobId)
}

type JobAttr struct {
	Reason  string
	Queue   string
	Timeout time.Duration
}
