package common

import (
	"bytes"
	"github.com/basenana/nanafs/pkg/types"
	"io"
	"io/ioutil"
	"sync"
)

type Entry interface {
	Name() string
	IsGroup() bool
	SubEntries() []Entry
	OpenReader() (io.ReadCloser, error)

	//Metadata() types.Metadata
}

type GroupEntry struct {
	name       string
	subEntries map[string]Entry
	mux        sync.Mutex
}

func NewGroupEntry(name string) Entry {
	return &GroupEntry{
		name:       name,
		subEntries: map[string]Entry{},
	}
}

func (g *GroupEntry) Name() string {
	return g.name
}

func (g *GroupEntry) IsGroup() bool {
	return true
}

func (g *GroupEntry) SubEntries() []Entry {
	result := make([]Entry, 0, len(g.subEntries))
	g.mux.Lock()
	for enName := range g.subEntries {
		result = append(result, g.subEntries[enName])
	}
	g.mux.Unlock()
	return result
}

func (g *GroupEntry) NewEntries(entries ...Entry) error {
	g.mux.Lock()
	for i := range entries {
		ent := entries[i]
		if _, ok := g.subEntries[ent.Name()]; ok {
			g.mux.Unlock()
			return types.ErrIsExist
		}
		g.subEntries[ent.Name()] = ent
	}
	g.mux.Unlock()
	return nil
}

func (g *GroupEntry) DeleteEntries(entries ...Entry) error {
	g.mux.Lock()
	for i := range entries {
		ent := entries[i]
		if _, ok := g.subEntries[ent.Name()]; !ok {
			g.mux.Unlock()
			return types.ErrNotFound
		}
		delete(g.subEntries, ent.Name())
	}
	g.mux.Unlock()
	return nil
}

func (g *GroupEntry) OpenReader() (io.ReadCloser, error) {
	return nil, types.ErrIsGroup
}

type FileEntry struct {
	name    string
	content []byte
}

func NewFileEntry(name string, content []byte) Entry {
	return &FileEntry{
		name:    name,
		content: content,
	}
}

func (f *FileEntry) Name() string {
	return f.name
}

func (f *FileEntry) IsGroup() bool {
	return false
}

func (f *FileEntry) SubEntries() []Entry {
	return nil
}

func (f *FileEntry) OpenReader() (io.ReadCloser, error) {
	rc := ioutil.NopCloser(bytes.NewReader(f.content))
	return rc, nil
}

func (f *FileEntry) OpenWrite() (io.WriteCloser, error) {
	return &fileWriter{Buffer: bytes.NewBuffer([]byte{}), ent: f}, nil
}

type fileWriter struct {
	*bytes.Buffer
	ent *FileEntry
}

func (f *fileWriter) Close() error {
	f.ent.content = f.Bytes()
	return nil
}
