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
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/plugin/stub"
	"github.com/basenana/nanafs/pkg/types"
	"os"
	"path"
	"strconv"
	"strings"
)

const (
	opEntryInit    = "entryInit"
	opEntryCollect = "entryCollect"
	opPluginCall   = "pluginCall"
)

func registerOperators(entryMgr dentry.Manager) error {
	b := operatorBuilder{entryMgr: entryMgr}
	r := exec.NewLocalOperatorBuilderRegister()

	if err := r.Register(opEntryInit, b.buildEntryInitOperator); err != nil && err != exec.OperatorIsExisted {
		return err
	}
	if err := r.Register(opEntryCollect, b.buildEntryCollectOperator); err != nil && err != exec.OperatorIsExisted {
		return err
	}
	if err := r.Register(opPluginCall, b.buildPluginCallOperator); err != nil && err != exec.OperatorIsExisted {
		return err
	}

	flow.RegisterExecutorBuilder("local", func(flow *flow.Flow) flow.Executor {
		return exec.NewLocalExecutor(flow, r)
	})
	return nil
}

type operatorBuilder struct {
	entryMgr dentry.Manager
}

func (b *operatorBuilder) buildEntryInitOperator(task flow.Task, operatorSpec flow.Spec) (flow.Operator, error) {
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

func (b *operatorBuilder) buildEntryCollectOperator(task flow.Task, operatorSpec flow.Spec) (flow.Operator, error) {
	return &entryCollectOperator{}, nil
}

func (b *operatorBuilder) buildPluginCallOperator(task flow.Task, operatorSpec flow.Spec) (flow.Operator, error) {
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

const (
	paramEntryIdKey    = "nanafs.workflow.entry_id"
	paramEntryPathKey  = "nanafs.workflow.entry_path"
	paramPluginName    = "nanafs.workflow.plugin_name"
	paramPluginVersion = "nanafs.workflow.plugin_version"
	paramPluginType    = "nanafs.workflow.plugin_type"
	paramPluginAction  = "nanafs.workflow.plugin_action"
)

type entryInitOperator struct {
	entryID   int64
	entryMgr  dentry.Manager
	entryPath string
}

func (e *entryInitOperator) Do(ctx context.Context, param *flow.Parameter) error {
	entryPath := e.entryPath
	if !path.IsAbs(e.entryPath) {
		entryPath = path.Join(param.Workdir, path.Base(e.entryPath))
	}

	enInfo, err := os.Stat(entryPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if enInfo != nil {
		return nil
	}

	if !strings.HasPrefix(entryPath, param.Workdir) {
		wfLogger.Warnf("init entry unexpected, entryPath=%s, workdir=%s", entryPath, param.Workdir)
		return types.ErrNotFound
	}

	entry, err := e.entryMgr.GetEntry(ctx, e.entryID)
	if err != nil {
		return fmt.Errorf("load entry failed: %s", err)
	}
	f, err := e.entryMgr.Open(ctx, entry, dentry.Attr{Read: true})
	if err != nil {
		return fmt.Errorf("open entry failed: %s", err)
	}
	defer f.Close(ctx)

	if err = copyEntryToJobWorkDir(ctx, entryPath, f); err != nil {
		return fmt.Errorf("copy entry file failed: %s", err)
	}
	return nil
}

type entryCollectOperator struct{}

func (e *entryCollectOperator) Do(ctx context.Context, param *flow.Parameter) error {
	return nil
}

type pluginCallOperator struct {
	plugin    types.PlugScope
	entryID   int64
	entryPath string
}

func (e *pluginCallOperator) Do(ctx context.Context, param *flow.Parameter) error {
	req := stub.NewRequest()
	req.WorkPath = param.Workdir
	req.EntryId = e.entryID
	req.EntryPath = e.entryPath
	req.Action = e.plugin.Action
	req.Parameter = e.plugin.Parameters
	resp, err := plugin.Call(ctx, e.plugin, req)
	if err != nil {
		return fmt.Errorf("plugin action error: %s", err)
	}
	if !resp.IsSucceed {
		return fmt.Errorf("plugin action failed: %s", resp.Message)
	}
	return nil
}
