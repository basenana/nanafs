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
	flowcontroller "github.com/basenana/go-flow/controller"
	goflowctrl "github.com/basenana/go-flow/controller"
	"github.com/basenana/go-flow/flow"
	"github.com/basenana/go-flow/fsm"
	flowstorage "github.com/basenana/go-flow/storage"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/plugin/common"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"sync"
)

var (
	ErrJobNotFound = errors.New("job not found")
)

type Runner struct {
	*flowcontroller.FlowController

	stopCh   chan struct{}
	recorder metastore.ScheduledTaskRecorder
	logger   *zap.SugaredLogger

	sync.RWMutex
}

func InitWorkflowRunner(recorder metastore.ScheduledTaskRecorder) (*Runner, error) {
	runner := &Runner{
		recorder: recorder,
		logger:   logger.NewLogger("workflowRuntime"),
	}

	var err error
	flowCtrl, err := goflowctrl.NewFlowController(goflowctrl.Option{Storage: runner})
	if err != nil {
		return nil, err
	}
	if err = flowCtrl.Register(&Job{}); err != nil {
		return nil, err
	}
	runner.FlowController = flowCtrl

	return runner, nil
}

func (r *Runner) Start(stopCh chan struct{}) error {
	return nil
}

func (r *Runner) WorkFlowHandler(ctx context.Context, wf *types.WorkflowSpec) (*types.WorkflowJob, error) {
	r.logger.Infow("receive workflow", "workflow", wf.Name)

	job, err := assembleWorkflowJob(wf)
	if err != nil {
		r.logger.Errorw("assemble job failed", "workflow", wf.Name, "err", err)
		return nil, err
	}

	err = r.recorder.SaveWorkflowJob(ctx, job)
	if err != nil {
		return nil, err
	}

	go r.triggerJob(context.TODO(), &Job{
		WorkflowJob: job,
		logger:      r.logger.With(zap.String("job", job.Id)),
	})

	return job, nil
}

func (r *Runner) triggerJob(ctx context.Context, job *Job) {
	if err := r.SaveFlow(job); err != nil {
		r.logger.Errorw("save job failed", "err", err)
		return
	}

	if err := r.TriggerFlow(ctx, job.ID()); err != nil {
		r.logger.Errorw("trigger job flow failed", "job", job.ID(), "err", err)
	}
	return
}

func (r *Runner) GetFlow(flowId flow.FID) (flow.Flow, error) {
	wfJob, err := r.recorder.ListWorkflowJob(context.Background(), types.JobFilter{JobID: string(flowId)})
	if err != nil {
		r.logger.Errorw("load job failed", "err", err)
		return nil, err
	}
	if len(wfJob) == 0 {
		return nil, types.ErrNotFound
	}

	job := &Job{
		WorkflowJob: wfJob[0],
		logger:      r.logger.With(zap.String("job", string(flowId))),
	}
	return job, nil
}

func (r *Runner) GetFlowMeta(flowId flow.FID) (*flowstorage.FlowMeta, error) {
	flowJob, err := r.GetFlow(flowId)
	if err != nil {
		return nil, err
	}

	job, ok := flowJob.(*Job)
	if !ok {
		return nil, ErrJobNotFound
	}

	result := &flowstorage.FlowMeta{
		Type:       job.Type(),
		Id:         job.ID(),
		Status:     job.GetStatus(),
		TaskStatus: map[flow.TName]fsm.Status{},
	}
	for _, step := range job.Steps {
		result.TaskStatus[flow.TName(step.StepName)] = fsm.Status(step.Status)
	}
	return result, nil
}

func (r *Runner) SaveFlow(flow flow.Flow) error {
	job, ok := flow.(*Job)
	if !ok {
		return fmt.Errorf("flow %s not a Job object", flow.ID())
	}

	err := r.recorder.SaveWorkflowJob(context.Background(), job.WorkflowJob)
	if err != nil {
		r.logger.Errorw("save job to metadb failed", "err", err)
		return err
	}

	return nil
}

func (r *Runner) DeleteFlow(flowId flow.FID) error {
	err := r.recorder.DeleteWorkflowJob(context.Background(), string(flowId))
	if err != nil {
		r.logger.Errorw("delete job to metadb failed", "err", err)
		return err
	}
	return nil
}

func (r *Runner) SaveTask(flowId flow.FID, task flow.Task) error {
	r.Lock()
	defer r.Unlock()

	flowJob, err := r.GetFlow(flowId)
	if err != nil {
		return err
	}

	job, ok := flowJob.(*Job)
	if !ok {
		return ErrJobNotFound
	}

	for i, step := range job.Steps {
		if step.StepName == string(task.Name()) {
			job.Steps[i].Status = string(task.GetStatus())
			job.Steps[i].Message = task.GetMessage()
			break
		}
	}

	return r.SaveFlow(job)
}

func (r *Runner) DeleteTask(flowId flow.FID, taskName flow.TName) error {
	return nil
}

type Job struct {
	*types.WorkflowJob
	logger *zap.SugaredLogger
}

func assembleWorkflowJob(spec *types.WorkflowSpec) (*types.WorkflowJob, error) {
	j := &types.WorkflowJob{
		Id:       uuid.New().String(),
		Workflow: spec.Id,
		Status:   string(flow.CreatingStatus),
		Steps:    make([]types.WorkflowJobStep, len(spec.Steps)),
	}

	for i, stepSpec := range spec.Steps {
		j.Steps[i] = types.WorkflowJobStep{
			StepName: stepSpec.Name,
			Status:   string(flow.CreatingStatus),
			Plugin:   stepSpec.Plugin,
		}
	}

	return j, nil
}

var _ flow.Flow = &Job{}

func (n *Job) GetStatus() fsm.Status {
	return fsm.Status(n.Status)
}

func (n *Job) SetStatus(status fsm.Status) {
	n.Status = string(status)
}

func (n *Job) SetStepStatus(stepName flow.TName, status fsm.Status) {
	for i := range n.Steps {
		if n.Steps[i].StepName == string(stepName) {
			n.Steps[i].Status = string(status)
		}
	}
}

func (n *Job) GetMessage() string {
	return n.Message
}

func (n *Job) SetMessage(msg string) {
	n.Message = msg
}

func (n *Job) ID() flow.FID {
	return flow.FID(n.Id)
}

func (n *Job) Type() flow.FType {
	return "Job"
}

func (n *Job) GetHooks() flow.Hooks {
	return map[flow.HookType]flow.Hook{}
}

func (n *Job) Setup(ctx *flow.Context) error {
	ctx.Succeed()
	return nil
}

func (n *Job) Teardown(ctx *flow.Context) {
	ctx.Succeed()
	return
}

func (n *Job) NextBatch(ctx *flow.Context) ([]flow.Task, error) {
	if len(n.Steps) == 0 {
		return nil, nil
	}

	t := n.Steps[0]
	n.Steps = n.Steps[1:]

	return []flow.Task{&JobStep{WorkflowJobStep: &t, job: n}}, nil
}

func (n *Job) GetControlPolicy() flow.ControlPolicy {
	return flow.ControlPolicy{
		FailedPolicy: flow.PolicyFastFailed,
	}
}

type JobStep struct {
	*types.WorkflowJobStep
	job *Job
}

var _ flow.Task = &JobStep{}

func (n *JobStep) GetStatus() fsm.Status {
	return fsm.Status(n.Status)
}

func (n *JobStep) SetStatus(status fsm.Status) {
	n.Status = string(status)
}

func (n *JobStep) GetMessage() string {
	return n.Message
}

func (n *JobStep) SetMessage(msg string) {
	n.Message = msg
}

func (n *JobStep) Name() flow.TName {
	return flow.TName(n.StepName)
}

func (n *JobStep) Setup(ctx *flow.Context) error {
	ctx.Succeed()
	return nil
}

func (n *JobStep) Do(ctx *flow.Context) error {
	req := common.NewRequest()
	_, err := pluginCall(ctx.Context, n.Plugin, req)
	if err != nil {
		ctx.Fail(err.Error(), 0)
		return err
	}
	ctx.Succeed()
	return nil
}

func (n *JobStep) Teardown(ctx *flow.Context) {
	ctx.Succeed()
	return
}

var (
	pluginCall = plugin.Call
)
