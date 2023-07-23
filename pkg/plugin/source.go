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
	"github.com/basenana/nanafs/pkg/plugin/stub"
	"github.com/basenana/nanafs/pkg/types"
	"io"
)

type SourcePlugin interface {
	Plugin
	Fresh(ctx context.Context, opt stub.FreshOption) ([]*stub.Entry, error)
	Open(ctx context.Context, entry *stub.Entry) (io.ReadCloser, error)
}

type DummySourcePlugin struct{}

var _ SourcePlugin = &DummySourcePlugin{}

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
