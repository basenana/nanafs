package fs

import (
	"context"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/files"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/hanwen/go-fuse/v2/fs"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sync"
	"testing"
	"time"
)

var (
	cfg config.Config
)

type MockController struct {
	objects map[string]*types.Object
	mux     sync.Mutex
}

func (m *MockController) GetObject(ctx context.Context, id string) (*types.Object, error) {
	m.mux.Lock()
	defer m.mux.Unlock()
	obj, ok := m.objects[id]
	if !ok {
		return nil, types.ErrNotFound
	}
	return obj, nil
}

func (m *MockController) IsStructured(obj *types.Object) bool {
	//TODO implement me
	panic("implement me")
}

func (m *MockController) ListStructuredObject(ctx context.Context, cType types.Kind, version string) ([]*types.Object, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockController) OpenStructuredObject(ctx context.Context, obj *types.Object, spec interface{}, attr files.Attr) (files.File, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockController) CleanStructuredObject(ctx context.Context, parent, obj *types.Object) error {
	//TODO implement me
	panic("implement me")
}

func (m *MockController) CreateStructuredObject(ctx context.Context, parent *types.Object, attr types.ObjectAttr, cType types.Kind, version string) (*types.Object, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockController) LoadStructureObject(ctx context.Context, obj *types.Object, spec interface{}) error {
	//TODO implement me
	panic("implement me")
}

func (m *MockController) SaveStructureObject(ctx context.Context, obj *types.Object, spec interface{}) error {
	//TODO implement me
	panic("implement me")
}

func (m *MockController) GetJobs(ctx context.Context) map[string]*types.Job {
	//TODO implement me
	panic("implement me")
}

func (m *MockController) ReadFile(ctx context.Context, file files.File, data []byte, offset int64) (n int, err error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockController) OpenFile(ctx context.Context, obj *types.Object, attr files.Attr) (files.File, error) {
	obj.ModifiedAt = time.Now()
	return files.Open(ctx, obj, attr)
}

func (m *MockController) WriteFile(ctx context.Context, file files.File, data []byte, offset int64) (n int64, err error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockController) CloseFile(ctx context.Context, file files.File) error {
	//TODO implement me
	panic("implement me")
}

func (m *MockController) Trigger(ctx context.Context, o *types.Object) {
	//TODO implement me
	panic("implement me")
}

func (m *MockController) ChangeObjectParent(ctx context.Context, old, oldParent, newParent *types.Object, newName string, opt controller.ChangeParentOpt) error {
	//TODO implement me
	panic("implement me")
}

var _ controller.Controller = &MockController{}

func (m *MockController) LoadRootObject(ctx context.Context) (*types.Object, error) {
	m.mux.Lock()
	defer m.mux.Unlock()

	obj, ok := m.objects[dentry.RootObjectID]
	if ok {
		return obj, nil
	}

	root := dentry.InitRootObject()
	m.objects[dentry.RootObjectID] = root
	return root, nil
}

func (m *MockController) FindObject(ctx context.Context, parent *types.Object, name string) (*types.Object, error) {
	m.mux.Lock()
	defer m.mux.Unlock()

	var result *types.Object
	for oid, obj := range m.objects {
		if obj.ParentID != parent.ID {
			continue
		}
		if obj.Name == name {
			result = m.objects[oid]
			break
		}
	}

	if result == nil {
		return nil, types.ErrNotFound
	}

	return result, nil
}

func (m *MockController) CreateObject(ctx context.Context, parent *types.Object, attr types.ObjectAttr) (*types.Object, error) {
	obj, err := types.InitNewObject(parent, attr)
	if err != nil {
		return nil, err
	}

	return obj, m.SaveObject(ctx, parent, obj)
}

func (m *MockController) SaveObject(ctx context.Context, parent, obj *types.Object) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.objects[obj.ID] = obj
	return nil
}

func (m *MockController) DestroyObject(ctx context.Context, parent, obj *types.Object) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	delete(m.objects, obj.ID)
	return nil
}

func (m *MockController) MirrorObject(ctx context.Context, src, dstParent *types.Object, attr types.ObjectAttr) (*types.Object, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockController) ListObjectChildren(ctx context.Context, obj *types.Object) ([]*types.Object, error) {
	result := make([]*types.Object, 0)
	for oid, e := range m.objects {
		if e.ParentID != obj.ID {
			continue
		}
		if e.ID == e.ParentID {
			continue
		}
		result = append(result, m.objects[oid])
	}
	return result, nil
}

func (m *MockController) DeleteFileData(ctx context.Context, obj *types.Object) error {
	//TODO implement me
	panic("implement me")
}

func (m *MockController) FsInfo(ctx context.Context) controller.Info {
	return controller.Info{}
}

func NewMockController() controller.Controller {
	return &MockController{
		objects: map[string]*types.Object{},
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

func mustGetNanaObj(node *NanaNode, ctrl controller.Controller) *types.Object {
	result, err := ctrl.GetObject(context.Background(), node.oid)
	if err != nil {
		panic(err)
	}
	return result
}

func TestFs(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()

	cfg = config.Config{}
	_ = config.Verify(&cfg)

	s, _ := storage.NewStorage("memory", config.Storage{})
	files.InitFileIoChain(cfg, s, make(chan struct{}))
	RegisterFailHandler(Fail)
	RunSpecs(t, "Fs Suite")
}
