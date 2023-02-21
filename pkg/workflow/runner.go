package workflow

import (
	"context"
	"errors"
	"fmt"
	flowcontroller "github.com/basenana/go-flow/controller"
	goflowctrl "github.com/basenana/go-flow/controller"
	"github.com/basenana/go-flow/flow"
	"github.com/basenana/go-flow/fsm"
	flowstorage "github.com/basenana/go-flow/storage"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/hyponet/eventbus/bus"
	"go.uber.org/zap"
	"sync"
)

var (
	ErrJobNotFound = errors.New("job not found")
)

type Runner struct {
	*flowcontroller.FlowController

	stopCh   chan struct{}
	recorder storage.PluginRecorder
	logger   *zap.SugaredLogger

	sync.RWMutex
}

func InitWorkflowRunner() (*Runner, error) {
	runner := &Runner{
		logger: logger.NewLogger("workflowRuntime"),
	}

	var err error
	flowCtrl, err := goflowctrl.NewFlowController(goflowctrl.Option{Storage: runner})
	if err != nil {
		return nil, err
	}
	if err = flowCtrl.Register(&Job{}); err != nil {
		return nil, err
	}
	runner.FlowController = flowCtrl

	return runner, nil
}

func (r *Runner) Init() error {
	_, err := bus.Subscribe("object.workflow.*.trigger", r.WorkFlowHandler)
	if err != nil {
		return err
	}
	return nil
}

func (r *Runner) Start(stopCh chan struct{}) error {
	return nil
}

func (r *Runner) WorkFlowHandler(wf *types.WorkflowSpec) {
	r.logger.Infow("receive workflow", "workflow", wf.Name)

	job, err := assembleWorkflowJob(wf)
	if err != nil {
		r.logger.Errorw("assemble job failed", "workflow", wf.Name, "err", err)
		return
	}

	go r.triggerJob(context.TODO(), job)

	return
}

func (r *Runner) triggerJob(ctx context.Context, job *Job) {
	if err := r.SaveFlow(job); err != nil {
		r.logger.Errorw("save job failed", "err", err)
		return
	}

	if err := r.TriggerFlow(ctx, job.ID()); err != nil {
		r.logger.Errorw("trigger job flow failed", "job", job.ID(), "err", err)
	}
	return
}

func (r *Runner) GetFlow(flowId flow.FID) (flow.Flow, error) {
	job := &Job{}
	err := r.recorder.GetRecord(context.Background(), string(flowId), job)
	if err != nil {
		r.logger.Errorw("load job failed", "err", err)
		return nil, err
	}

	return job, nil
}

func (r *Runner) GetFlowMeta(flowId flow.FID) (*flowstorage.FlowMeta, error) {
	flowJob, err := r.GetFlow(flowId)
	if err != nil {
		return nil, err
	}

	job, ok := flowJob.(*Job)
	if !ok {
		return nil, ErrJobNotFound
	}

	result := &flowstorage.FlowMeta{
		Type:       job.Type(),
		Id:         job.ID(),
		Status:     job.GetStatus(),
		TaskStatus: map[flow.TName]fsm.Status{},
	}
	for _, step := range job.Steps {
		result.TaskStatus[step.StepName] = step.Status
	}
	return result, nil
}

func (r *Runner) SaveFlow(flow flow.Flow) error {
	job, ok := flow.(*Job)
	if !ok {
		return fmt.Errorf("flow %s not a Job object", flow.ID())
	}

	err := r.recorder.SaveRecord(context.Background(), job.Workflow, job.Id, job)
	if err != nil {
		r.logger.Errorw("save job to metadb failed", "err", err)
		return err
	}

	return nil
}

func (r *Runner) DeleteFlow(flowId flow.FID) error {
	err := r.recorder.DeleteRecord(context.Background(), string(flowId))
	if err != nil {
		r.logger.Errorw("delete job to metadb failed", "err", err)
		return err
	}
	return nil
}

func (r *Runner) SaveTask(flowId flow.FID, task flow.Task) error {
	r.Lock()
	defer r.Unlock()

	flowJob, err := r.GetFlow(flowId)
	if err != nil {
		return err
	}

	job, ok := flowJob.(*Job)
	if !ok {
		return ErrJobNotFound
	}

	newStep, ok := task.(*JobStep)
	if !ok {
		return fmt.Errorf("task not a JobStep object")
	}

	for i, step := range job.Steps {
		if step.Name() == task.Name() {
			job.Steps[i] = newStep
			break
		}
	}

	return r.SaveFlow(job)
}

func (r *Runner) DeleteTask(flowId flow.FID, taskName flow.TName) error {
	return nil
}
