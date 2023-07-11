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
	"fmt"
	"github.com/basenana/nanafs/pkg/plugin/stub"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"io"
	"strings"
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

func (d *DummySourcePlugin) Run(ctx context.Context, request *stub.Request, params map[string]string) (*stub.Response, error) {
	resp := stub.NewResponse()
	resp.IsSucceed = true
	mockedEntries := make([]stub.Entry, 0)
	mockedEntries = append(mockedEntries, stub.NewFileEntry("dummy-file-1.json", []byte(`{"key": "value"}`)))
	mockedEntries = append(mockedEntries, stub.NewFileEntry("dummy-file-2.json", []byte(`{"key": "value"}`)))
	resp.Entries = mockedEntries
	return resp, nil
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

type DummyMirrorPlugin struct {
	dataSets *stub.GroupEntry
}

func (d *DummyMirrorPlugin) Name() string {
	return "dummy-mirror-plugin"
}

func (d *DummyMirrorPlugin) Type() types.PluginType {
	return types.TypeMirror
}

func (d *DummyMirrorPlugin) Version() string {
	return "1.0"
}

func (d *DummyMirrorPlugin) Run(ctx context.Context, request *stub.Request, params map[string]string) (*stub.Response, error) {
	switch request.CallType {
	case stub.CallListEntries:
		return d.handleList(ctx, request, params)
	case stub.CallAddEntry:
		return d.handleAdd(ctx, request, params)
	case stub.CallUpdateEntry:
		return d.handleUpdate(ctx, request, params)
	case stub.CallDeleteEntry:
		return d.handleDelete(ctx, request, params)
	default:
		return nil, fmt.Errorf("unknow callType: %s", request.CallType)
	}
}

func (d *DummyMirrorPlugin) handleList(ctx context.Context, request *stub.Request, params map[string]string) (*stub.Response, error) {
	workdir, err := d.currentWorkDir(ctx, request.WorkPath)
	if err != nil {
		return nil, err
	}
	resp := stub.NewResponse()
	resp.IsSucceed = true
	resp.Entries = workdir.SubEntries()
	return resp, nil
}

func (d *DummyMirrorPlugin) handleAdd(ctx context.Context, request *stub.Request, params map[string]string) (*stub.Response, error) {
	workdir, err := d.currentWorkDir(ctx, request.WorkPath)
	if err != nil {
		return nil, err
	}
	newEntry := request.Entry
	// overwrite old file if newEntry's name already existed
	if err := workdir.NewEntries(newEntry); err != nil {
		return nil, err
	}
	resp := stub.NewResponse()
	resp.IsSucceed = true
	return resp, nil
}

func (d *DummyMirrorPlugin) handleUpdate(ctx context.Context, request *stub.Request, params map[string]string) (*stub.Response, error) {
	workdir, err := d.currentWorkDir(ctx, request.WorkPath)
	if err != nil {
		return nil, err
	}
	ent := request.Entry
	var oldEnt stub.Entry
	subEntries := workdir.SubEntries()

	for i := range subEntries {
		tmpEnt := subEntries[i]
		if tmpEnt.Name() == ent.Name() {
			oldEnt = tmpEnt
			break
		}
	}
	if oldEnt == nil {
		return nil, types.ErrNotFound
	}
	if oldEnt.IsGroup() || ent.IsGroup() {

		return nil, types.ErrIsGroup
	}

	fileEnt, ok := oldEnt.(*stub.FileEntry)
	if !ok {
		return nil, fmt.Errorf("no file entry")
	}

	r, err := ent.OpenReader()
	if err != nil {
		return nil, err
	}
	defer r.Close()

	w, err := fileEnt.OpenWrite()
	if err != nil {
		return nil, err
	}
	defer w.Close()

	_, err = io.Copy(w, r)
	if err != nil {
		return nil, err
	}
	resp := stub.NewResponse()
	resp.IsSucceed = true
	resp.Entries = []stub.Entry{fileEnt}

	return resp, nil
}

func (d *DummyMirrorPlugin) handleDelete(ctx context.Context, request *stub.Request, params map[string]string) (*stub.Response, error) {
	workdir, err := d.currentWorkDir(ctx, request.WorkPath)
	if err != nil {
		return nil, err
	}
	ent := request.Entry
	if err = workdir.DeleteEntries(ent); err != nil {
		return nil, err
	}

	resp := stub.NewResponse()
	resp.IsSucceed = true
	return resp, nil
}

func (d *DummyMirrorPlugin) currentWorkDir(ctx context.Context, workDir string) (*stub.GroupEntry, error) {
	crtEntry := stub.Entry(d.dataSets)
	if workDir == crtEntry.Name() {
		return d.dataSets, nil
	}

	subEntryNames := strings.Split(workDir, utils.PathSeparator)
SEARCH:
	for _, sub := range subEntryNames {
		if crtEntry.Name() == sub {
			break
		}

		if !crtEntry.IsGroup() {
			return nil, types.ErrNoGroup
		}

		subEntries := crtEntry.SubEntries()
		for i := range subEntries {
			ent := subEntries[i]
			if ent.Name() == sub {
				crtEntry = ent
				continue SEARCH
			}
		}
		return nil, types.ErrNotFound
	}
	if !crtEntry.IsGroup() {
		return nil, types.ErrNoGroup
	}
	return crtEntry.(*stub.GroupEntry), nil
}
