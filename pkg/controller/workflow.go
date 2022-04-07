package controller

import (
	"github.com/basenana/go-flow/fsm"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/pkg/workflow"
)

type WorkflowController interface {
	SaveWorkflow(name string, rule types.Rule, plugins []plugin.Plugin) *workflow.Workflow
	DeleteWorkFlow(name string)
	GetWorkflows() []*workflow.Workflow
	Trigger(o types.Object)
	GetJobs() map[fsm.Status]*workflow.Job
}

func (c *controller) SaveWorkflow(name string, rule types.Rule, plugins []plugin.Plugin) *workflow.Workflow {
	return nil
}

func (c *controller) DeleteWorkFlow(name string) {
	//TODO implement me
	panic("implement me")
}

func (c *controller) GetWorkflows() []*workflow.Workflow {
	//TODO implement me
	panic("implement me")
}

func (c *controller) Trigger(o types.Object) {
	//TODO implement me
	panic("implement me")
}

func (c *controller) GetJobs() map[fsm.Status]*workflow.Job {
	//TODO implement me
	panic("implement me")
}
