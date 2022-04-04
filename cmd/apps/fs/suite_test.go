package fs

import (
	"context"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/files"
	"github.com/basenana/nanafs/pkg/object"
	"github.com/hanwen/go-fuse/v2/fs"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sync"
	"testing"
	"time"
)

type MockController struct {
	entries map[string]*dentry.Entry
	mux     sync.Mutex
}

func (m *MockController) LoadRootEntry(ctx context.Context) (*dentry.Entry, error) {
	m.mux.Lock()
	defer m.mux.Unlock()

	entry, ok := m.entries[dentry.RootEntryID]
	if ok {
		return entry, nil
	}

	root := dentry.InitRootEntry()
	m.entries[dentry.RootEntryID] = root
	return root, nil
}

func (m *MockController) FindEntry(ctx context.Context, parent *dentry.Entry, name string) (*dentry.Entry, error) {
	m.mux.Lock()
	defer m.mux.Unlock()

	var result *dentry.Entry
	for oid, entry := range m.entries {
		if entry.ParentID != parent.ID {
			continue
		}
		if entry.Name == name {
			result = m.entries[oid]
			break
		}
	}

	if result == nil {
		return nil, object.ErrNotFound
	}

	return result, nil
}

func (m *MockController) CreateEntry(ctx context.Context, parent *dentry.Entry, attr dentry.EntryAttr) (*dentry.Entry, error) {
	entry, err := dentry.InitNewEntry(parent, attr)
	if err != nil {
		return nil, err
	}

	return entry, m.SaveEntry(ctx, entry)
}

func (m *MockController) SaveEntry(ctx context.Context, entry *dentry.Entry) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.entries[entry.ID] = entry
	return nil
}

func (m *MockController) DestroyEntry(ctx context.Context, entry *dentry.Entry) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	delete(m.entries, entry.ID)
	return nil
}

func (m *MockController) MirrorEntry(ctx context.Context, src, dstParent *dentry.Entry, attr dentry.EntryAttr) (*dentry.Entry, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockController) ListEntryChildren(ctx context.Context, entry *dentry.Entry) ([]*dentry.Entry, error) {
	result := make([]*dentry.Entry, 0)
	for oid, e := range m.entries {
		if e.ParentID != entry.ID {
			continue
		}
		if e.ID == e.ParentID {
			continue
		}
		result = append(result, m.entries[oid])
	}
	return result, nil
}

func (m *MockController) ChangeEntryParent(ctx context.Context, old, newParent *dentry.Entry, newName string) error {
	//TODO implement me
	panic("implement me")
}

func (m *MockController) OpenFile(ctx context.Context, entry *dentry.Entry, attr files.Attr) (*files.File, error) {
	return &files.File{}, nil
}

func (m *MockController) CloseFile(ctx context.Context, file *files.File) error {
	//TODO implement me
	panic("implement me")
}

func (m *MockController) DeleteFileData(ctx context.Context, entry *dentry.Entry) error {
	//TODO implement me
	panic("implement me")
}

func (m *MockController) FsInfo(ctx context.Context) controller.Info {
	return controller.Info{}
}

func NewMockController() controller.Controller {
	return &MockController{
		entries: map[string]*dentry.Entry{},
	}
}

func initFsBridge(nfs *NanaFS) *NanaNode {
	root, _ := nfs.newFsNode(context.Background(), nil, nil)
	oneSecond := time.Second
	_ = fs.NewNodeFS(root, &fs.Options{
		EntryTimeout: &oneSecond,
		AttrTimeout:  &oneSecond,
	})
	return root
}

func TestFs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Fs Suite")
}
