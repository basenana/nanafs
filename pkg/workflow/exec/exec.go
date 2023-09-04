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

package exec

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/pkg/workflow/jobrun"
	"strconv"
)

const (
	localExecName = "local"
)

func RegisterOperators(entryMgr dentry.Manager) error {
	jobrun.RegisterExecutorBuilder(localExecName, func(job *types.WorkflowJob) jobrun.Executor {
		return &localExecutor{job: job, entryMgr: entryMgr}
	})
	return nil
}

type localExecutor struct {
	job      *types.WorkflowJob
	entryMgr dentry.Manager
}

func (b *localExecutor) Setup(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (b *localExecutor) DoOperation(ctx context.Context, step types.WorkflowJobStep, operatorSpec jobrun.Spec) error {
	//TODO implement me
	panic("implement me")
}

func (b *localExecutor) Teardown(ctx context.Context) {
	//TODO implement me
	panic("implement me")
}

var _ jobrun.Executor = &localExecutor{}

func (b *localExecutor) buildEntryInitOperator(step types.WorkflowJobStep, operatorSpec jobrun.Spec) (jobrun.Operator, error) {
	op := &entryInitOperator{
		entryMgr:  b.entryMgr,
		entryPath: operatorSpec.Parameters[paramEntryPathKey],
	}

	entryIDStr := operatorSpec.Parameters[paramEntryIdKey]
	entryID, err := strconv.ParseInt(entryIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse entry id failed: %s", err)
	}
	op.entryID = entryID
	return op, nil
}

func (b *localExecutor) buildEntryCollectOperator(step types.WorkflowJobStep, operatorSpec jobrun.Spec) (jobrun.Operator, error) {
	return &entryCollectOperator{}, nil
}

func (b *localExecutor) buildPluginCallOperator(step types.WorkflowJobStep, operatorSpec jobrun.Spec) (jobrun.Operator, error) {
	entryIDStr := operatorSpec.Parameters[paramEntryIdKey]
	entryID, err := strconv.ParseInt(entryIDStr, 10, 64)
	if err != nil {
		return nil, err
	}

	return &pluginCallOperator{
		plugin: types.PlugScope{
			PluginName: operatorSpec.Parameters[paramPluginName],
			Version:    operatorSpec.Parameters[paramPluginVersion],
			PluginType: types.PluginType(operatorSpec.Parameters[paramPluginType]),
			Action:     operatorSpec.Parameters[paramPluginAction],
			Parameters: operatorSpec.Parameters,
		},
		entryID:   entryID,
		entryPath: operatorSpec.Parameters[paramEntryPathKey],
	}, nil
}
