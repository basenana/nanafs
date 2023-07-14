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
	MirrorPluginName          = "workflow"
	MirrorPluginVersion       = "1.0"
	MirrorDirWorkflows        = "workflows"
	MirrorDirJobs             = "jobs"
	MirrorFileType            = ".yaml"
	MirrorFileMaxSize   int64 = 1 << 22 // 4M
)

var mirrorPlugin = types.PluginSpec{
	Name:       MirrorPluginName,
	Version:    MirrorPluginVersion,
	Type:       types.TypeMirror,
	Parameters: map[string]string{},
}

type MirrorPlugin struct {
	*dirHandler
	*fileHandler
}

var _ plugin.MirrorPlugin = &MirrorPlugin{}

func (m *MirrorPlugin) Name() string {
	return MirrorPluginName
}

func (m *MirrorPlugin) Type() types.PluginType {
	return types.TypeMirror
}

func (m *MirrorPlugin) Version() string {
	return MirrorPluginVersion
}

func (m *MirrorPlugin) build(ctx context.Context, _ types.PluginSpec, scope types.PlugScope) (plugin.Plugin, error) {
	return nil, nil
}

type dirHandler struct {
	mgr     Manager
	dirKind string
	path    string
}

func (d *dirHandler) IsGroup(ctx context.Context) (bool, error) {
	if d == nil {
		return false, nil
	}
	return true, nil
}

func (d *dirHandler) FindEntry(ctx context.Context, name string) (*stub.Entry, error) {
	if d == nil {
		return nil, types.ErrNoGroup
	}
	return nil, nil
}

func (d *dirHandler) CreateEntry(ctx context.Context, attr stub.EntryAttr) (*stub.Entry, error) {
	if d == nil {
		return nil, types.ErrNoGroup
	}
	return nil, nil
}

func (d *dirHandler) UpdateEntry(ctx context.Context, en *stub.Entry) error {
	if d == nil {
		return types.ErrNoGroup
	}
	return nil
}

func (d *dirHandler) RemoveEntry(ctx context.Context, en *stub.Entry) error {
	if d == nil {
		return types.ErrNoGroup
	}
	return nil
}

func (d *dirHandler) ListChildren(ctx context.Context) ([]*stub.Entry, error) {
	if d == nil {
		return nil, types.ErrNoGroup
	}
	return nil, nil
}

type fileHandler struct {
	mgr     Manager
	data    []byte
	dirKind string
	path    string
	err     error
}

func (f *fileHandler) WriteAt(ctx context.Context, data []byte, off int64) (int64, error) {
	if f == nil {
		return 0, types.ErrIsGroup
	}
	return 0, nil
}

func (f *fileHandler) ReadAt(ctx context.Context, dest []byte, off int64) (int64, error) {
	if f == nil {
		return 0, types.ErrIsGroup
	}
	return 0, nil
}

func (f *fileHandler) Fsync(ctx context.Context) error {
	if f == nil {
		return types.ErrIsGroup
	}
	return nil
}

func (f *fileHandler) Flush(ctx context.Context) error {
	if f == nil {
		return types.ErrIsGroup
	}
	return nil
}

func (f *fileHandler) Close(ctx context.Context) error {
	if f == nil {
		return types.ErrIsGroup
	}
	return nil
}

func buildWorkflowMirrorPlugin(mgr Manager) plugin.Builder {
	mp := &MirrorPlugin{dirHandler: &dirHandler{mgr: mgr, path: "/"}}
	return mp.build
}
