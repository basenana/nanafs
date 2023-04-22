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
	"github.com/basenana/go-flow/flow"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
)

const (
	dataGroupWorkflow = "workflow"
	dataGroupJob      = "job"
)

type Manager interface {
	ListWorkflows(ctx context.Context) ([]types.WorkflowSpec, error)
	GetWorkflow(ctx context.Context, wfId string) (types.WorkflowSpec, error)
	SaveWorkflow(ctx context.Context, spec *types.WorkflowSpec) (*types.WorkflowSpec, error)
	DeleteWorkflow(ctx context.Context, wfId string) error
	ListJobs(ctx context.Context, wfId string) ([]types.WorkflowJob, error)

	TriggerWorkflow(ctx context.Context, wfId string) (types.WorkflowJob, error)
	PauseWorkflowJob(ctx context.Context, jobId string) error
	ResumeWorkflowJob(ctx context.Context, jobId string) error
	CancelWorkflowJob(ctx context.Context, jobId string) error
}

type manager struct {
	recorder metastore.PluginRecorder
	runner   *Runner
	logger   *zap.SugaredLogger
}

var _ Manager = &manager{}

func NewManager(recorder metastore.PluginRecorder) (Manager, error) {
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

func (m *manager) ListWorkflows(ctx context.Context) ([]types.WorkflowSpec, error) {
	wfIdList, err := m.recorder.ListRecords(ctx, dataGroupWorkflow)
	if err != nil {
		return nil, err
	}

	result := make([]types.WorkflowSpec, len(wfIdList))
	for i, wfId := range wfIdList {
		spec := types.WorkflowSpec{}
		if err = m.recorder.GetRecord(ctx, wfId, &spec); err != nil {
			return nil, err
		}
		result[i] = spec
	}
	return result, nil
}

func (m *manager) GetWorkflow(ctx context.Context, wfId string) (types.WorkflowSpec, error) {
	spec := types.WorkflowSpec{}
	if err := m.recorder.GetRecord(ctx, wfId, &spec); err != nil {
		return spec, err
	}
	return spec, nil
}

func (m *manager) SaveWorkflow(ctx context.Context, spec *types.WorkflowSpec) (*types.WorkflowSpec, error) {
	wfId, err := nextWorkflowId(ctx, m.recorder)
	if err != nil {
		return nil, err
	}
	spec.Id = wfId
	if err = m.recorder.SaveRecord(ctx, dataGroupWorkflow, wfId, &spec); err != nil {
		return spec, err
	}
	return spec, nil
}

func (m *manager) DeleteWorkflow(ctx context.Context, wfId string) error {
	return m.recorder.DeleteRecord(ctx, wfId)
}

func (m *manager) ListJobs(ctx context.Context, wfId string) ([]types.WorkflowJob, error) {
	jobIdList, err := m.recorder.ListRecords(ctx, jobGroupId(wfId))
	if err != nil {
		return nil, err
	}

	result := make([]types.WorkflowJob, len(jobIdList))
	for i, jobId := range jobIdList {
		job := types.WorkflowJob{}
		if err = m.recorder.GetRecord(ctx, jobId, &job); err != nil {
			return nil, err
		}
		result[i] = job
	}
	return result, nil
}

func (m *manager) TriggerWorkflow(ctx context.Context, wfId string) (types.WorkflowJob, error) {
	workflow, err := m.GetWorkflow(ctx, wfId)
	if err != nil {
		return types.WorkflowJob{}, err
	}

	job, err := m.runner.WorkFlowHandler(ctx, &workflow)
	if err != nil {
		return types.WorkflowJob{}, err
	}

	return *job, nil
}

func (m *manager) PauseWorkflowJob(ctx context.Context, jobId string) error {
	flowId := flow.FID(jobId)
	_, err := m.runner.GetFlow(flowId)
	if err != nil {
		return err
	}
	return m.runner.PauseFlow(flow.FID(jobId))
}

func (m *manager) ResumeWorkflowJob(ctx context.Context, jobId string) error {
	flowId := flow.FID(jobId)
	_, err := m.runner.GetFlow(flowId)
	if err != nil {
		return err
	}
	return m.runner.ResumeFlow(flow.FID(jobId))
}

func (m *manager) CancelWorkflowJob(ctx context.Context, jobId string) error {
	flowId := flow.FID(jobId)
	_, err := m.runner.GetFlow(flowId)
	if err != nil {
		return err
	}
	return m.runner.CancelFlow(flow.FID(jobId))
}
