/*
 Copyright 2023 NanaFS Authors.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

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
