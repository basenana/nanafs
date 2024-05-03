/*
 Copyright 2024 NanaFS Authors.

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

package controller

import (
	"context"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/pkg/workflow"
	"runtime/trace"
)

func (c *controller) ListWorkflows(ctx context.Context) ([]*types.WorkflowSpec, error) {
	defer trace.StartRegion(ctx, "controller.ListWorkflows").End()
	return c.workflow.ListWorkflows(ctx)
}

func (c *controller) GetWorkflow(ctx context.Context, wfId string) (*types.WorkflowSpec, error) {
	defer trace.StartRegion(ctx, "controller.GetWorkflow").End()
	return c.workflow.GetWorkflow(ctx, wfId)
}

func (c *controller) CreateWorkflow(ctx context.Context, spec *types.WorkflowSpec) (*types.WorkflowSpec, error) {
	defer trace.StartRegion(ctx, "controller.CreateWorkflow").End()
	return c.workflow.CreateWorkflow(ctx, spec)
}

func (c *controller) UpdateWorkflow(ctx context.Context, spec *types.WorkflowSpec) (*types.WorkflowSpec, error) {
	defer trace.StartRegion(ctx, "controller.UpdateWorkflow").End()
	return c.workflow.UpdateWorkflow(ctx, spec)
}

func (c *controller) DeleteWorkflow(ctx context.Context, wfId string) error {
	defer trace.StartRegion(ctx, "controller.DeleteWorkflow").End()
	return c.workflow.DeleteWorkflow(ctx, wfId)
}

func (c *controller) ListJobs(ctx context.Context, wfId string) ([]*types.WorkflowJob, error) {
	defer trace.StartRegion(ctx, "controller.ListJobs").End()
	return c.workflow.ListJobs(ctx, wfId)
}

func (c *controller) GetJob(ctx context.Context, wfId string, jobID string) (*types.WorkflowJob, error) {
	defer trace.StartRegion(ctx, "controller.GetJob").End()
	return c.workflow.GetJob(ctx, wfId, jobID)
}

func (c *controller) PauseWorkflowJob(ctx context.Context, jobId string) error {
	defer trace.StartRegion(ctx, "controller.PauseWorkflowJob").End()
	return c.workflow.PauseWorkflowJob(ctx, jobId)
}

func (c *controller) ResumeWorkflowJob(ctx context.Context, jobId string) error {
	defer trace.StartRegion(ctx, "controller.ResumeWorkflowJob").End()
	return c.workflow.ResumeWorkflowJob(ctx, jobId)
}

func (c *controller) CancelWorkflowJob(ctx context.Context, jobId string) error {
	defer trace.StartRegion(ctx, "controller.CancelWorkflowJob").End()
	return c.workflow.CancelWorkflowJob(ctx, jobId)
}

func (c *controller) TriggerWorkflow(ctx context.Context, wfId string, tgt types.WorkflowTarget, attr workflow.JobAttr) (*types.WorkflowJob, error) {
	defer trace.StartRegion(ctx, "controller.TriggerWorkflow").End()
	return c.workflow.TriggerWorkflow(ctx, wfId, tgt, attr)
}
