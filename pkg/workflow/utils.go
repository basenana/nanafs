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
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/pkg/workflow/jobrun"
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

func assembleWorkflowJob(spec *types.WorkflowSpec, tgt types.WorkflowTarget) (*types.WorkflowJob, error) {
	var globalParam = map[string]string{}
	j := &types.WorkflowJob{
		Id:        uuid.New().String(),
		Workflow:  spec.Id,
		Target:    tgt,
		Status:    jobrun.InitializingStatus,
		Executor:  spec.Executor,
		QueueName: spec.QueueName,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	for _, stepSpec := range spec.Steps {
		if stepSpec.Plugin != nil {
			for k, v := range globalParam {
				stepSpec.Plugin.Parameters[k] = v
			}
		}
		j.Steps = append(j.Steps,
			types.WorkflowJobStep{
				StepName: stepSpec.Name,
				Status:   jobrun.InitializingStatus,
				Plugin:   stepSpec.Plugin,
			},
		)
	}

	return j, nil
}

func initWorkflowJobRootWorkdir(wfCfg *config.Workflow) error {
	if wfCfg.JobWorkdir == "" {
		switch runtime.GOOS {
		case "linux":
			wfCfg.JobWorkdir = defaultLinuxWorkdir
		default:
			wfCfg.JobWorkdir = os.TempDir()
		}
	}
	wfLogger.Infof("job root workdir: %s", wfCfg.JobWorkdir)
	return os.MkdirAll(wfCfg.JobWorkdir, 0755)
}

func initWorkflow(wf *types.WorkflowSpec) *types.WorkflowSpec {
	if wf.Id == "" {
		wf.Id = uuid.New().String()
	}
	wf.CreatedAt = time.Now()
	wf.UpdatedAt = time.Now()
	return wf
}
