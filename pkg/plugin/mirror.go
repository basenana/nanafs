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

	GetEntry(ctx context.Context, entryPath string) (*pluginapi.Entry, error)
	CreateEntry(ctx context.Context, parentPath string, attr pluginapi.EntryAttr) (*pluginapi.Entry, error)
	UpdateEntry(ctx context.Context, entryPath string, en *pluginapi.Entry) error
	RemoveEntry(ctx context.Context, entryPath string) error
	ListChildren(ctx context.Context, parentPath string) ([]*pluginapi.Entry, error)

	Open(ctx context.Context, entryPath string) (pluginapi.File, error)
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
	*MemFS
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

type MemFS struct {
	entries map[string]*pluginapi.Entry
	files   map[string]*memFile
	groups  map[string][]string
	mux     sync.Mutex
}

func (m *MemFS) GetEntry(ctx context.Context, enPath string) (*pluginapi.Entry, error) {
	m.mux.Lock()
	defer m.mux.Unlock()

	en, ok := m.entries[enPath]
	if !ok {
		return nil, types.ErrNotFound
	}
	return en, nil
}

func (m *MemFS) CreateEntry(ctx context.Context, parentPath string, attr pluginapi.EntryAttr) (*pluginapi.Entry, error) {
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

func (m *MemFS) UpdateEntry(ctx context.Context, enPath string, en *pluginapi.Entry) error {
	m.mux.Lock()
	defer m.mux.Unlock()

	old, ok := m.entries[enPath]
	if !ok {
		return types.ErrNotFound
	}

	// do update
	old.Size = en.Size
	return nil
}

func (m *MemFS) RemoveEntry(ctx context.Context, enPath string) error {
	m.mux.Lock()
	defer m.mux.Unlock()

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

	parentPath := path.Dir(enPath)
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

func (m *MemFS) ListChildren(ctx context.Context, enPath string) ([]*pluginapi.Entry, error) {
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

func (m *MemFS) Open(ctx context.Context, filePath string) (pluginapi.File, error) {
	m.mux.Lock()
	mf, ok := m.files[filePath]
	if !ok {
		m.mux.Unlock()
		return nil, types.ErrNotFound
	}
	m.mux.Unlock()
	return mf, nil
}

func NewMemFS() *MemFS {
	fs := &MemFS{
		entries: map[string]*pluginapi.Entry{"/": {Name: ".", Kind: types.ExternalGroupKind, Size: 0, IsGroup: true}},
		groups:  map[string][]string{"/": make([]string, 0)},
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

var _ pluginapi.File = &memFile{}

func (m *memFile) Fsync(ctx context.Context) error {
	return nil
}

func (m *memFile) Trunc(ctx context.Context) error {
	m.Size = 0
	return nil
}

func (m *memFile) Close(ctx context.Context) error {
	return nil
}

func (m *memFile) WriteAt(ctx context.Context, data []byte, off int64) (n int64, err error) {
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
	n = int64(copy(m.data[off:], data))
	if off+int64(n) > m.Size {
		m.Size = off + int64(n)
	}
	return
}

func (m *memFile) ReadAt(ctx context.Context, dest []byte, off int64) (n int64, err error) {
	if m.data == nil {
		err = types.ErrNotFound
		return
	}
	if off >= m.Size {
		return 0, io.EOF
	}
	return int64(copy(dest, m.data[off:m.Size])), nil
}

func (m *memFile) release() {
	if m.data != nil {
		utils.ReleaseMemoryBlock(m.data)
		m.data = nil
	}
}

func newMemFile(entry *pluginapi.Entry) *memFile {
	return &memFile{
		Entry: entry,
		data:  utils.NewMemoryBlock(memFileMaxSize / 16), // 1M
	}
}

func NewMemFSPlugin() *MemFSPlugin {
	plugin := &MemFSPlugin{MemFS: NewMemFS()}
	return plugin
}

func registerBuildInMirrorPlugin(r *registry) {
	plugin := NewMemFSPlugin()
	r.Register(memFSPluginName, types.PluginSpec{Name: memFSPluginName, Version: memFSPluginVersion,
		Type: types.TypeMirror, Parameters: map[string]string{}}, func(ctx context.Context, spec types.PluginSpec, scope types.PlugScope) (Plugin, error) {
		return plugin, nil
	})
}
