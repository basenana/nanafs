package workflow

import (
	"context"
	goflow "github.com/basenana/go-flow/flow"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/google/uuid"
)

type Workflow struct {
	Name    string
	Rule    types.Rule
	Plugins []plugin.Plugin
}

type Job struct {
	Id       string
	workflow *Workflow
	Plugins  []plugin.Plugin
	object   *types.Object
	flow     *NanaFlow
}

func NewWorkflow(name string, rule types.Rule, plugins []plugin.Plugin) *Workflow {
	return &Workflow{
		Name:    name,
		Rule:    rule,
		Plugins: plugins,
	}
}

func NewJob(workflow *Workflow, value *types.Object) *Job {
	JobId := uuid.New().String()
	tasks := []*NanaTask{}
	for _, p := range workflow.Plugins {
		tasks = append(tasks, &NanaTask{
			name:   goflow.TName(p.Name()),
			status: goflow.CreatingStatus,
			plugin: p,
			object: value,
		})
	}
	f := NanaFlow{
		id:     goflow.FID(workflow.Name),
		name:   workflow.Name,
		status: goflow.CreatingStatus,
		tasks:  tasks,
	}
	err := FlowStorage.SaveFlow(&f)
	if err != nil {
		panic(err)
	}
	return &Job{
		Id:       JobId,
		workflow: workflow,
		Plugins:  workflow.Plugins,
		object:   value,
		flow:     &f,
	}
}

func (w *Job) Run() error {
	if err := FlowCtl.TriggerFlow(context.TODO(), w.flow.ID()); err != nil {
		return err
	}
	return nil
}
