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

package jobrun

import (
	"github.com/basenana/go-flow"
	"github.com/basenana/nanafs/pkg/types"
	"strings"
)

var (
	InitializingStatus = flow.InitializingStatus
	RunningStatus      = flow.RunningStatus
	PausingStatus      = flow.PausedStatus
	SucceedStatus      = flow.SucceedStatus
	FailedStatus       = flow.FailedStatus
	ErrorStatus        = flow.ErrorStatus
	PausedStatus       = flow.PausedStatus
	CanceledStatus     = flow.CanceledStatus
)

type Task struct {
	job  *types.WorkflowJob
	step *types.WorkflowJobStep
}

func (t *Task) GetName() string {
	return t.step.StepName
}

func (t *Task) GetStatus() string {
	return t.step.Status
}

func (t *Task) SetStatus(s string) {
	t.step.Status = s
}

func (t *Task) GetMessage() string {
	return t.step.Message
}

func (t *Task) SetMessage(s string) {
	t.step.Message = s
}

func newTask(job *types.WorkflowJob, step *types.WorkflowJobStep) flow.Task {
	return &Task{job: job, step: step}
}

var _ flow.Task = &Task{}

type JobID struct {
	namespace string
	id        string
}

func (j JobID) FlowID() string {
	return j.namespace + "." + j.id
}

func NewJobID(flowID string) (jid JobID) {
	parts := strings.Split(flowID, ".")
	jid.namespace = parts[0]
	if len(parts) > 1 {
		jid.id = parts[1]
	}
	return
}

func workflowJob2Flow(ctrl *Controller, job *types.WorkflowJob) *flow.Flow {
	fb := flow.NewFlowBuilder(JobID{namespace: job.Namespace, id: job.Id}.FlowID()).
		Executor(newExecutor(ctrl, job)).
		Observer(ctrl)

	for i := range job.Steps {
		fb.Task(newTask(job, &job.Steps[i]))
	}

	return fb.Finish()
}
