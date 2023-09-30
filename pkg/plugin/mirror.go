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
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"io"
	"path"
	"sync"
)

type MirrorPlugin interface {
	Plugin

	IsGroup(ctx context.Context) (bool, error)
	FindEntry(ctx context.Context, name string) (*pluginapi.Entry, error)
	CreateEntry(ctx context.Context, attr pluginapi.EntryAttr) (*pluginapi.Entry, error)
	UpdateEntry(ctx context.Context, en *pluginapi.Entry) error
	RemoveEntry(ctx context.Context, en *pluginapi.Entry) error
	ListChildren(ctx context.Context) ([]*pluginapi.Entry, error)

	WriteAt(ctx context.Context, data []byte, off int64) (int64, error)
	ReadAt(ctx context.Context, dest []byte, off int64) (int64, error)
	Fsync(ctx context.Context) error
	Trunc(ctx context.Context) error
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

	_, err := d.fs.GetEntry(enPath)
	if err != nil {
		return nil, err
	}
	return &MemFSPlugin{path: enPath, fs: d.fs}, nil
}

func (d *MemFSPlugin) IsGroup(ctx context.Context) (bool, error) {
	en, err := d.fs.GetEntry(d.path)
	if err != nil {
		return false, err
	}
	return en.IsGroup, nil
}

func (d *MemFSPlugin) FindEntry(ctx context.Context, name string) (*pluginapi.Entry, error) {
	return d.fs.FindEntry(d.path, name)
}

func (d *MemFSPlugin) CreateEntry(ctx context.Context, attr pluginapi.EntryAttr) (*pluginapi.Entry, error) {
	return d.fs.CreateEntry(d.path, attr)
}

func (d *MemFSPlugin) UpdateEntry(ctx context.Context, en *pluginapi.Entry) error {
	return d.fs.UpdateEntry(d.path, en)
}

func (d *MemFSPlugin) RemoveEntry(ctx context.Context, en *pluginapi.Entry) error {
	return d.fs.RemoveEntry(d.path, en)
}

func (d *MemFSPlugin) ListChildren(ctx context.Context) ([]*pluginapi.Entry, error) {
	return d.fs.ListChildren(d.path)
}

func (d *MemFSPlugin) WriteAt(ctx context.Context, data []byte, off int64) (int64, error) {
	return d.fs.WriteAt(d.path, data, off)
}

func (d *MemFSPlugin) ReadAt(ctx context.Context, dest []byte, off int64) (int64, error) {
	return d.fs.ReadAt(d.path, dest, off)
}

func (d *MemFSPlugin) Trunc(ctx context.Context) error {
	return d.fs.Trunc(d.path)
}

func (d *MemFSPlugin) Fsync(ctx context.Context) error {
	return nil
}

func (d *MemFSPlugin) Close(ctx context.Context) error {
	return nil
}

type MemFS struct {
	entries map[string]*pluginapi.Entry
	files   map[string]*memFile
	groups  map[string][]string
	mux     sync.Mutex
}

func (m *MemFS) GetEntry(enPath string) (*pluginapi.Entry, error) {
	m.mux.Lock()
	defer m.mux.Unlock()

	en, ok := m.entries[enPath]
	if !ok {
		return nil, types.ErrNotFound
	}
	return en, nil
}

func (m *MemFS) FindEntry(parentPath string, name string) (*pluginapi.Entry, error) {
	m.mux.Lock()
	defer m.mux.Unlock()

	en, ok := m.entries[path.Join(parentPath, name)]
	if !ok {
		return nil, types.ErrNotFound
	}
	return en, nil
}

func (m *MemFS) CreateEntry(parentPath string, attr pluginapi.EntryAttr) (*pluginapi.Entry, error) {
	m.mux.Lock()
	defer m.mux.Unlock()

	parent, ok := m.entries[parentPath]
	if !ok {
		return nil, types.ErrNotFound
	}
	if !parent.IsGroup {
		return nil, types.ErrNoGroup
	}

	child := m.groups[parentPath]
	for _, chName := range child {
		if chName == attr.Name {
			return nil, types.ErrIsExist
		}
	}

	child = append(child, attr.Name)
	m.groups[parentPath] = child

	en := &pluginapi.Entry{
		Name:    attr.Name,
		Kind:    attr.Kind,
		IsGroup: types.IsGroup(attr.Kind),
	}
	enPath := path.Join(parentPath, attr.Name)
	if en.IsGroup {
		m.groups[enPath] = make([]string, 0)
	} else {
		m.files[enPath] = newMemFile(en)
	}
	m.entries[enPath] = en
	return en, nil
}

func (m *MemFS) UpdateEntry(parentPath string, en *pluginapi.Entry) error {
	m.mux.Lock()
	defer m.mux.Unlock()

	enPath := path.Join(parentPath, en.Name)
	old, ok := m.entries[enPath]
	if !ok {
		return types.ErrNotFound
	}

	// do update
	old.Size = en.Size
	return nil
}

func (m *MemFS) RemoveEntry(parentPath string, en *pluginapi.Entry) error {
	m.mux.Lock()
	defer m.mux.Unlock()

	enPath := path.Join(parentPath, en.Name)
	en, ok := m.entries[enPath]
	if !ok {
		return types.ErrNotFound
	}

	if en.IsGroup {
		if len(m.groups[enPath]) > 0 {
			return types.ErrNotEmpty
		}
		delete(m.groups, enPath)
	} else {
		f := m.files[enPath]
		if f != nil {
			f.release()
			delete(m.files, enPath)
		}
	}

	child := m.groups[parentPath]
	newChild := make([]string, 0, len(child)-1)
	for _, chName := range child {
		if chName == en.Name {
			continue
		}
		newChild = append(newChild, chName)
	}
	m.groups[parentPath] = newChild

	delete(m.entries, enPath)
	return nil
}

func (m *MemFS) ListChildren(enPath string) ([]*pluginapi.Entry, error) {
	m.mux.Lock()
	defer m.mux.Unlock()

	en, ok := m.entries[enPath]
	if !ok {
		return nil, types.ErrNotFound
	}

	if !en.IsGroup {
		return nil, types.ErrNoGroup
	}

	childNames := m.groups[enPath]
	result := make([]*pluginapi.Entry, len(childNames))
	for i, chName := range childNames {
		result[i] = m.entries[path.Join(enPath, chName)]
	}
	return result, nil
}

func (m *MemFS) WriteAt(filePath string, data []byte, off int64) (int64, error) {
	m.mux.Lock()

	mf, ok := m.files[filePath]
	if !ok {
		m.mux.Unlock()
		return 0, types.ErrNotFound
	}
	m.mux.Unlock()
	n, err := mf.WriteAt(data, off)
	return int64(n), err
}

func (m *MemFS) ReadAt(filePath string, dest []byte, off int64) (int64, error) {
	m.mux.Lock()

	mf, ok := m.files[filePath]
	if !ok {
		m.mux.Unlock()
		return 0, types.ErrNotFound
	}
	m.mux.Unlock()
	n, err := mf.ReadAt(dest, off)
	return int64(n), err
}

func (m *MemFS) Trunc(filePath string) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	mf, ok := m.files[filePath]
	if !ok {
		return types.ErrNotFound
	}
	mf.Size = 0
	return nil
}

func NewMemFS() *MemFS {
	fs := &MemFS{
		entries: map[string]*pluginapi.Entry{"/": {Name: ".", Kind: types.ExternalGroupKind, Size: 0, IsGroup: true}},
		groups:  map[string][]string{"/": {}},
		files:   map[string]*memFile{},
	}
	return fs
}

const (
	memFileMaxSize int64 = 1 << 24 // 16M
)

type memFile struct {
	*pluginapi.Entry
	data []byte
}

func newMemFile(entry *pluginapi.Entry) *memFile {
	return &memFile{
		Entry: entry,
		data:  utils.NewMemoryBlock(memFileMaxSize / 16), // 1M
	}
}

func (m *memFile) WriteAt(data []byte, off int64) (n int, err error) {
	if m.data == nil {
		err = types.ErrNotFound
		return
	}
	if off+int64(len(data)) > int64(len(m.data)) {
		if off+int64(len(data)) > memFileMaxSize {
			return 0, io.ErrShortBuffer
		}
		blk := utils.NewMemoryBlock(off + int64(len(data)) + 1)
		copy(blk, m.data[:m.Size])
		utils.ReleaseMemoryBlock(m.data)
		m.data = blk
	}
	n = copy(m.data[off:], data)
	if off+int64(n) > m.Size {
		m.Size = off + int64(n)
	}
	return
}

func (m *memFile) ReadAt(dest []byte, off int64) (n int, err error) {
	if m.data == nil {
		err = types.ErrNotFound
		return
	}
	if off >= m.Size {
		return 0, io.EOF
	}
	return copy(dest, m.data[off:m.Size]), nil
}

func (m *memFile) release() {
	if m.data != nil {
		utils.ReleaseMemoryBlock(m.data)
		m.data = nil
	}
}

func NewMemFSPlugin() *MemFSPlugin {
	plugin := &MemFSPlugin{path: "/", fs: NewMemFS()}
	return plugin
}

func registerMemfsPlugin(r *registry) {
	plugin := NewMemFSPlugin()
	plugin.fs.groups["/"] = make([]string, 0)
	r.Register(memFSPluginName, types.PluginSpec{Name: memFSPluginName, Version: memFSPluginVersion,
		Type: types.TypeMirror, Parameters: map[string]string{}}, plugin.build)
}
