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
	"github.com/basenana/go-flow/cfg"
	"github.com/basenana/go-flow/exec"
	"github.com/basenana/go-flow/flow"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"os"
	"strconv"
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

func init() {
	flow.RegisterExecutorBuilder("local", exec.NewLocalExecutor)
}

type manager struct {
	ctrl     *flow.Controller
	recorder metastore.ScheduledTaskRecorder
	config   config.Workflow
	logger   *zap.SugaredLogger
}

var _ Manager = &manager{}

func NewManager(recorder metastore.ScheduledTaskRecorder, config config.Workflow) (Manager, error) {
	wdInfo, err := os.Stat(config.JobWorkdir)
	if err != nil {
		return nil, err
	}
	if !wdInfo.IsDir() {
		return nil, fmt.Errorf("%s not a dir", config.JobWorkdir)
	}

	cfg.LocalWorkdirBase = config.JobWorkdir

	l := logger.NewLogger("workflow")
	_ = logger.NewFlowLogger(l)
	if err = registerOperators(); err != nil {
		return nil, fmt.Errorf("register operators failed: %s", err)
	}
	flowCtrl := flow.NewFlowController(&storageWrapper{recorder: recorder, logger: l})

	mgr := &manager{
		ctrl:     flowCtrl,
		recorder: recorder,
		logger:   l,
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

	m.logger.Infow("receive workflow", "workflow", workflow.Name, "entryID", entryID)
	var en dentry.Entry
	if entryID != 0 {
		// TODO
	}
	job, err := assembleWorkflowJob(workflow, en)
	if err != nil {
		m.logger.Errorw("assemble job failed", "workflow", workflow.Name, "err", err)
		return nil, err
	}

	err = m.recorder.SaveWorkflowJob(ctx, job)
	if err != nil {
		return nil, err
	}

	if err = m.ctrl.TriggerFlow(ctx, job.Id); err != nil {
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
		return nil
	}
	if jobs[0].Status != flow.RunningStatus {
		return fmt.Errorf("pausing is not supported in non-running state")
	}
	return m.ctrl.PauseFlow(jobId)
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
	return m.ctrl.ResumeFlow(jobId)
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
	return m.ctrl.CancelFlow(jobId)
}

func assembleWorkflowJob(spec *types.WorkflowSpec, entry dentry.Entry) (*types.WorkflowJob, error) {
	j := &types.WorkflowJob{
		Id:        uuid.New().String(),
		Workflow:  spec.Id,
		Target:    types.WorkflowTarget{Rule: &spec.Rule},
		Status:    flow.InitializingStatus,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	globalParam := map[string]string{}
	if entry != nil {
		globalParam[paramEntryIdKey] = strconv.FormatInt(entry.Metadata().ID, 10)
		globalParam[paramEntryPathKey] = entry.Metadata().Name
		j.Target.EntryID = &entry.Metadata().ID
		j.Steps = append(j.Steps, types.WorkflowJobStep{
			StepName: opEntryInit,
			Status:   flow.InitializingStatus,
			Operator: &types.WorkflowJobOperator{
				Name:       opEntryInit,
				Parameters: globalParam,
			},
		})
	}

	for _, stepSpec := range spec.Steps {
		if stepSpec.Plugin != nil {
			for k, v := range globalParam {
				stepSpec.Plugin.Parameters[k] = v
			}
		}
		j.Steps = append(j.Steps,
			types.WorkflowJobStep{
				StepName: stepSpec.Name,
				Status:   flow.InitializingStatus,
				Plugin:   stepSpec.Plugin,
				Script:   stepSpec.Script,
			},
		)
	}

	j.Steps = append(j.Steps, types.WorkflowJobStep{
		StepName: opEntryCollect,
		Status:   flow.InitializingStatus,
		Operator: &types.WorkflowJobOperator{
			Name:       opEntryCollect,
			Parameters: globalParam,
		},
	})

	return j, nil
}

func assembleFlow(job *types.WorkflowJob) (*flow.Flow, error) {
	f := &flow.Flow{
		ID:            job.Id,
		Executor:      "local",
		Status:        job.Status,
		Message:       job.Message,
		ControlPolicy: flow.ControlPolicy{FailedPolicy: flow.PolicyFastFailed},
	}

	for _, step := range job.Steps {
		var t flow.Task
		switch {
		case step.Plugin != nil:
			param := map[string]string{
				paramPluginName:    step.Plugin.PluginName,
				paramPluginVersion: step.Plugin.Version,
				paramPluginType:    string(step.Plugin.PluginType),
			}
			for k, v := range step.Plugin.Parameters {
				if _, ok := param[k]; ok {
					continue
				}
				param[k] = v
			}
			t = flow.Task{
				Name:    step.StepName,
				Status:  step.Status,
				Message: step.Message,
				OperatorSpec: flow.Spec{
					Type:      opPluginCall,
					Parameter: param,
				},
				RetryOnFailed: 1,
			}
		case step.Operator != nil:
			t = flow.Task{
				Name: step.Operator.Name,
				OperatorSpec: flow.Spec{
					Type:      step.Operator.Name,
					Parameter: step.Operator.Parameters,
				},
				RetryOnFailed: 1,
			}
		case step.Script != nil:
			if step.Script.Type == exec.ShellOperator || step.Script.Type == exec.PythonOperator {
				t = flow.Task{
					Name: step.Operator.Name,
					OperatorSpec: flow.Spec{
						Type:   step.Script.Type,
						Script: &flow.Script{Content: step.Script.Content, Command: step.Script.Command},
						Env:    step.Script.Env,
					},
					RetryOnFailed: 1,
				}
			} else {
				return nil, fmt.Errorf("step has unknown script type %s", step.Script.Type)
			}

		}
		f.Tasks = append(f.Tasks, t)
	}

	f.Tasks = append(f.Tasks, flow.Task{
		Name: opEntryCollect,
		OperatorSpec: flow.Spec{
			Type:      opEntryCollect,
			Parameter: map[string]string{},
		},
		RetryOnFailed: 1,
	})

	for i := 1; i < len(f.Tasks); i++ {
		f.Tasks[i-1].Next.OnSucceed = f.Tasks[i].Name
	}

	return f, nil
}
