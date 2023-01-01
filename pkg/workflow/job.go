package workflow

import (
	"github.com/basenana/go-flow/controller"
	"github.com/basenana/go-flow/flow"
	"github.com/basenana/go-flow/fsm"
	"github.com/basenana/go-flow/storage"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/types"
	"go.uber.org/zap"
)

var (
	flowCtrl    *controller.FlowController
	flowStorage storage.Interface
)

func init() {
	flowStorage = storage.NewInMemoryStorage()
	opt := controller.Option{
		Storage: flowStorage,
	}
	ctl, err := controller.NewFlowController(opt)
	if err != nil {
		panic(err)
	}
	flowCtrl = ctl
	if err := flowCtrl.Register(&Job{}); err != nil {
		panic(err)
	}
}

type Job struct {
	Id       string
	workflow *Workflow
	status   fsm.Status
	message  string
	steps    []*JobStep
	logger   *zap.SugaredLogger
}

var _ flow.Flow = &Job{}

func (n *Job) GetStatus() fsm.Status {
	return n.status
}

func (n *Job) SetStatus(status fsm.Status) {
	n.status = status
	if err := n.writeBack(); err != nil {
		n.logger.Errorf("update status to %s failed: %s", status, err)
	}
}

func (n *Job) SetStepStatus(stepName flow.TName, status fsm.Status) {
	for i := range n.steps {
		if n.steps[i].name == stepName {
			n.steps[i].status = status
		}
	}
	if err := n.writeBack(); err != nil {
		n.logger.Errorf("update status to %s failed: %s", status, err)
	}
}

func (n *Job) GetMessage() string {
	return n.message
}

func (n *Job) SetMessage(msg string) {
	n.message = msg
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
	if len(n.steps) == 0 {
		return nil, nil
	}

	t := n.steps[0]
	n.steps = n.steps[1:]

	return []flow.Task{t}, nil
}

func (n *Job) GetControlPolicy() flow.ControlPolicy {
	return flow.ControlPolicy{
		FailedPolicy: flow.PolicyFastFailed,
	}
}

func (n *Job) writeBack() error {
	return nil
}

type JobStep struct {
	job     *Job
	name    flow.TName
	message string
	status  fsm.Status
	object  *types.Object

	plugin plugin.RunnablePlugin
}

var _ flow.Task = &JobStep{}

func (n *JobStep) GetStatus() fsm.Status {
	return n.status
}

func (n *JobStep) SetStatus(status fsm.Status) {
	n.status = status
	n.job.SetStepStatus(n.name, n.status)
}

func (n *JobStep) GetMessage() string {
	return n.message
}

func (n *JobStep) SetMessage(msg string) {
	n.message = msg
}

func (n *JobStep) Name() flow.TName {
	return n.name
}

func (n *JobStep) Setup(ctx *flow.Context) error {
	ctx.Succeed()
	return nil
}

func (n *JobStep) Do(ctx *flow.Context) error {
	return nil
}

func (n *JobStep) Teardown(ctx *flow.Context) {
	ctx.Succeed()
	return
}
