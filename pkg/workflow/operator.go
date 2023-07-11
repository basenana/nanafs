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
	"strconv"
)

const (
	opEntryInit    = "entryInit"
	opEntryCollect = "entryCollect"
	opPluginCall   = "pluginCall"
)

func registerOperators(entryMgr dentry.Manager) error {
	b := operatorBuilder{
		entryMgr: entryMgr,
	}
	if err := exec.RegisterLocalOperatorBuilder(opEntryInit, b.buildEntryInitOperator); err != nil {
		return err
	}
	if err := exec.RegisterLocalOperatorBuilder(opEntryCollect, b.buildEntryCollectOperator); err != nil {
		return err
	}
	if err := exec.RegisterLocalOperatorBuilder(opPluginCall, b.buildPluginCallOperator); err != nil {
		return err
	}
	return nil
}

type operatorBuilder struct {
	entryMgr dentry.Manager
}

func (b *operatorBuilder) buildEntryInitOperator(operatorSpec flow.Spec) (flow.Operator, error) {
	op := &entryInitOperator{
		entryMgr:  b.entryMgr,
		entryPath: operatorSpec.Parameter[paramEntryPathKey],
	}

	entryIDStr := operatorSpec.Parameter[paramEntryIdKey]
	if entryIDStr != "" {
		entryID, err := strconv.ParseInt(entryIDStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse entry id failed: %s", err)
		}
		op.entryID = entryID
	}
	return op, nil
}

func (b *operatorBuilder) buildEntryCollectOperator(operatorSpec flow.Spec) (flow.Operator, error) {
	return &entryCollectOperator{}, nil
}

func (b *operatorBuilder) buildPluginCallOperator(operatorSpec flow.Spec) (flow.Operator, error) {
	return &pluginCallOperator{
		plugin: types.PlugScope{
			PluginName: operatorSpec.Parameter[paramPluginName],
			Version:    operatorSpec.Parameter[paramPluginVersion],
			PluginType: types.PluginType(operatorSpec.Parameter[paramPluginType]),
			Parameters: operatorSpec.Parameter,
		},
		entryPath: "",
	}, nil
}

const (
	paramEntryIdKey    = "nanafs.internal.entry_id"
	paramEntryPathKey  = "nanafs.internal.entry_path"
	paramPluginName    = "nanafs.internal.plugin_name"
	paramPluginVersion = "nanafs.internal.plugin_version"
	paramPluginType    = "nanafs.internal.plugin_type"
)

type entryInitOperator struct {
	entryID   int64
	entryMgr  dentry.Manager
	entryPath string
}

func (e *entryInitOperator) Do(ctx context.Context, param flow.Parameter) error {
	entry, err := e.entryMgr.GetEntry(ctx, e.entryID)
	if err != nil {
		return fmt.Errorf("load entry failed: %s", err)
	}
	f, err := e.entryMgr.Open(ctx, entry, dentry.Attr{Read: true})
	if err != nil {
		return fmt.Errorf("open entry failed: %s", err)
	}

	defer f.Close(ctx)
	if _, err = copyEntryToJobWorkDir(ctx, param.Workdir, e.entryPath, f); err != nil {
		return fmt.Errorf("copy entry file failed: %s", err)
	}
	return nil
}

type entryCollectOperator struct{}

func (e *entryCollectOperator) Do(ctx context.Context, param flow.Parameter) error {
	return nil
}

type pluginCallOperator struct {
	plugin    types.PlugScope
	entryPath string
}

func (e *pluginCallOperator) Do(ctx context.Context, param flow.Parameter) error {
	req := stub.NewRequest()
	req.WorkPath = param.Workdir
	req.EntryPath = e.entryPath
	_, err := pluginCall(ctx, e.plugin, req)
	return err
}

var pluginCall = plugin.Call
