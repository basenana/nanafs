package workflow

import (
	"context"
	goflow "github.com/basenana/go-flow/flow"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
)

type Workflow struct {
	obj     types.Object
	Name    string          `json:"name"`
	Rule    *types.Rule     `json:"rule"`
	Plugins []plugin.Plugin `json:"plugins"`
}

func NewWorkflow(name string, rule *types.Rule, plugins []plugin.Plugin) *Workflow {
	return &Workflow{
		Name:    name,
		Rule:    rule,
		Plugins: plugins,
	}
}

func NewNanaJob(ctrl controller.Controller, workflow *Workflow, jobObj, value *types.Object) (*NanaJob, *types.Job, error) {
	nanaTasks := make([]*NanaTask, len(workflow.Plugins))
	tasks := make([]types.Task, len(workflow.Plugins))
	for i, p := range workflow.Plugins {
		nanaTasks[i] = &NanaTask{
			ctrl:   ctrl,
			jobId:  jobObj.ID,
			name:   goflow.TName(p.Name()),
			status: goflow.CreatingStatus,
			plugin: p,
			object: value,
		}
		tasks[i] = types.Task{
			Name:   p.Name(),
			Status: string(goflow.CreatingStatus),
		}
	}
	nanaJob := NanaJob{
		logger:   logger.NewLogger("Job"),
		ctrl:     ctrl,
		Id:       jobObj.ID,
		workflow: workflow,
		object:   value,
		status:   goflow.CreatingStatus,
		tasks:    nanaTasks,
	}
	err := FlowStorage.SaveFlow(&nanaJob)
	if err != nil {
		panic(err)
	}

	job := &types.Job{
		Id:           jobObj.ID,
		WorkflowName: workflow.Name,
		Status:       string(goflow.CreatingStatus),
		Tasks:        tasks,
	}
	return &nanaJob, job, nil
}

func (n *NanaJob) Run() error {
	if err := FlowCtl.TriggerFlow(context.TODO(), n.ID()); err != nil {
		return err
	}
	return nil
}
