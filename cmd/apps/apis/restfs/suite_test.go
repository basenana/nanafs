package restfs

import (
	"context"
	"github.com/basenana/go-flow/fsm"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/files"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/pkg/workflow"
	"github.com/basenana/nanafs/utils/logger"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type MockController struct {
}

func (m MockController) LoadRootObject(ctx context.Context) (*types.Object, error) {
	//TODO implement me
	panic("implement me")
}

func (m MockController) FindObject(ctx context.Context, parent *types.Object, name string) (*types.Object, error) {
	//TODO implement me
	panic("implement me")
}

func (m MockController) CreateObject(ctx context.Context, parent *types.Object, attr types.ObjectAttr) (*types.Object, error) {
	//TODO implement me
	panic("implement me")
}

func (m MockController) SaveObject(ctx context.Context, obj *types.Object) error {
	//TODO implement me
	panic("implement me")
}

func (m MockController) DestroyObject(ctx context.Context, obj *types.Object) error {
	//TODO implement me
	panic("implement me")
}

func (m MockController) MirrorObject(ctx context.Context, src, dstParent *types.Object, attr types.ObjectAttr) (*types.Object, error) {
	//TODO implement me
	panic("implement me")
}

func (m MockController) ListObjectChildren(ctx context.Context, obj *types.Object) ([]*types.Object, error) {
	//TODO implement me
	panic("implement me")
}

func (m MockController) ChangeObjectParent(ctx context.Context, old, newParent *types.Object, newName string, opt controller.ChangeParentOpt) error {
	//TODO implement me
	panic("implement me")
}

func (m MockController) OpenFile(ctx context.Context, obj *types.Object, attr files.Attr) (files.File, error) {
	//TODO implement me
	panic("implement me")
}

func (m MockController) ReadFile(ctx context.Context, file files.File, data []byte, offset int64) (n int, err error) {
	//TODO implement me
	panic("implement me")
}

func (m MockController) WriteFile(ctx context.Context, file files.File, data []byte, offset int64) (n int64, err error) {
	//TODO implement me
	panic("implement me")
}

func (m MockController) CloseFile(ctx context.Context, file files.File) error {
	//TODO implement me
	panic("implement me")
}

func (m MockController) DeleteFileData(ctx context.Context, obj *types.Object) error {
	//TODO implement me
	panic("implement me")
}

func (m MockController) FsInfo(ctx context.Context) controller.Info {
	//TODO implement me
	panic("implement me")
}

func (m MockController) ListStructuredObject(ctx context.Context, cType types.Kind, version string) ([]*types.Object, error) {
	//TODO implement me
	panic("implement me")
}

func (m MockController) OpenStructuredObject(ctx context.Context, obj *types.Object, spec interface{}, attr files.Attr) (files.File, error) {
	//TODO implement me
	panic("implement me")
}

func (m MockController) CleanStructuredObject(ctx context.Context, obj *types.Object) error {
	//TODO implement me
	panic("implement me")
}

func (m MockController) CreateStructuredObject(ctx context.Context, parent *types.Object, attr types.ObjectAttr, cType types.Kind, version string) (*types.Object, error) {
	//TODO implement me
	panic("implement me")
}

func (m MockController) Trigger(ctx context.Context, o *types.Object) {
	//TODO implement me
	panic("implement me")
}

func (m MockController) GetJobs(ctx context.Context) map[fsm.Status]*workflow.Job {
	//TODO implement me
	panic("implement me")
}

func TestFile(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()
	RegisterFailHandler(Fail)
	RunSpecs(t, "RestFS Suite")
}
