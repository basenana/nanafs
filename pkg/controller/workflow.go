package controller

import (
	"context"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
)

const (
	WFNameFmt  = "workflow-%s"
	WFVersion  = "v1"
	JobNameFmt = "job-%s"
	JobVersion = "v1"
)

type WorkflowController interface {
	GetJobs(ctx context.Context) map[string]*types.Job
}

func (c *controller) GetJobs(ctx context.Context) map[string]*types.Job {
	objects, err := c.ListStructuredObject(ctx, types.JobKind, JobVersion)
	if err != nil {
		return nil
	}
	ws := make(map[string]*types.Job)
	for _, o := range objects {
		w := types.Job{}
		err = c.meta.LoadContent(ctx, o, types.JobKind, JobVersion, &w)
		if err != nil {
			return nil
		}
		ws[w.Status] = &w
	}
	return ws
}

func init() {
	dentry.Registry.Register(types.WorkflowKind, WFVersion, types.Workflow{})
	dentry.Registry.Register(types.JobKind, JobVersion, types.Job{})
}
