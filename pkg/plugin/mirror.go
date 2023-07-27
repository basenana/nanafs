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
	"github.com/basenana/nanafs/utils"
	"io"
	"path"
	"sync"
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

const (
	memFSPluginName    = "memfs"
	memFSPluginVersion = "1.0"
)

type MemFSPlugin struct {
	path string
	fs   *MemFS
}

var _ MirrorPlugin = &MemFSPlugin{}

func (d *MemFSPlugin) Name() string {
	return memFSPluginName
}

func (d *MemFSPlugin) Type() types.PluginType {
	return types.TypeMirror
}

func (d *MemFSPlugin) Version() string {
	return memFSPluginVersion
}
func (d *MemFSPlugin) build(ctx context.Context, spec types.PluginSpec, scope types.PlugScope) (Plugin, error) {
	if scope.Parameters == nil {
		scope.Parameters = map[string]string{}
	}
	enPath := scope.Parameters[types.PlugScopeEntryPath]
	if enPath == "" {
		return nil, fmt.Errorf("path is empty")
	}

	d.fs.mux.Lock()
	_, isDir := d.fs.groups[enPath]
	_, isFile := d.fs.files[enPath]
	d.fs.mux.Unlock()
	if isDir || isFile {
		return &MemFSPlugin{path: enPath, fs: d.fs}, nil
	}
	return nil, types.ErrNotFound
}

func (d *MemFSPlugin) IsGroup(ctx context.Context) (bool, error) {
	d.fs.mux.Lock()
	defer d.fs.mux.Unlock()
	_, ok := d.fs.groups[d.path]
	return ok, nil
}

func (d *MemFSPlugin) FindEntry(ctx context.Context, name string) (*stub.Entry, error) {
	d.fs.mux.Lock()
	defer d.fs.mux.Unlock()
	child, ok := d.fs.groups[d.path]
	if !ok {
		return nil, types.ErrNoGroup
	}

	for _, chName := range child {
		if chName != name {
			continue
		}
		en := &stub.Entry{Name: chName, Kind: types.RawKind}
		if _, ok = d.fs.groups[path.Join(d.path, name)]; ok {
			en.Kind = types.ExternalGroupKind
			en.IsGroup = true
		}
		return en, nil
	}

	return nil, types.ErrNotFound
}

func (d *MemFSPlugin) CreateEntry(ctx context.Context, attr stub.EntryAttr) (*stub.Entry, error) {
	d.fs.mux.Lock()
	defer d.fs.mux.Unlock()
	child, ok := d.fs.groups[d.path]
	if !ok {
		return nil, types.ErrNoGroup
	}

	for _, chName := range child {
		if chName == attr.Name {
			return nil, types.ErrIsExist
		}
	}

	child = append(child, attr.Name)
	d.fs.groups[d.path] = child
	d.fs.files[path.Join(d.path, attr.Name)] = newMemFile(attr.Name)

	if types.IsGroup(attr.Kind) {
		d.fs.groups[path.Join(d.path, attr.Name)] = make([]string, 0)
	}

	return &stub.Entry{
		Name:    attr.Name,
		Kind:    attr.Kind,
		IsGroup: types.IsGroup(attr.Kind),
	}, nil
}

func (d *MemFSPlugin) UpdateEntry(ctx context.Context, en *stub.Entry) error {
	d.fs.mux.Lock()
	defer d.fs.mux.Unlock()
	child, ok := d.fs.groups[d.path]
	if !ok {
		return types.ErrNoGroup
	}

	for _, chName := range child {
		if chName == en.Name {
			return nil
		}
	}
	return types.ErrNotFound
}

func (d *MemFSPlugin) RemoveEntry(ctx context.Context, en *stub.Entry) error {
	d.fs.mux.Lock()
	defer d.fs.mux.Unlock()
	child, ok := d.fs.groups[d.path]
	if !ok {
		return types.ErrNoGroup
	}

	newChild := make([]string, 0, len(child))
	found := false
	for _, chName := range child {
		if chName == en.Name {
			found = true
			continue
		}
		newChild = append(newChild, chName)
	}

	if !found {
		return types.ErrNotFound
	}

	needDeleteChild, ok := d.fs.groups[path.Join(d.path, en.Name)]
	if ok && len(needDeleteChild) > 0 {
		return types.ErrNotEmpty
	}
	d.fs.groups[d.path] = newChild
	return nil
}

func (d *MemFSPlugin) ListChildren(ctx context.Context) ([]*stub.Entry, error) {
	d.fs.mux.Lock()
	defer d.fs.mux.Unlock()
	child, ok := d.fs.groups[d.path]
	if !ok {
		return nil, types.ErrNoGroup
	}

	result := make([]*stub.Entry, len(child))
	for i, chName := range child {
		result[i] = &stub.Entry{
			Name:    chName,
			Kind:    types.RawKind,
			IsGroup: false,
		}
		if _, isDir := d.fs.groups[path.Join(d.path, chName)]; isDir {
			result[i].Kind = types.ExternalGroupKind
			result[i].IsGroup = true
		}
	}
	return result, nil
}

func (d *MemFSPlugin) WriteAt(ctx context.Context, data []byte, off int64) (int64, error) {
	d.fs.mux.Lock()
	defer d.fs.mux.Unlock()

	mf, ok := d.fs.files[d.path]
	if !ok {
		return 0, types.ErrNotFound
	}

	n, err := mf.WriteAt(data, off)
	return int64(n), err
}

func (d *MemFSPlugin) ReadAt(ctx context.Context, dest []byte, off int64) (int64, error) {
	d.fs.mux.Lock()
	defer d.fs.mux.Unlock()

	mf, ok := d.fs.files[d.path]
	if !ok {
		return 0, types.ErrNotFound
	}

	n, err := mf.ReadAt(dest, off)
	return int64(n), err
}

func (d *MemFSPlugin) Fsync(ctx context.Context) error {
	return nil
}

func (d *MemFSPlugin) Flush(ctx context.Context) error {
	return nil
}

func (d *MemFSPlugin) Close(ctx context.Context) error {
	return nil
}

type MemFS struct {
	files  map[string]*memFile
	groups map[string][]string
	mux    sync.Mutex
}

const (
	memFileMaxSize int64 = 1 << 24 // 16M
)

type memFile struct {
	name string
	data []byte
	off  int64
}

func newMemFile(name string) *memFile {
	return &memFile{
		name: name,
		data: utils.NewMemoryBlock(memFileMaxSize / 16), // 1M
		off:  0,
	}
}

func (m *memFile) WriteAt(data []byte, off int64) (n int, err error) {
	if m.data == nil {
		err = types.ErrNotFound
		return
	}
	if off+int64(len(data)) > int64(len(data)) {
		if off+int64(len(data)) > memFileMaxSize {
			return 0, io.ErrShortBuffer
		}
		blk := utils.NewMemoryBlock(off + int64(len(data)) + 1)
		copy(blk, m.data[:m.off])
		utils.ReleaseMemoryBlock(m.data)
		m.data = blk
	}
	n = copy(m.data[off:], data)
	m.off += int64(n)
	return
}

func (m *memFile) ReadAt(dest []byte, off int64) (n int, err error) {
	if m.data == nil {
		err = types.ErrNotFound
		return
	}
	if off >= m.off {
		return 0, io.EOF
	}
	return copy(dest, m.data[off:m.off]), nil
}

func registerMemfsPlugin(r *registry) {
	plugin := &MemFSPlugin{
		path: "/",
		fs:   &MemFS{files: map[string]*memFile{}, groups: map[string][]string{}},
	}
	plugin.fs.groups["/"] = make([]string, 0)
	r.Register(memFSPluginName, types.PluginSpec{Name: memFSPluginName, Version: memFSPluginVersion,
		Type: types.TypeMirror, Parameters: map[string]string{}}, plugin.build)
}
