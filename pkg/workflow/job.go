package workflow

import (
	"github.com/basenana/go-flow/flow"
	"github.com/basenana/go-flow/fsm"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/plugin/common"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Job struct {
	Id       string     `json:"id"`
	Workflow string     `json:"workflow"`
	Status   fsm.Status `json:"status"`
	Message  string     `json:"message"`
	Steps    []*JobStep `json:"steps"`

	logger *zap.SugaredLogger
}

func assembleWorkflowJob(spec *types.WorkflowSpec) (*Job, error) {
	jid := uuid.New().String()
	j := &Job{
		Id:     jid,
		Status: flow.CreatingStatus,
		Steps:  make([]*JobStep, len(spec.Steps)),
	}

	for i, stepSpec := range spec.Steps {
		j.Steps[i] = &JobStep{
			StepName: flow.TName(stepSpec.Name),
			Status:   flow.CreatingStatus,
			Plugin:   stepSpec.Plugin,
		}
	}

	return j, nil
}

var _ flow.Flow = &Job{}

func (n *Job) GetStatus() fsm.Status {
	return n.Status
}

func (n *Job) SetStatus(status fsm.Status) {
	n.Status = status
	if err := n.writeBack(); err != nil {
		n.logger.Errorf("update status to %s failed: %s", status, err)
	}
}

func (n *Job) SetStepStatus(stepName flow.TName, status fsm.Status) {
	for i := range n.Steps {
		if n.Steps[i].StepName == stepName {
			n.Steps[i].Status = status
		}
	}
	if err := n.writeBack(); err != nil {
		n.logger.Errorf("update status to %s failed: %s", status, err)
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
	n.logger = logger.NewLogger("workflow").With(zap.String("job", n.Id))
	for i := range n.Steps {
		n.Steps[i].job = n
	}
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
	job *Job

	StepName flow.TName      `json:"step_name"`
	Message  string          `json:"message"`
	Status   fsm.Status      `json:"status"`
	Plugin   types.PlugScope `json:"plugin"`
}

var _ flow.Task = &JobStep{}

func (n *JobStep) GetStatus() fsm.Status {
	return n.Status
}

func (n *JobStep) SetStatus(status fsm.Status) {
	n.Status = status
	n.job.SetStepStatus(n.StepName, n.Status)
}

func (n *JobStep) GetMessage() string {
	return n.Message
}

func (n *JobStep) SetMessage(msg string) {
	n.Message = msg
}

func (n *JobStep) Name() flow.TName {
	return n.StepName
}

func (n *JobStep) Setup(ctx *flow.Context) error {
	ctx.Succeed()
	return nil
}

func (n *JobStep) Do(ctx *flow.Context) error {
	req := common.NewRequest()
	_, err := plugin.Call(ctx.Context, n.Plugin, req)
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
