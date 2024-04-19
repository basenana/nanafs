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

package dentry

import (
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
	"path"
	"sync"
	"sync/atomic"
	"time"
)

const (
	externalStorage         = "[ext]"
	externalIDPrefix  int64 = 1
	entryIDPrefixMask int64 = 32
)

type ExtIndexer struct {
	stubs   map[int64]*StubRoot
	entries map[int64]*StubEntry
	mux     sync.Mutex
}

func (e *ExtIndexer) AddStubRoot(rootEn *types.Metadata, mirror plugin.MirrorPlugin) {
	e.mux.Lock()
	defer e.mux.Unlock()
	_, ok := e.stubs[rootEn.ID]
	if ok {
		return
	}
	sr := &StubRoot{ExtIndexer: e, id: rootEn.ID, mirror: mirror, access: rootEn.Access}
	e.stubs[rootEn.ID] = sr
	e.entries[rootEn.ID] = &StubEntry{
		id:     rootEn.ID,
		name:   rootEn.Name,
		parent: rootEn.ParentID,
		path:   "/",
		info: &pluginapi.Entry{
			Kind:       rootEn.Kind,
			IsGroup:    true,
			Parameters: map[string]string{},
		},
		root:     sr,
		children: map[int64]*StubEntry{},
	}
}

func (e *ExtIndexer) SetStubEntry(entry *StubEntry) {
	e.mux.Lock()
	defer e.mux.Unlock()
	if entry.id == 0 {
		entry.id = nextExtID()
	}
	e.entries[entry.id] = entry
	parent, ok := e.entries[entry.parent]
	if ok {
		parent.children[entry.id] = entry
	}
}

func (e *ExtIndexer) GetStubEntry(entryID int64) (*StubEntry, error) {
	e.mux.Lock()
	defer e.mux.Unlock()
	en, ok := e.entries[entryID]
	if !ok {
		return nil, types.ErrNotFound
	}
	return en, nil
}

func (e *ExtIndexer) RemoveStubEntry(entryID int64) {
	e.mux.Lock()
	defer e.mux.Unlock()
	en, ok := e.entries[entryID]
	if !ok {
		return
	}
	delete(e.entries, entryID)

	parent, ok := e.entries[en.parent]
	if ok {
		delete(parent.children, entryID)
	}
}

func NewExtIndexer() *ExtIndexer {
	return &ExtIndexer{stubs: map[int64]*StubRoot{}, entries: map[int64]*StubEntry{}}
}

type StubRoot struct {
	*ExtIndexer
	id     int64
	mirror plugin.MirrorPlugin
	access types.Access
}

type StubEntry struct {
	id         int64
	name       string
	parent     int64
	path       string
	info       *pluginapi.Entry
	root       *StubRoot
	registerAt time.Time
	children   map[int64]*StubEntry
}

func (s *StubEntry) updateChild(entry *pluginapi.Entry) error {
	exist, err := s.findChild(entry.Name)
	if err != nil {
		return err
	}
	en, err := s.root.GetStubEntry(exist.ID)
	if err != nil {
		return err
	}
	en.info = entry
	return nil
}

func (s *StubEntry) createChild(entry *pluginapi.Entry) (*types.Metadata, error) {
	exist, _ := s.findChild(entry.Name)
	if exist != nil {
		return exist, nil
	}

	en := covert2StubEntry(s.root, s, entry)
	s.root.SetStubEntry(en)
	return en.toEntry(), nil
}

func (s *StubEntry) removeChild(entryID int64) error {
	en, err := s.root.GetStubEntry(entryID)
	if err != nil {
		return err
	}
	if en.parent == s.id {
		s.root.RemoveStubEntry(entryID)
		return nil
	}
	return types.ErrNotFound
}

func (s *StubEntry) findChild(name string) (*types.Metadata, error) {
	for _, ch := range s.listChildren() {
		if ch.Name == name {
			return ch, nil
		}
	}
	return nil, types.ErrNotFound
}

func (s *StubEntry) listChildren() []*types.Metadata {
	var children []*StubEntry
	s.root.mux.Lock()
	for eid := range s.children {
		children = append(children, s.children[eid])
	}
	s.root.mux.Unlock()

	result := make([]*types.Metadata, len(children))
	for i, en := range children {
		result[i] = en.toEntry()
	}

	return result
}

func (s *StubEntry) toEntry() *types.Metadata {
	return &types.Metadata{
		ID:         s.id,
		Name:       s.name,
		ParentID:   s.parent,
		Kind:       s.info.Kind,
		IsGroup:    s.info.IsGroup,
		Size:       s.info.Size,
		Storage:    externalStorage,
		CreatedAt:  s.registerAt,
		ChangedAt:  s.registerAt,
		ModifiedAt: s.registerAt,
		AccessAt:   time.Now(),
		Access:     s.root.access,
	}
}

var (
	globalNextID uint32 = 1024
)

func nextExtID() int64 {
	return externalIDPrefix<<entryIDPrefixMask | int64(atomic.AddUint32(&globalNextID, 1))
}

func covert2StubEntry(root *StubRoot, parent *StubEntry, en *pluginapi.Entry) *StubEntry {
	se := &StubEntry{
		name:       en.Name,
		parent:     parent.id,
		path:       path.Join(parent.path, en.Name),
		info:       en,
		root:       root,
		registerAt: time.Now(),
	}
	if en.IsGroup {
		se.children = map[int64]*StubEntry{}
	}
	return se
}
