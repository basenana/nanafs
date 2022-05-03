package workflow

import (
	"context"
	"fmt"
	"github.com/basenana/go-flow/flow"
	"github.com/basenana/go-flow/fsm"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/types"
	"go.uber.org/zap"
)

type NanaJob struct {
	ctrl   controller.Controller
	logger *zap.SugaredLogger

	Id       string
	workflow *Workflow
	object   *types.Object
	status   fsm.Status
	message  string

	tasks []*NanaTask
}

func (n *NanaJob) GetStatus() fsm.Status {
	return n.status
}

func (n *NanaJob) SetStatus(status fsm.Status) {
	n.status = status
	job, err := n.ctrl.GetObject(context.TODO(), n.Id)
	if err != nil {
		n.logger.Errorw("save job content error: %v", err)
		return
	}
	content := &types.Job{}
	err = n.ctrl.LoadStructureObject(context.TODO(), job, content)
	if err != nil {
		return
	}
	content.Status = string(status)
	err = n.ctrl.SaveStructureObject(context.TODO(), job, content)
	if err != nil {
		return
	}
}

func (n *NanaJob) GetMessage() string {
	return n.message
}

func (n *NanaJob) SetMessage(msg string) {
	n.message = msg
}

func (n NanaJob) ID() flow.FID {
	return flow.FID(n.Id)
}

func (n NanaJob) Type() flow.FType {
	return "NanaJob"
}

func (n NanaJob) GetHooks() flow.Hooks {
	return map[flow.HookType]flow.Hook{}
}

func (n NanaJob) Setup(ctx *flow.Context) error {
	ctx.Succeed()
	return nil
}

func (n NanaJob) Teardown(ctx *flow.Context) {
	ctx.Succeed()
	return
}

func (n *NanaJob) NextBatch(ctx *flow.Context) ([]flow.Task, error) {
	if len(n.tasks) == 0 {
		return nil, nil
	}

	t := n.tasks[0]
	n.tasks = n.tasks[1:]

	return []flow.Task{t}, nil
}

func (n NanaJob) GetControlPolicy() flow.ControlPolicy {
	return flow.ControlPolicy{
		FailedPolicy: flow.PolicyFastFailed,
	}
}

type NanaTask struct {
	ctrl    controller.Controller
	jobId   string
	name    flow.TName
	message string
	status  fsm.Status
	object  *types.Object

	plugin plugin.Plugin
}

func (n *NanaTask) GetStatus() fsm.Status {
	return n.status
}

func (n *NanaTask) SetStatus(status fsm.Status) {
	n.status = status
	job, err := n.ctrl.GetObject(context.TODO(), n.jobId)
	if err != nil {
		return
	}
	content := &types.Job{}
	err = n.ctrl.LoadStructureObject(context.TODO(), job, content)
	if err != nil {
		return
	}
	for i, t := range content.Tasks {
		if t.Name == string(n.name) {
			t.Status = string(status)
			content.Tasks[i] = t
			break
		}
	}
	err = n.ctrl.SaveStructureObject(context.TODO(), job, content)
	if err != nil {
		return
	}
}

func (n *NanaTask) GetMessage() string {
	return n.message
}

func (n *NanaTask) SetMessage(msg string) {
	n.message = msg
}

func (n *NanaTask) Name() flow.TName {
	return n.name
}

func (n *NanaTask) Setup(ctx *flow.Context) error {
	ctx.Succeed()
	return nil
}

func (n *NanaTask) Do(ctx *flow.Context) error {
	err := n.plugin.Run(n.object)
	if err != nil {
		ctx.Fail(fmt.Sprintf("err %v", err), 3)
		return err
	}
	ctx.Succeed()
	return nil
}

func (n *NanaTask) Teardown(ctx *flow.Context) {
	ctx.Succeed()
	return
}
