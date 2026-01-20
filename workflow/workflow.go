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
	"strings"
	"time"

	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/indexer"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/plugin"
	pluginlogger "github.com/basenana/plugin/logger"
	plugintypes "github.com/basenana/plugin/types"

	"go.uber.org/zap"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/notify"
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
	ListJobs(ctx context.Context, namespace string, wfId string, filter types.JobFilter) ([]*types.WorkflowJob, error)
	GetJob(ctx context.Context, namespace string, wfId string, jobID string) (*types.WorkflowJob, error)

	TriggerWorkflow(ctx context.Context, namespace string, wfId string, tgt types.WorkflowTarget, attr JobAttr) (*types.WorkflowJob, error)
	PauseWorkflowJob(ctx context.Context, namespace string, jobId string) error
	ResumeWorkflowJob(ctx context.Context, namespace string, jobId string) error
	CancelWorkflowJob(ctx context.Context, namespace string, jobId string) error

	ListPlugins(ctx context.Context) []plugintypes.PluginSpec
}

type manager struct {
	ctrl    *jobrun.Controller
	core    core.Core
	notify  *notify.Notify
	meta    metastore.Meta
	plugin  plugin.Manager
	config  config.Workflow
	trigger *triggers
	logger  *zap.SugaredLogger
}

var _ Workflow = &manager{}

func New(fsCore core.Core, notify *notify.Notify, meta metastore.Meta, indexer indexer.Indexer, cfg config.Workflow) (Workflow, error) {
	wfLogger = logger.NewLogger("workflow")
	pluginlogger.SetLogger(wfLogger.Named("plugin"))

	if err := initWorkflowJobRootWorkdir(cfg.JobWorkdir); err != nil {
		return nil, fmt.Errorf("init workflow job root workdir error: %s", err)
	}

	pluginMgr := plugin.New()
	flowCtrl := jobrun.NewJobController(pluginMgr, fsCore, meta, indexer, notify, cfg.JobWorkdir)
	mgr := &manager{ctrl: flowCtrl, core: fsCore, meta: meta, plugin: pluginMgr, config: cfg, logger: wfLogger}
	mgr.trigger = initTriggers(mgr, fsCore, meta)

	return mgr, nil
}

func (m *manager) Start(ctx context.Context) {
	m.ctrl.Start(ctx)
	m.trigger.start(ctx)
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
	workflow = initWorkflow(namespace, workflow)
	err := validateWorkflowSpec(workflow)
	if err != nil {
		return nil, err
	}
	if err = m.meta.SaveWorkflow(ctx, namespace, workflow); err != nil {
		return nil, err
	}

	m.trigger.handleWorkflowUpdate(workflow, false)
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

	m.trigger.handleWorkflowUpdate(workflow, false)
	return workflow, nil
}

func (m *manager) DeleteWorkflow(ctx context.Context, namespace string, wfId string) error {
	wf, err := m.meta.GetWorkflow(ctx, namespace, wfId)
	if err != nil {
		return err
	}
	jobs, err := m.ListJobs(ctx, namespace, wfId, types.JobFilter{})
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
	m.trigger.handleWorkflowUpdate(wf, true)
	return nil
}

func (m *manager) GetJob(ctx context.Context, namespace string, wfId string, jobID string) (*types.WorkflowJob, error) {
	result, err := m.meta.GetWorkflowJob(ctx, namespace, jobID)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (m *manager) ListJobs(ctx context.Context, namespace string, wfId string, filter types.JobFilter) ([]*types.WorkflowJob, error) {
	filter.WorkFlowID = wfId
	result, err := m.meta.ListWorkflowJobs(ctx, namespace, filter)
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

	if attr.Timeout == 0 {
		attr.Timeout = defaultJobTimeout
	}
	if attr.Queue == "" {
		attr.Queue = workflow.QueueName
	}

	if len(tgt.Entries) == 0 {
		return nil, fmt.Errorf("no entries found for job")
	}

	m.logger.Infow("receive workflow", "workflow", workflow.Name, "targets", tgt)
	job, err := assembleWorkflowJob(workflow, tgt, attr)
	if err != nil {
		m.logger.Errorw("assemble job failed", "workflow", workflow.Name, "err", err)
		return nil, err
	}

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

func (m *manager) ListPlugins(ctx context.Context) []plugintypes.PluginSpec {
	return m.plugin.ListPlugins()
}

type JobAttr struct {
	Reason     string
	Queue      string
	Parameters map[string]string
	Timeout    time.Duration
}
