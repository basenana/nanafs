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
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/pkg/workflow/exec"
	"github.com/basenana/nanafs/pkg/workflow/jobrun"
	"github.com/google/uuid"
	"strconv"
	"time"
)

func assembleWorkflowJob(ctx context.Context, mgr dentry.Manager, spec *types.WorkflowSpec, entry *types.Metadata, fuseCfg config.FUSE) (*types.WorkflowJob, error) {
	var globalParam = map[string]string{}
	j := &types.WorkflowJob{
		Id:        uuid.New().String(),
		Workflow:  spec.Id,
		Target:    types.WorkflowTarget{EntryID: entry.ID},
		Status:    jobrun.InitializingStatus,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// plugin info
	var pluginNames []string
	for _, stepSpec := range spec.Steps {
		if stepSpec.Plugin != nil {
			pluginNames = append(pluginNames, stepSpec.Plugin.PluginName)
		}
	}

	entryPath, err := entryID2FusePath(ctx, entry.ID, mgr, fuseCfg)
	if err != nil {
		return nil, err
	}
	if entryPath == "" && !plugin.IsBuildInPlugin(pluginNames...) {
		// if fuse disabled, using filename in workdir
		entryPath = entry.Name
		j.Steps = append(j.Steps, types.WorkflowJobStep{
			StepName: exec.OpEntryInit,
			Status:   jobrun.InitializingStatus,
			Operator: &types.WorkflowJobOperator{
				Name:       exec.OpEntryInit,
				Parameters: globalParam,
			},
		})
	}

	globalParam[exec.paramEntryIdKey] = strconv.FormatInt(entry.ID, 10)
	globalParam[exec.paramEntryPathKey] = entryPath
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
				Script:   stepSpec.Script,
			},
		)
	}

	j.Steps = append(j.Steps, types.WorkflowJobStep{
		StepName: exec.OpEntryCollect,
		Status:   jobrun.InitializingStatus,
		Operator: &types.WorkflowJobOperator{
			Name:       exec.OpEntryCollect,
			Parameters: globalParam,
		},
	})

	return j, nil
}

func assembleFlow(job *types.WorkflowJob) (*jobrun.Flow, error) {
	f := &jobrun.Flow{
		ID:            job.Id,
		Executor:      "local",
		Status:        job.Status,
		Message:       job.Message,
		ControlPolicy: jobrun.ControlPolicy{FailedPolicy: jobrun.PolicyFastFailed},
	}

	for _, step := range job.Steps {
		var t jobrun.Task
		switch {
		case step.Plugin != nil:
			param := map[string]string{
				exec.paramPluginName:    step.Plugin.PluginName,
				exec.paramPluginVersion: step.Plugin.Version,
				exec.paramPluginType:    string(step.Plugin.PluginType),
				exec.paramPluginAction:  step.Plugin.Action,
			}
			for k, v := range step.Plugin.Parameters {
				if _, ok := param[k]; ok {
					continue
				}
				param[k] = v
			}
			t = jobrun.Task{
				Name:    step.StepName,
				Status:  step.Status,
				Message: step.Message,
				OperatorSpec: jobrun.Spec{
					Type:       exec.OpPluginCall,
					Parameters: param,
				},
				RetryOnFailed: 1,
			}
		case step.Operator != nil:
			t = jobrun.Task{
				Name: step.Operator.Name,
				OperatorSpec: jobrun.Spec{
					Type:       step.Operator.Name,
					Parameters: step.Operator.Parameters,
				},
				RetryOnFailed: 1,
			}
		case step.Script != nil:
			if step.Script.Type == exec.ShellOperator || step.Script.Type == exec.PythonOperator {
				t = jobrun.Task{
					Name: step.Operator.Name,
					OperatorSpec: jobrun.Spec{
						Type: step.Script.Type,
						Script: &jobrun.Script{
							Content: step.Script.Content,
							Command: step.Script.Command,
							Env:     step.Script.Env,
						},
					},
					RetryOnFailed: 1,
				}
			} else {
				return nil, fmt.Errorf("step has unknown script type %s", step.Script.Type)
			}

		}
		f.Tasks = append(f.Tasks, t)
	}

	for i := 1; i < len(f.Tasks); i++ {
		f.Tasks[i-1].Next.OnSucceed = f.Tasks[i].Name
	}

	return f, nil
}

type JobAttr struct {
	JobID  string
	Reason string
}
