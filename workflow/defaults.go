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
	"fmt"
	"github.com/basenana/nanafs/pkg/types"
)

func NewDefaultsWorkflows(namespace string) []*types.Workflow {
	return []*types.Workflow{
		{
			Id:        fmt.Sprintf("defaults-%s-rss", namespace),
			Namespace: namespace,
			Name:      "RSS",
			Trigger: types.WorkflowTrigger{
				RSS:      &types.WorkflowRssTrigger{},
				Interval: ptr(6),
			},
			Nodes: []types.WorkflowNode{
				{
					Name:       "rss-collect",
					Type:       "rss",
					Parameters: map[string]string{},
					Next:       "",
				},
			},
			Enable:    true,
			QueueName: types.WorkflowQueueFile,
		},
	}
}

func ptr[T any](v T) *T {
	return &v
}
