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
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/plugin/stub"
	"github.com/basenana/nanafs/pkg/types"
)

const (
	MirrorPluginName    = "workflow"
	MirrorPluginVersion = "1.0"
)

type MirrorPlugin struct {
	Manager
}

func (m MirrorPlugin) Name() string {
	//TODO implement me
	panic("implement me")
}

func (m MirrorPlugin) Type() types.PluginType {
	//TODO implement me
	panic("implement me")
}

func (m MirrorPlugin) Version() string {
	//TODO implement me
	panic("implement me")
}

func (m MirrorPlugin) IsGroup(ctx context.Context) (bool, error) {
	//TODO implement me
	panic("implement me")
}

func (m MirrorPlugin) FindEntry(ctx context.Context, name string) (stub.Entry, error) {
	//TODO implement me
	panic("implement me")
}

func (m MirrorPlugin) CreateEntry(ctx context.Context, attr stub.EntryAttr) (stub.Entry, error) {
	//TODO implement me
	panic("implement me")
}

func (m MirrorPlugin) UpdateEntry(ctx context.Context, en stub.Entry) error {
	//TODO implement me
	panic("implement me")
}

func (m MirrorPlugin) RemoveEntry(ctx context.Context, en stub.Entry) error {
	//TODO implement me
	panic("implement me")
}

func (m MirrorPlugin) ListChildren(ctx context.Context) ([]stub.Entry, error) {
	//TODO implement me
	panic("implement me")
}

func (m MirrorPlugin) WriteAt(ctx context.Context, data []byte, off int64) (int64, error) {
	//TODO implement me
	panic("implement me")
}

func (m MirrorPlugin) ReadAt(ctx context.Context, dest []byte, off int64) (int64, error) {
	//TODO implement me
	panic("implement me")
}

func (m MirrorPlugin) Fsync(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (m MirrorPlugin) Flush(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (m MirrorPlugin) Close(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

var mirrorPlugin = types.PluginSpec{
	Name:       MirrorPluginName,
	Version:    MirrorPluginVersion,
	Type:       types.TypeMirror,
	Parameters: map[string]string{},
}

func buildWorkflowMirrorPlugin(mgr Manager) plugin.Builder {
	return func(ctx context.Context, spec types.PluginSpec, scope types.PlugScope) (plugin.Plugin, error) {
		return &MirrorPlugin{Manager: mgr}, nil
	}
}
