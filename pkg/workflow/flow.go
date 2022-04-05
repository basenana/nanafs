package workflow

import (
	"fmt"
	"github.com/basenana/go-flow/flow"
	"github.com/basenana/go-flow/fsm"
	"github.com/basenana/nanafs/pkg/object"
	"github.com/basenana/nanafs/pkg/plugin"
)

type NanaFlow struct {
	id      flow.FID
	name    string
	status  fsm.Status
	message string

	tasks []*NanaTask
}

func (n *NanaFlow) GetStatus() fsm.Status {
	return n.status
}

func (n *NanaFlow) SetStatus(status fsm.Status) {
	n.status = status
}

func (n *NanaFlow) GetMessage() string {
	return n.message
}

func (n *NanaFlow) SetMessage(msg string) {
	n.message = msg
}

func (n NanaFlow) ID() flow.FID {
	return n.id
}

func (n NanaFlow) Type() flow.FType {
	return "NanaFlow"
}

func (n NanaFlow) GetHooks() flow.Hooks {
	return map[flow.HookType]flow.Hook{}
}

func (n NanaFlow) Setup(ctx *flow.Context) error {
	ctx.Succeed()
	return nil
}

func (n NanaFlow) Teardown(ctx *flow.Context) {
	ctx.Succeed()
	return
}

func (n *NanaFlow) NextBatch(ctx *flow.Context) ([]flow.Task, error) {
	if len(n.tasks) == 0 {
		return nil, nil
	}

	t := n.tasks[0]
	n.tasks = n.tasks[1:]

	return []flow.Task{t}, nil
}

func (n NanaFlow) GetControlPolicy() flow.ControlPolicy {
	return flow.ControlPolicy{
		FailedPolicy: flow.PolicyFastFailed,
	}
}

type NanaTask struct {
	name    flow.TName
	message string
	status  fsm.Status
	object  object.Object

	plugin plugin.Plugin
}

func (n *NanaTask) GetStatus() fsm.Status {
	return n.status
}

func (n *NanaTask) SetStatus(status fsm.Status) {
	n.status = status
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
