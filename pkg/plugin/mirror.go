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

package plugin

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/plugin/stub"
	"github.com/basenana/nanafs/pkg/types"
)

type MirrorPlugin interface {
	Plugin

	IsGroup(ctx context.Context) (bool, error)
	FindEntry(ctx context.Context, name string) (*stub.Entry, error)
	CreateEntry(ctx context.Context, attr stub.EntryAttr) (*stub.Entry, error)
	UpdateEntry(ctx context.Context, en *stub.Entry) error
	RemoveEntry(ctx context.Context, en *stub.Entry) error
	ListChildren(ctx context.Context) ([]*stub.Entry, error)

	WriteAt(ctx context.Context, data []byte, off int64) (int64, error)
	ReadAt(ctx context.Context, dest []byte, off int64) (int64, error)
	Fsync(ctx context.Context) error
	Flush(ctx context.Context) error
	Close(ctx context.Context) error
}

func NewMirrorPlugin(ctx context.Context, ps types.PlugScope) (MirrorPlugin, error) {
	plugin, err := BuildPlugin(ctx, ps)
	if err != nil {
		return nil, err
	}

	mirrorPlugin, ok := plugin.(MirrorPlugin)
	if !ok {
		return nil, fmt.Errorf("not mirror plugin")
	}
	return mirrorPlugin, nil
}

type DummyMirrorPlugin struct{}

var _ MirrorPlugin = &DummyMirrorPlugin{}

func (d *DummyMirrorPlugin) Name() string {
	return "dummy-mirror-plugin"
}

func (d *DummyMirrorPlugin) Type() types.PluginType {
	return types.TypeMirror
}

func (d *DummyMirrorPlugin) Version() string {
	return "1.0"
}

func (d *DummyMirrorPlugin) IsGroup(ctx context.Context) (bool, error) {
	//TODO implement me
	panic("implement me")
}

func (d *DummyMirrorPlugin) FindEntry(ctx context.Context, name string) (*stub.Entry, error) {
	//TODO implement me
	panic("implement me")
}

func (d *DummyMirrorPlugin) CreateEntry(ctx context.Context, attr stub.EntryAttr) (*stub.Entry, error) {
	//TODO implement me
	panic("implement me")
}

func (d *DummyMirrorPlugin) UpdateEntry(ctx context.Context, en *stub.Entry) error {
	//TODO implement me
	panic("implement me")
}

func (d *DummyMirrorPlugin) RemoveEntry(ctx context.Context, en *stub.Entry) error {
	//TODO implement me
	panic("implement me")
}

func (d *DummyMirrorPlugin) ListChildren(ctx context.Context) ([]*stub.Entry, error) {
	//TODO implement me
	panic("implement me")
}

func (d *DummyMirrorPlugin) WriteAt(ctx context.Context, data []byte, off int64) (int64, error) {
	//TODO implement me
	panic("implement me")
}

func (d *DummyMirrorPlugin) ReadAt(ctx context.Context, dest []byte, off int64) (int64, error) {
	//TODO implement me
	panic("implement me")
}

func (d *DummyMirrorPlugin) Fsync(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (d *DummyMirrorPlugin) Flush(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (d *DummyMirrorPlugin) Close(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}
