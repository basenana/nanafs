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
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/workflow/jobrun"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"os"
	"runtime"
	"time"
)

var (
	defaultLinuxWorkdir = "/var/lib/nanafs/workflow"
	wfLogger            *zap.SugaredLogger
)

const (
	defaultJobTimeout = time.Hour * 3
)

func assembleWorkflowJob(wf *types.Workflow, tgt types.WorkflowTarget, attr JobAttr) (*types.WorkflowJob, error) {
	j := &types.WorkflowJob{
		Id:             uuid.New().String(),
		Namespace:      wf.Namespace,
		Workflow:       wf.Id,
		TriggerReason:  attr.Reason,
		Targets:        tgt,
		Nodes:          make([]types.WorkflowJobNode, 0, len(wf.Nodes)),
		Parameters:     attr.Parameters,
		Status:         jobrun.InitializingStatus,
		Message:        "pending",
		QueueName:      attr.Queue,
		TimeoutSeconds: int(attr.Timeout.Seconds()),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	for i := range wf.Nodes {
		node := wf.Nodes[i]
		j.Nodes = append(j.Nodes,
			types.WorkflowJobNode{
				WorkflowNode: node,
				Status:       jobrun.InitializingStatus,
			},
		)
	}

	return j, nil
}

func initWorkflowJobRootWorkdir(jobWorkdir string) error {
	if jobWorkdir == "" {
		jobWorkdir = genDefaultJobRootWorkdir()
	}
	wfLogger.Infof("job root workdir: %s", jobWorkdir)
	return os.MkdirAll(jobWorkdir, 0755)
}

func genDefaultJobRootWorkdir() (jobWorkdir string) {
	switch runtime.GOOS {
	case "linux":
		jobWorkdir = defaultLinuxWorkdir
	default:
		jobWorkdir = os.TempDir()
	}
	return
}

func initWorkflow(namespace string, wf *types.Workflow) *types.Workflow {
	wf.Id = uuid.New().String()
	wf.Namespace = namespace
	wf.CreatedAt = time.Now()
	wf.UpdatedAt = time.Now()
	return wf
}
