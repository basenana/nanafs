package fs

import (
	"context"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/files"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/hanwen/go-fuse/v2/fs"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sync"
	"testing"
	"time"
)

type MockController struct {
	entries map[string]*types.Object
	mux     sync.Mutex
}

var _ controller.Controller = &MockController{}

func (m *MockController) LoadRootEntry(ctx context.Context) (*types.Object, error) {
	m.mux.Lock()
	defer m.mux.Unlock()

	entry, ok := m.entries[types.RootEntryID]
	if ok {
		return entry, nil
	}

	root := types.InitRootEntry()
	m.entries[types.RootEntryID] = root
	return root, nil
}

func (m *MockController) FindEntry(ctx context.Context, parent *types.Object, name string) (*types.Object, error) {
	m.mux.Lock()
	defer m.mux.Unlock()

	var result *types.Object
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
		return nil, types.ErrNotFound
	}

	return result, nil
}

func (m *MockController) CreateEntry(ctx context.Context, parent *types.Object, attr types.ObjectAttr) (*types.Object, error) {
	entry, err := types.InitNewEntry(parent, attr)
	if err != nil {
		return nil, err
	}

	return entry, m.SaveEntry(ctx, entry)
}

func (m *MockController) SaveEntry(ctx context.Context, entry *types.Object) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.entries[entry.ID] = entry
	return nil
}

func (m *MockController) DestroyEntry(ctx context.Context, entry *types.Object) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	delete(m.entries, entry.ID)
	return nil
}

func (m *MockController) MirrorEntry(ctx context.Context, src, dstParent *types.Object, attr types.ObjectAttr) (*types.Object, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockController) ListEntryChildren(ctx context.Context, entry *types.Object) ([]*types.Object, error) {
	result := make([]*types.Object, 0)
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

func (m *MockController) ChangeEntryParent(ctx context.Context, old, newParent *types.Object, newName string) error {
	//TODO implement me
	panic("implement me")
}

func (m *MockController) OpenFile(ctx context.Context, entry *types.Object, attr files.Attr) (*files.File, error) {
	return &files.File{}, nil
}

func (m *MockController) WriteFile(ctx context.Context, file *files.File, data []byte, offset int64) (n int64, err error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockController) CloseFile(ctx context.Context, file *files.File) error {
	//TODO implement me
	panic("implement me")
}

func (m *MockController) DeleteFileData(ctx context.Context, entry *types.Object) error {
	//TODO implement me
	panic("implement me")
}

func (m *MockController) FsInfo(ctx context.Context) controller.Info {
	return controller.Info{}
}

func NewMockController() controller.Controller {
	return &MockController{
		entries: map[string]*types.Object{},
	}
}

func initFsBridge(nfs *NanaFS) *NanaNode {
	nfs.logger = logger.NewLogger("test-fs")
	root, _ := nfs.newFsNode(context.Background(), nil, nil)
	oneSecond := time.Second
	_ = fs.NewNodeFS(root, &fs.Options{
		EntryTimeout: &oneSecond,
		AttrTimeout:  &oneSecond,
	})
	return root
}

func TestFs(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Fs Suite")
}
