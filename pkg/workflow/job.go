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
	"fmt"
	"github.com/basenana/go-flow/flow"
	"github.com/basenana/go-flow/fsm"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/plugin/common"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func assembleWorkflowJob(spec *types.WorkflowSpec, entryID int64) (*types.WorkflowJob, error) {
	j := &types.WorkflowJob{
		Id:       uuid.New().String(),
		Workflow: spec.Id,
		Target: types.WorkflowTarget{
			EntryID: &entryID,
			Rule:    &spec.Rule,
		},
		Status: string(flow.CreatingStatus),
		Steps:  make([]types.WorkflowJobStep, len(spec.Steps)),
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

type Job struct {
	*types.WorkflowJob

	localEntryPath string

	cfg    config.Workflow
	entry  dentry.Manager
	logger *zap.SugaredLogger
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
	if err := initJobWorkDir(n.cfg.JobWorkdir, n.WorkflowJob.Id); err != nil {
		ctx.Fail(fmt.Sprintf("init job work dir failed: %s", err), 3)
		return err
	}
	if n.localEntryPath == "" && n.WorkflowJob.Target.EntryID != nil {
		entry, err := n.entry.GetEntry(ctx.Context, *n.WorkflowJob.Target.EntryID)
		if err != nil {
			ctx.Fail(fmt.Sprintf("load entry failed: %s", err), 3)
			return err
		}
		f, err := n.entry.Open(ctx.Context, entry, dentry.Attr{Read: true})
		if err != nil {
			ctx.Fail(fmt.Sprintf("open entry failed: %s", err), 3)
			return err
		}
		defer f.Close(ctx.Context)
		if n.localEntryPath, err = copyEntryToJobWorkDir(ctx.Context, n.cfg.JobWorkdir, n.WorkflowJob.Id, f); err != nil {
			ctx.Fail(fmt.Sprintf("copy entry file failed: %s", err), 3)
			return err
		}
	}
	ctx.Succeed()
	return nil
}

func (n *Job) Teardown(ctx *flow.Context) {
	if err := cleanUpJobWorkDir(n.cfg.JobWorkdir, n.WorkflowJob.Id); err != nil {
		ctx.Fail(fmt.Sprintf("clean up job work dir failed: %s", err), 3)
		return
	}
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

	workPath       string
	localEntryPath string
	job            *Job
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
	req.WorkPath = n.workPath
	req.EntryPath = n.localEntryPath
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
