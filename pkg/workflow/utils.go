package workflow

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
)

func nextWorkflowId(ctx context.Context, recorder storage.PluginRecorder) (string, error) {
	count, err := resourceCount(ctx, dataGroupWorkflow, recorder)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("workflow-%d", count.Workflow), nil
}

func nextJobId(ctx context.Context, recorder storage.PluginRecorder) (string, error) {
	count, err := resourceCount(ctx, dataGroupJob, recorder)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("job-%d", count.Job), nil
}

func jobGroupId(wfId string) string {
	return fmt.Sprintf("%s-%s", wfId, dataGroupJob)
}

type CountData struct {
	Workflow int64 `json:"workflow"`
	Job      int64 `json:"job"`
}

func resourceCount(ctx context.Context, groupId string, recorder storage.PluginRecorder) (CountData, error) {
	data := CountData{}
	if err := recorder.GetRecord(ctx, "counter", &data); err != nil && err != types.ErrNotFound {
		return CountData{}, err
	}
	switch groupId {
	case dataGroupWorkflow:
		data.Workflow += 1
	case dataGroupJob:
		data.Job += 1
	}
	if err := recorder.SaveRecord(ctx, "counter", "counter", &data); err != nil {
		return CountData{}, err
	}
	return data, nil
}
