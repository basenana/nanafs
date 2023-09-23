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
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/notify"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/pkg/workflow/exec"
	"github.com/basenana/nanafs/pkg/workflow/jobrun"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"strings"
	"time"
)

type Manager interface {
	ListWorkflows(ctx context.Context) ([]*types.WorkflowSpec, error)
	GetWorkflow(ctx context.Context, wfId string) (*types.WorkflowSpec, error)
	CreateWorkflow(ctx context.Context, spec *types.WorkflowSpec) (*types.WorkflowSpec, error)
	UpdateWorkflow(ctx context.Context, spec *types.WorkflowSpec) (*types.WorkflowSpec, error)
	DeleteWorkflow(ctx context.Context, wfId string) error
	ListJobs(ctx context.Context, wfId string) ([]*types.WorkflowJob, error)

	TriggerWorkflow(ctx context.Context, wfId string, entryID int64, attr JobAttr) (*types.WorkflowJob, error)
	PauseWorkflowJob(ctx context.Context, jobId string) error
	ResumeWorkflowJob(ctx context.Context, jobId string) error
	CancelWorkflowJob(ctx context.Context, jobId string) error
}

func init() {
}

type manager struct {
	ctrl     *jobrun.Controller
	entryMgr dentry.Manager
	notify   *notify.Notify
	recorder metastore.ScheduledTaskRecorder
	config   config.Workflow
	fuse     config.FUSE
	logger   *zap.SugaredLogger
}

var _ Manager = &manager{}

func NewManager(entryMgr dentry.Manager, notify *notify.Notify, recorder metastore.ScheduledTaskRecorder, config config.Workflow, fuse config.FUSE) (Manager, error) {
	wfLogger = logger.NewLogger("workflow")

	if !config.Enable {
		return disabledManager{}, nil
	}

	if err := initWorkflowJobRootWorkdir(&config); err != nil {
		return nil, fmt.Errorf("init workflow job root workdir error: %s", err)
	}

	if err := exec.RegisterOperators(entryMgr, exec.LocalConfig{Workflow: config}); err != nil {
		return nil, fmt.Errorf("register operators failed: %s", err)
	}

	flowCtrl := jobrun.NewJobController(recorder)
	mgr := &manager{ctrl: flowCtrl, entryMgr: entryMgr, notify: notify, recorder: recorder, config: config, fuse: fuse, logger: wfLogger}
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

	return mgr, nil
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
	return spec, nil
}

func (m *manager) DeleteWorkflow(ctx context.Context, wfId string) error {
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
	return m.recorder.DeleteWorkflow(ctx, wfId)
}

func (m *manager) ListJobs(ctx context.Context, wfId string) ([]*types.WorkflowJob, error) {
	result, err := m.recorder.ListWorkflowJob(ctx, types.JobFilter{WorkFlowID: wfId})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (m *manager) TriggerWorkflow(ctx context.Context, wfId string, entryID int64, attr JobAttr) (*types.WorkflowJob, error) {
	workflow, err := m.GetWorkflow(ctx, wfId)
	if err != nil {
		return nil, err
	}
	if entryID == 0 {
		return nil, fmt.Errorf("no entry martch")
	}

	m.logger.Infow("receive workflow", "workflow", workflow.Name, "entryID", entryID)
	var en *types.Metadata
	en, err = m.entryMgr.GetEntry(ctx, entryID)
	if err != nil {
		m.logger.Errorw("query entry failed", "workflow", workflow.Name, "entryID", entryID, "err", err)
		return nil, err
	}

	job, err := assembleWorkflowJob(workflow, en)
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
	jobs, err := m.recorder.ListWorkflowJob(ctx, types.JobFilter{JobID: jobId})
	if err != nil {
		return err
	}
	if len(jobs) == 0 {
		return types.ErrNotFound
	}
	if jobs[0].Status != jobrun.RunningStatus {
		return fmt.Errorf("pausing is not supported in non-running state")
	}
	return m.ctrl.PauseJob(jobId)
}

func (m *manager) ResumeWorkflowJob(ctx context.Context, jobId string) error {
	jobs, err := m.recorder.ListWorkflowJob(ctx, types.JobFilter{JobID: jobId})
	if err != nil {
		return err
	}
	if len(jobs) == 0 {
		return types.ErrNotFound
	}
	if jobs[0].Status != jobrun.PausedStatus {
		return fmt.Errorf("resuming is not supported in non-paused state")
	}
	return m.ctrl.ResumeJob(jobId)
}

func (m *manager) CancelWorkflowJob(ctx context.Context, jobId string) error {
	jobs, err := m.recorder.ListWorkflowJob(ctx, types.JobFilter{JobID: jobId})
	if err != nil {
		return err
	}
	if len(jobs) == 0 {
		return types.ErrNotFound
	}
	if !jobs[0].FinishAt.IsZero() {
		return fmt.Errorf("canceling is not supported in finished state")
	}
	return m.ctrl.CancelJob(jobId)
}

type disabledManager struct{}

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

func (d disabledManager) ListJobs(ctx context.Context, wfId string) ([]*types.WorkflowJob, error) {
	return nil, types.ErrNotFound
}

func (d disabledManager) TriggerWorkflow(ctx context.Context, wfId string, entryID int64, attr JobAttr) (*types.WorkflowJob, error) {
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
	JobID  string
	Reason string
}
