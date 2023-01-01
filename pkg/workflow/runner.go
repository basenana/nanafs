package workflow

import (
	"context"
	"errors"
	"fmt"
	goflowctrl "github.com/basenana/go-flow/controller"
	"github.com/basenana/go-flow/flow"
	"github.com/basenana/go-flow/fsm"
	"github.com/basenana/go-flow/storage"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/google/uuid"
	"github.com/hyponet/eventbus/bus"
	"go.uber.org/zap"
	"sync"
)

var (
	ErrJobNotFound = errors.New("job not found")
)

type WfRequest struct {
	context.Context
	target *types.Object
}

type Workflow struct {
	*types.Object

	req    *WfRequest
	spec   types.WorkflowSpec
	logger *zap.SugaredLogger
}

func (w *Workflow) PrepareJob() (*Job, error) {
	jid := uuid.New().String()

	steps, err := w.initJobSteps()
	if err != nil {
		return nil, err
	}

	return &Job{
		Id:       jid,
		workflow: w,
		status:   flow.CreatingStatus,
		steps:    steps,
		logger:   logger.NewLogger(fmt.Sprintf("job.%s", jid)),
	}, nil
}

func (w *Workflow) initJobSteps() ([]*JobStep, error) {
	return nil, nil
}

func InitWorkflow(wf *types.Object, spec types.WorkflowSpec, req *WfRequest) *Workflow {
	return &Workflow{
		Object: wf,

		req:    req,
		spec:   spec,
		logger: logger.NewLogger(fmt.Sprintf("workflow.%s", spec.Name)),
	}
}

type Runner struct {
	sync.RWMutex
	ctrl   controller.Controller
	stopCh chan struct{}

	jobParent *types.Object
	jobs      map[flow.FID]*Job

	logger *zap.SugaredLogger
}

var _ storage.Interface = &Runner{}

func InitWorkflowRunner(ctrl controller.Controller, stopCh chan struct{}) error {
	runner := &Runner{
		ctrl:   ctrl,
		stopCh: stopCh,
		jobs:   map[flow.FID]*Job{},
		logger: logger.NewLogger("workflowRuntime"),
	}

	var err error
	flowCtrl, err = goflowctrl.NewFlowController(goflowctrl.Option{Storage: runner})
	if err != nil {
		return err
	}
	if err = flowCtrl.Register(&Job{}); err != nil {
		return err
	}

	return runner.Init()
}

func (r *Runner) Init() error {
	_, err := bus.Subscribe("object.workflow.*.trigger", r.WorkFlowHandler)
	if err != nil {
		return err
	}
	return nil
}

func (r *Runner) WorkFlowHandler(wf *Workflow) {
	r.logger.Infow("receive workflow", "workflow", wf.Name)

	job, err := wf.PrepareJob()
	if err != nil {
		r.logger.Errorw("init job failed", "workflow", wf.Name, "err", err)
		return
	}

	go r.triggerJob(context.TODO(), job)

	return
}

func (r *Runner) triggerJob(ctx context.Context, job *Job) {
	r.Lock()
	r.jobs[job.ID()] = job
	r.Unlock()

	if err := flowCtrl.TriggerFlow(ctx, job.ID()); err != nil {
		r.logger.Errorw("trigger job flow failed", "job", job.ID(), "err", err)
	}
	return
}

func (r *Runner) GetFlow(flowId flow.FID) (flow.Flow, error) {
	r.RLock()
	job, ok := r.jobs[flowId]
	r.RUnlock()
	if !ok {
		return nil, ErrJobNotFound
	}
	return job, nil
}

func (r *Runner) GetFlowMeta(flowId flow.FID) (*storage.FlowMeta, error) {
	r.RLock()
	job, ok := r.jobs[flowId]
	r.RUnlock()
	if !ok {
		return nil, ErrJobNotFound
	}
	result := &storage.FlowMeta{
		Type:       job.Type(),
		Id:         job.ID(),
		Status:     job.GetStatus(),
		TaskStatus: map[flow.TName]fsm.Status{},
	}
	for _, step := range job.steps {
		result.TaskStatus[step.name] = step.status
	}
	return result, nil
}

func (r *Runner) SaveFlow(flow flow.Flow) error {
	job, ok := flow.(*Job)
	if !ok {
		return fmt.Errorf("flow %s not a Job object", flow.ID())
	}
	r.Lock()
	r.jobs[flow.ID()] = job
	r.Unlock()
	return nil
}

func (r *Runner) DeleteFlow(flowId flow.FID) error {
	r.Lock()
	delete(r.jobs, flowId)
	r.Unlock()
	return nil
}

func (r *Runner) SaveTask(flowId flow.FID, task flow.Task) error {
	r.Lock()
	job, ok := r.jobs[flowId]
	if !ok {
		r.Unlock()
		return ErrJobNotFound
	}

	newStep, ok := task.(*JobStep)
	if !ok {
		return fmt.Errorf("task not a JobStep object")
	}

	for i, step := range job.steps {
		if step.Name() == task.Name() {
			job.steps[i] = newStep
			break
		}
	}
	r.Unlock()
	return nil
}

func (r *Runner) DeleteTask(flowId flow.FID, taskName flow.TName) error {
	return nil
}
