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

package buildin

import (
	"context"
	"github.com/basenana/nanafs/pkg/plugin/stub"
	"github.com/basenana/nanafs/pkg/types"
	"io"
	"time"
)

type DummySourcePlugin struct{}

func (d *DummySourcePlugin) Name() string {
	return "dummy-source-plugin"
}

func (d *DummySourcePlugin) Type() types.PluginType {
	return types.TypeSource
}

func (d *DummySourcePlugin) Version() string {
	return "1.0"
}

func (d *DummySourcePlugin) Fresh(ctx context.Context, opt stub.FreshOption) ([]*stub.Entry, error) {
	//TODO implement me
	panic("implement me")
}

func (d *DummySourcePlugin) Open(ctx context.Context, entry *stub.Entry) (io.ReadCloser, error) {
	//TODO implement me
	panic("implement me")
}

type DummyProcessPlugin struct{}

func (d *DummyProcessPlugin) Name() string {
	return "dummy-process-plugin"
}

func (d *DummyProcessPlugin) Type() types.PluginType {
	return types.TypeProcess
}

func (d *DummyProcessPlugin) Version() string {
	return "1.0"
}

func (d *DummyProcessPlugin) Run(ctx context.Context, request *stub.Request, params map[string]string) (*stub.Response, error) {
	time.Sleep(time.Second * 2)
	return &stub.Response{IsSucceed: true}, nil
}

type DummyMirrorPlugin struct{}

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
