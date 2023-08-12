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
	"github.com/basenana/go-flow/exec"
	"github.com/basenana/go-flow/flow"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"strconv"
	"time"
)

func assembleWorkflowJob(ctx context.Context, mgr dentry.Manager, spec *types.WorkflowSpec, entry *types.Metadata, fuseCfg config.FUSE) (*types.WorkflowJob, error) {
	var globalParam = map[string]string{}
	j := &types.WorkflowJob{
		Id:        uuid.New().String(),
		Workflow:  spec.Id,
		Target:    types.WorkflowTarget{EntryID: entry.ID},
		Status:    flow.InitializingStatus,
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
			StepName: opEntryInit,
			Status:   flow.InitializingStatus,
			Operator: &types.WorkflowJobOperator{
				Name:       opEntryInit,
				Parameters: globalParam,
			},
		})
	}

	globalParam[paramEntryIdKey] = strconv.FormatInt(entry.ID, 10)
	globalParam[paramEntryPathKey] = entryPath
	for _, stepSpec := range spec.Steps {
		if stepSpec.Plugin != nil {
			for k, v := range globalParam {
				stepSpec.Plugin.Parameters[k] = v
			}
		}
		j.Steps = append(j.Steps,
			types.WorkflowJobStep{
				StepName: stepSpec.Name,
				Status:   flow.InitializingStatus,
				Plugin:   stepSpec.Plugin,
				Script:   stepSpec.Script,
			},
		)
	}

	j.Steps = append(j.Steps, types.WorkflowJobStep{
		StepName: opEntryCollect,
		Status:   flow.InitializingStatus,
		Operator: &types.WorkflowJobOperator{
			Name:       opEntryCollect,
			Parameters: globalParam,
		},
	})

	return j, nil
}

func assembleFlow(job *types.WorkflowJob) (*flow.Flow, error) {
	f := &flow.Flow{
		ID:            job.Id,
		Executor:      "local",
		Status:        job.Status,
		Message:       job.Message,
		ControlPolicy: flow.ControlPolicy{FailedPolicy: flow.PolicyFastFailed},
	}

	for _, step := range job.Steps {
		var t flow.Task
		switch {
		case step.Plugin != nil:
			param := map[string]string{
				paramPluginName:    step.Plugin.PluginName,
				paramPluginVersion: step.Plugin.Version,
				paramPluginType:    string(step.Plugin.PluginType),
				paramPluginAction:  step.Plugin.Action,
			}
			for k, v := range step.Plugin.Parameters {
				if _, ok := param[k]; ok {
					continue
				}
				param[k] = v
			}
			t = flow.Task{
				Name:    step.StepName,
				Status:  step.Status,
				Message: step.Message,
				OperatorSpec: flow.Spec{
					Type:       opPluginCall,
					Parameters: param,
				},
				RetryOnFailed: 1,
			}
		case step.Operator != nil:
			t = flow.Task{
				Name: step.Operator.Name,
				OperatorSpec: flow.Spec{
					Type:       step.Operator.Name,
					Parameters: step.Operator.Parameters,
				},
				RetryOnFailed: 1,
			}
		case step.Script != nil:
			if step.Script.Type == exec.ShellOperator || step.Script.Type == exec.PythonOperator {
				t = flow.Task{
					Name: step.Operator.Name,
					OperatorSpec: flow.Spec{
						Type: step.Script.Type,
						Script: &flow.Script{
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

type storageWrapper struct {
	recorder metastore.ScheduledTaskRecorder
	logger   *zap.SugaredLogger
}

var _ flow.Storage = &storageWrapper{}

func (s *storageWrapper) GetFlow(ctx context.Context, flowId string) (*flow.Flow, error) {
	wfJob, err := s.recorder.ListWorkflowJob(ctx, types.JobFilter{JobID: flowId})
	if err != nil {
		s.logger.Errorw("load job failed", "err", err)
		return nil, err
	}
	if len(wfJob) == 0 {
		return nil, types.ErrNotFound
	}
	wf := wfJob[0]

	return assembleFlow(wf)
}

func (s *storageWrapper) SaveFlow(ctx context.Context, flow *flow.Flow) error {
	wfJob, err := s.recorder.ListWorkflowJob(ctx, types.JobFilter{JobID: flow.ID})
	if err != nil {
		return fmt.Errorf("query workflow job failed: %s", err)
	}
	if len(wfJob) == 0 {
		return fmt.Errorf("query workflow job failed: %s not found", flow.ID)
	}
	wf := wfJob[0]

	wf.Status = flow.Status
	wf.Message = flow.Message
	for i, step := range wf.Steps {
		for _, task := range flow.Tasks {
			if step.StepName == task.Name {
				wf.Steps[i].Status = task.Status
				wf.Steps[i].Message = task.Message
				break
			}
		}
	}
	wf.UpdatedAt = time.Now()

	err = s.recorder.SaveWorkflowJob(ctx, wf)
	if err != nil {
		s.logger.Errorw("save job to metadb failed", "err", err)
		return err
	}

	return nil
}

func (s *storageWrapper) DeleteFlow(ctx context.Context, flowId string) error {
	err := s.recorder.DeleteWorkflowJob(ctx, flowId)
	if err != nil {
		s.logger.Errorw("delete job to metadb failed", "err", err)
		return err
	}
	return nil
}

func (s *storageWrapper) SaveTask(ctx context.Context, flowId string, task *flow.Task) error {
	flowJob, err := s.GetFlow(ctx, flowId)
	if err != nil {
		return err
	}

	for i, t := range flowJob.Tasks {
		if t.Name == task.Name {
			flowJob.Tasks[i] = *task
			break
		}
	}

	return s.SaveFlow(ctx, flowJob)
}

type JobAttr struct {
	JobID  string
	Reason string
}
