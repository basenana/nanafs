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
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/document"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/notify"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/pkg/workflow/exec"
	"github.com/basenana/nanafs/pkg/workflow/jobrun"
	"github.com/basenana/nanafs/utils/logger"
)

type Manager interface {
	Start(stopCh chan struct{})

	ListWorkflows(ctx context.Context) ([]*types.WorkflowSpec, error)
	GetWorkflow(ctx context.Context, wfId string) (*types.WorkflowSpec, error)
	CreateWorkflow(ctx context.Context, spec *types.WorkflowSpec) (*types.WorkflowSpec, error)
	UpdateWorkflow(ctx context.Context, spec *types.WorkflowSpec) (*types.WorkflowSpec, error)
	DeleteWorkflow(ctx context.Context, wfId string) error
	ListJobs(ctx context.Context, wfId string) ([]*types.WorkflowJob, error)
	GetJob(ctx context.Context, wfId string, jobID string) (*types.WorkflowJob, error)

	TriggerWorkflow(ctx context.Context, wfId string, tgt types.WorkflowTarget, attr JobAttr) (*types.WorkflowJob, error)
	PauseWorkflowJob(ctx context.Context, jobId string) error
	ResumeWorkflowJob(ctx context.Context, jobId string) error
	CancelWorkflowJob(ctx context.Context, jobId string) error
}

type manager struct {
	ctrl     *jobrun.Controller
	entryMgr dentry.Manager
	docMgr   document.Manager
	notify   *notify.Notify
	cron     *CronHandler
	recorder metastore.ScheduledTaskRecorder
	config   config.Loader
	logger   *zap.SugaredLogger
}

var _ Manager = &manager{}

func NewManager(entryMgr dentry.Manager, docMgr document.Manager, notify *notify.Notify, recorder metastore.ScheduledTaskRecorder, cfg config.Loader) (Manager, error) {
	wfLogger = logger.NewLogger("workflow")

	workflowEnabled, err := cfg.GetSystemConfig(context.TODO(), config.WorkflowConfigGroup, "enable").Bool()
	if err != nil || !workflowEnabled {
		if err != nil && errors.Is(err, config.ErrNotConfigured) {
			wfLogger.Warnw("get workflow enable config failed, set disable", "err", err)
		}
		return disabledManager{}, nil
	}
	jobWorkdir, err := cfg.GetSystemConfig(context.TODO(), config.WorkflowConfigGroup, "jobWorkdir").String()
	if err != nil && !errors.Is(err, config.ErrNotConfigured) {
		return nil, fmt.Errorf("get workflow job workdir failed: %w", err)
	}
	if jobWorkdir == "" {
		jobWorkdir = genDefaultJobRootWorkdir()
		wfLogger.Warnw("using default job root workdir", "jobWorkdir", jobWorkdir)
	}

	if err = initWorkflowJobRootWorkdir(jobWorkdir); err != nil {
		return nil, fmt.Errorf("init workflow job root workdir error: %s", err)
	}

	if err := exec.RegisterOperators(entryMgr, docMgr, exec.Config{Enable: true, JobWorkdir: jobWorkdir}); err != nil {
		return nil, fmt.Errorf("register operators failed: %s", err)
	}

	flowCtrl := jobrun.NewJobController(recorder, notify)
	mgr := &manager{ctrl: flowCtrl, entryMgr: entryMgr, docMgr: docMgr, recorder: recorder, config: cfg, logger: wfLogger}
	root, err := entryMgr.Root(context.Background())
	if err != nil {
		mgr.logger.Errorw("query root failed", "err", err)
		return nil, err
	}

	mgr.logger.Infof("init workflow mirror dir to %s", MirrorRootDirName)
	plugin.Register(mirrorPlugin, buildWorkflowMirrorPlugin(mgr))
	if err := initWorkflowMirrorDir(root, entryMgr); err != nil {
		return nil, fmt.Errorf("init workflow mirror dir failed: %s", err)
	}
	mgr.cron = newCronHandler(mgr)

	return mgr, nil
}

func (m *manager) Start(stopCh chan struct{}) {
	bgCtx, canF := context.WithCancel(context.Background())
	m.cron.Start(bgCtx)
	m.ctrl.Start(bgCtx)

	if err := registerBuildInWorkflow(bgCtx, m); err != nil {
		m.logger.Errorw("register build-in workflow failed", "err", err)
	}

	// delay register
	time.Sleep(time.Minute)

	allWorkflows, err := m.ListWorkflows(bgCtx)
	if err != nil {
		m.logger.Errorw("init cron workflows failed: list workflows error", "err", err)
	}

	if len(allWorkflows) > 0 {
		for i, wf := range allWorkflows {
			if wf.Cron == "" {
				continue
			}
			err = m.cron.Register(allWorkflows[i])
			if err != nil {
				m.logger.Errorw("init cron workflows failed: registry workflow error", "workflow", wf.Id, "err", err)
			}
		}
	}

	<-stopCh
	canF()
}

func (m *manager) ListWorkflows(ctx context.Context) ([]*types.WorkflowSpec, error) {
	result, err := m.recorder.ListWorkflow(ctx)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (m *manager) GetWorkflow(ctx context.Context, wfId string) (*types.WorkflowSpec, error) {
	return m.recorder.GetWorkflow(ctx, wfId)
}

func (m *manager) CreateWorkflow(ctx context.Context, spec *types.WorkflowSpec) (*types.WorkflowSpec, error) {
	if spec.Name == "" {
		return nil, fmt.Errorf("workflow name is empty")
	}
	spec = initWorkflow(spec)
	err := validateWorkflowSpec(spec)
	if err != nil {
		return nil, err
	}

	if err = m.recorder.SaveWorkflow(ctx, spec); err != nil {
		return nil, err
	}

	if spec.Cron != "" {
		if err = m.cron.Register(spec); err != nil {
			return spec, fmt.Errorf("handle cron rules encounter failure: %s", err)
		}
	}
	return spec, nil
}

func (m *manager) UpdateWorkflow(ctx context.Context, spec *types.WorkflowSpec) (*types.WorkflowSpec, error) {
	err := validateWorkflowSpec(spec)
	if err != nil {
		return nil, err
	}
	spec.UpdatedAt = time.Now()
	if err = m.recorder.SaveWorkflow(ctx, spec); err != nil {
		return nil, err
	}

	if spec.Cron != "" {
		if err = m.cron.Register(spec); err != nil {
			return spec, fmt.Errorf("handle cron rules encounter failure: %s", err)
		}
	}
	return spec, nil
}

func (m *manager) DeleteWorkflow(ctx context.Context, wfId string) error {
	wfSpec, err := m.GetWorkflow(ctx, wfId)
	if err != nil {
		return err
	}

	jobs, err := m.ListJobs(ctx, wfId)
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

	if wfSpec.Cron != "" {
		m.cron.Unregister(wfId)
	}
	return m.recorder.DeleteWorkflow(ctx, wfId)
}

func (m *manager) GetJob(ctx context.Context, wfId string, jobID string) (*types.WorkflowJob, error) {
	result, err := m.recorder.GetWorkflowJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (m *manager) ListJobs(ctx context.Context, wfId string) ([]*types.WorkflowJob, error) {
	result, err := m.recorder.ListWorkflowJob(ctx, types.JobFilter{WorkFlowID: wfId})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (m *manager) TriggerWorkflow(ctx context.Context, wfId string, tgt types.WorkflowTarget, attr JobAttr) (*types.WorkflowJob, error) {
	workflow, err := m.GetWorkflow(ctx, wfId)
	if err != nil {
		return nil, err
	}

	m.logger.Infow("receive workflow", "workflow", workflow.Name, "entryID", tgt)
	if tgt.EntryID != 0 {
		var en *types.Metadata
		en, err = m.entryMgr.GetEntry(ctx, tgt.EntryID)
		if err != nil {
			m.logger.Errorw("query entry failed", "workflow", workflow.Name, "entryID", tgt, "err", err)
			return nil, err
		}
		tgt.ParentEntryID = en.ParentID
	}

	job, err := assembleWorkflowJob(workflow, tgt)
	if err != nil {
		m.logger.Errorw("assemble job failed", "workflow", workflow.Name, "err", err)
		return nil, err
	}

	if attr.JobID != "" {
		// TODO: improve this
		jobs, err := m.ListJobs(ctx, wfId)
		if err != nil {
			return nil, err
		}
		for _, j := range jobs {
			if j.Id == attr.JobID {
				return nil, fmt.Errorf("job id %s is already existes", attr.JobID)
			}
		}
		job.Id = attr.JobID
	}

	if attr.Timeout == 0 {
		attr.Timeout = defaultJobTimeout
	}
	job.TimeoutSeconds = int(attr.Timeout.Seconds())
	job.TriggerReason = attr.Reason

	err = m.recorder.SaveWorkflowJob(ctx, job)
	if err != nil {
		return nil, err
	}

	if err = m.ctrl.TriggerJob(ctx, job.Id); err != nil {
		m.logger.Errorw("trigger job flow failed", "job", job.Id, "err", err)
		return nil, err
	}
	return job, nil
}

func (m *manager) PauseWorkflowJob(ctx context.Context, jobId string) error {
	job, err := m.recorder.GetWorkflowJob(ctx, jobId)
	if err != nil {
		return err
	}
	if job.Status != jobrun.RunningStatus {
		return fmt.Errorf("pausing is not supported in non-running state")
	}
	return m.ctrl.PauseJob(jobId)
}

func (m *manager) ResumeWorkflowJob(ctx context.Context, jobId string) error {
	job, err := m.recorder.GetWorkflowJob(ctx, jobId)
	if err != nil {
		return err
	}
	if job.Status != jobrun.PausedStatus {
		return fmt.Errorf("resuming is not supported in non-paused state")
	}
	return m.ctrl.ResumeJob(jobId)
}

func (m *manager) CancelWorkflowJob(ctx context.Context, jobId string) error {
	job, err := m.recorder.GetWorkflowJob(ctx, jobId)
	if err != nil {
		return err
	}
	if !job.FinishAt.IsZero() {
		return fmt.Errorf("canceling is not supported in finished state")
	}
	return m.ctrl.CancelJob(jobId)
}

type disabledManager struct{}

func (d disabledManager) Start(stopCh chan struct{}) {}

func (d disabledManager) ListWorkflows(ctx context.Context) ([]*types.WorkflowSpec, error) {
	return make([]*types.WorkflowSpec, 0), nil
}

func (d disabledManager) GetWorkflow(ctx context.Context, wfId string) (*types.WorkflowSpec, error) {
	return nil, types.ErrNotFound
}

func (d disabledManager) CreateWorkflow(ctx context.Context, spec *types.WorkflowSpec) (*types.WorkflowSpec, error) {
	return nil, fmt.Errorf("workflow is disabled")
}

func (d disabledManager) UpdateWorkflow(ctx context.Context, spec *types.WorkflowSpec) (*types.WorkflowSpec, error) {
	return nil, types.ErrNotFound
}

func (d disabledManager) DeleteWorkflow(ctx context.Context, wfId string) error {
	return types.ErrNotFound
}

func (d disabledManager) GetJob(ctx context.Context, wfId string, jobID string) (*types.WorkflowJob, error) {
	return nil, types.ErrNotFound
}

func (d disabledManager) ListJobs(ctx context.Context, wfId string) ([]*types.WorkflowJob, error) {
	return nil, types.ErrNotFound
}

func (d disabledManager) TriggerWorkflow(ctx context.Context, wfId string, tgt types.WorkflowTarget, attr JobAttr) (*types.WorkflowJob, error) {
	return nil, types.ErrNotFound
}

func (d disabledManager) PauseWorkflowJob(ctx context.Context, jobId string) error {
	return types.ErrNotFound
}

func (d disabledManager) ResumeWorkflowJob(ctx context.Context, jobId string) error {
	return types.ErrNotFound
}

func (d disabledManager) CancelWorkflowJob(ctx context.Context, jobId string) error {
	return types.ErrNotFound
}

type JobAttr struct {
	JobID   string
	Reason  string
	Queue   string
	Timeout time.Duration
}
