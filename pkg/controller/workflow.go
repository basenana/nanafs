package controller

import (
	"context"
	"github.com/basenana/go-flow/fsm"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/pkg/workflow"
)

const (
	WFNameFmt  = "workflow-%s"
	WFVersion  = "v1"
	JobNameFmt = "job-%s"
	JobVersion = "v1"
)

type WorkflowController interface {
	//GetWorkflows(ctx context.Context) []*workflow.Workflow
	//SaveWorkflows(ctx context.Context, name string, rule types.Rule, plugins []plugin.Plugin) *workflow.Workflow
	//DeleteWorkFlow(ctx context.Context, id string)
	Trigger(ctx context.Context, o *types.Object)
	GetJobs(ctx context.Context) map[fsm.Status]*workflow.Job
}

//func (c *controller) GetWorkflows(ctx context.Context) []*workflow.Workflow {
//	objects, err := c.listStructuredObject(ctx, types.WorkflowKind, WFVersion)
//	if err != nil {
//		return nil
//	}
//	ws := make([]*workflow.Workflow, len(objects))
//	for _, o := range objects {
//		w := workflow.Workflow{}
//		err = c.meta.LoadContent(ctx, o, types.WorkflowKind, WFVersion, &w)
//		if err != nil {
//			return nil
//		}
//		ws = append(ws, &w)
//	}
//	return ws
//}
//
//func (c *controller) SaveWorkflows(ctx context.Context, name string, rule types.Rule, plugins []plugin.Plugin) *workflow.Workflow {
//	workflowName := fmt.Sprintf(WFNameFmt, name)
//	w := workflow.NewWorkflow(workflowName, rule, plugins)
//	o := types.Object{
//		Metadata: types.NewMetadata(workflowName, types.WorkflowKind),
//	}
//	o.ParentID = c.workflowGroup.ID
//	w.ObjectId = o.ID
//	err := c.meta.SaveContent(ctx, &o, types.WorkflowKind, WFVersion, w)
//	if err != nil {
//		return nil
//	}
//	return w
//}
//
//func (c *controller) DeleteWorkFlow(ctx context.Context, id string) {
//	o, err := c.meta.GetObject(ctx, id)
//	if err != nil {
//		return
//	}
//	err = c.meta.DeleteContent(ctx, o, types.WorkflowKind, WFVersion)
//	if err != nil {
//		return
//	}
//}

func (c *controller) Trigger(ctx context.Context, value *types.Object) {
	objects, err := c.ListStructuredObject(ctx, types.WorkflowKind, WFVersion)
	if err != nil {
		return
	}
	for _, o := range objects {
		w := workflow.Workflow{}
		err = c.meta.LoadContent(ctx, o, types.WorkflowKind, WFVersion, &w)
		if err != nil {
			return
		}
		if !w.Rule.Apply(value) {
			continue
		}
		job := workflow.NewJob(&w, value)
		jObj := types.Object{Metadata: types.NewMetadata(job.Id, types.JobKind)}
		jObj.ID = o.ID
		err = c.meta.SaveContent(ctx, &jObj, types.JobKind, JobVersion, job)
		if err != nil {
			return
		}
		go job.Run()
	}
}

func (c *controller) GetJobs(ctx context.Context) map[fsm.Status]*workflow.Job {
	objects, err := c.ListStructuredObject(ctx, types.JobKind, JobVersion)
	if err != nil {
		return nil
	}
	ws := make(map[fsm.Status]*workflow.Job)
	for _, o := range objects {
		w := workflow.Job{}
		err = c.meta.LoadContent(ctx, o, types.JobKind, JobVersion, &w)
		if err != nil {
			return nil
		}
		ws[w.Flow.GetStatus()] = &w
	}
	return ws
}

func init() {
	dentry.Registry.Register(types.WorkflowKind, WFVersion, workflow.Workflow{})
	dentry.Registry.Register(types.JobKind, JobVersion, workflow.Job{})
}