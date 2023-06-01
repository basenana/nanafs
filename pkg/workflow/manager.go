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
	"github.com/basenana/go-flow/flow"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"time"
)

type Manager interface {
	ListWorkflows(ctx context.Context) ([]*types.WorkflowSpec, error)
	GetWorkflow(ctx context.Context, wfId string) (*types.WorkflowSpec, error)
	CreateWorkflow(ctx context.Context, spec *types.WorkflowSpec) (*types.WorkflowSpec, error)
	UpdateWorkflow(ctx context.Context, spec *types.WorkflowSpec) (*types.WorkflowSpec, error)
	DeleteWorkflow(ctx context.Context, wfId string) error
	ListJobs(ctx context.Context, wfId string) ([]*types.WorkflowJob, error)

	TriggerWorkflow(ctx context.Context, wfId string, entryID int64) (*types.WorkflowJob, error)
	PauseWorkflowJob(ctx context.Context, jobId string) error
	ResumeWorkflowJob(ctx context.Context, jobId string) error
	CancelWorkflowJob(ctx context.Context, jobId string) error
}

type manager struct {
	recorder metastore.ScheduledTaskRecorder
	runner   *Runner
	config   config.Workflow
	logger   *zap.SugaredLogger
}

var _ Manager = &manager{}

func NewManager(recorder metastore.ScheduledTaskRecorder) (Manager, error) {
	runner, err := InitWorkflowRunner(recorder)
	if err != nil {
		return nil, err
	}

	mgr := &manager{
		recorder: recorder,
		runner:   runner,
		logger:   logger.NewLogger("workflowManager"),
	}
	//InitWorkflowMirrorPlugin(mgr)
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
	if err := m.recorder.SaveWorkflow(ctx, spec); err != nil {
		return nil, err
	}
	return spec, nil
}

func (m *manager) UpdateWorkflow(ctx context.Context, spec *types.WorkflowSpec) (*types.WorkflowSpec, error) {
	if spec.Id == "" {
		return nil, fmt.Errorf("workflow id is empty")
	}
	if spec.Name == "" {
		return nil, fmt.Errorf("workflow name is empty")
	}
	spec.UpdatedAt = time.Now()
	if err := m.recorder.SaveWorkflow(ctx, spec); err != nil {
		return nil, err
	}
	return spec, nil
}

func (m *manager) DeleteWorkflow(ctx context.Context, wfId string) error {
	jobs, err := m.ListJobs(ctx, wfId)
	if err != nil {
		return err
	}
	for _, j := range jobs {
		if j.Status == flow.PausedStatus || j.Status == flow.RunningStatus {
			err = m.CancelWorkflowJob(ctx, wfId)
			if err != nil {
				return err
			}
		}
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

func (m *manager) TriggerWorkflow(ctx context.Context, wfId string, entryID int64) (*types.WorkflowJob, error) {
	workflow, err := m.GetWorkflow(ctx, wfId)
	if err != nil {
		return nil, err
	}

	m.logger.Infow("receive workflow", "workflow", workflow.Name)
	job, err := assembleWorkflowJob(workflow, entryID)
	if err != nil {
		m.logger.Errorw("assemble job failed", "workflow", workflow.Name, "err", err)
		return nil, err
	}

	err = m.recorder.SaveWorkflowJob(ctx, job)
	if err != nil {
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
		return nil
	}
	if jobs[0].Status != flow.RunningStatus {
		return fmt.Errorf("pausing is not supported in non-running state")
	}
	return m.runner.PauseFlow(jobId)
}

func (m *manager) ResumeWorkflowJob(ctx context.Context, jobId string) error {
	jobs, err := m.recorder.ListWorkflowJob(ctx, types.JobFilter{JobID: jobId})
	if err != nil {
		return err
	}
	if len(jobs) == 0 {
		return nil
	}
	if jobs[0].Status != flow.PausedStatus {
		return fmt.Errorf("resuming is not supported in non-paused state")
	}
	return m.runner.ResumeFlow(jobId)
}

func (m *manager) CancelWorkflowJob(ctx context.Context, jobId string) error {
	jobs, err := m.recorder.ListWorkflowJob(ctx, types.JobFilter{JobID: jobId})
	if err != nil {
		return err
	}
	if len(jobs) == 0 {
		return nil
	}
	if !jobs[0].FinishAt.IsZero() {
		return fmt.Errorf("canceling is not supported in finished state")
	}
	return m.runner.CancelFlow(jobId)
}
