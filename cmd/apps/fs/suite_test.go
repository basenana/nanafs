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

package fs

import (
	"context"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/files"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
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
	entries map[int64]dentry.Entry
	mux     sync.Mutex
}

func (m *MockController) GetEntry(ctx context.Context, id int64) (dentry.Entry, error) {
	m.mux.Lock()
	defer m.mux.Unlock()
	entry, ok := m.entries[id]
	if !ok {
		return nil, types.ErrNotFound
	}
	return entry, nil
}

func (m *MockController) ReadFile(ctx context.Context, file files.File, data []byte, offset int64) (n int, err error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockController) OpenFile(ctx context.Context, en dentry.Entry, attr files.Attr) (files.File, error) {
	en.Metadata().ModifiedAt = time.Now()
	return files.Open(ctx, en.Object(), attr)
}

func (m *MockController) WriteFile(ctx context.Context, file files.File, data []byte, offset int64) (n int64, err error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockController) CloseFile(ctx context.Context, file files.File) error {
	//TODO implement me
	panic("implement me")
}

func (m *MockController) Trigger(ctx context.Context, o dentry.Entry) {
	//TODO implement me
	panic("implement me")
}

func (m *MockController) ChangeEntryParent(ctx context.Context, old, oldParent, newParent dentry.Entry, newName string, opt types.ChangeParentAttr) error {
	//TODO implement me
	panic("implement me")
}

var _ controller.Controller = &MockController{}

func (m *MockController) LoadRootEntry(ctx context.Context) (dentry.Entry, error) {
	m.mux.Lock()
	defer m.mux.Unlock()

	entry, ok := m.entries[dentry.RootEntryID]
	if ok {
		return entry, nil
	}

	acc := types.Access{
		Permissions: []types.Permission{
			types.PermOwnerRead,
			types.PermOwnerWrite,
			types.PermOwnerExec,
			types.PermGroupRead,
			types.PermGroupWrite,
			types.PermOthersRead,
		},
	}
	root, _ := types.InitNewObject(nil, types.ObjectAttr{Name: "root", Kind: types.GroupKind, Access: acc})
	root.ID = dentry.RootEntryID
	root.ParentID = root.ID

	m.entries[dentry.RootEntryID] = dentry.BuildEntry(root, nil)
	return m.entries[dentry.RootEntryID], nil
}

func (m *MockController) FindEntry(ctx context.Context, parent dentry.Entry, name string) (dentry.Entry, error) {
	m.mux.Lock()
	defer m.mux.Unlock()

	var result dentry.Entry
	for oid, en := range m.entries {
		if en.Metadata().ParentID != parent.Metadata().ID {
			continue
		}
		if en.Metadata().Name == name {
			result = m.entries[oid]
			break
		}
	}

	if result == nil {
		return nil, types.ErrNotFound
	}

	return result, nil
}

func (m *MockController) CreateEntry(ctx context.Context, parent dentry.Entry, attr types.ObjectAttr) (dentry.Entry, error) {
	entry, err := types.InitNewObject(parent.Object(), attr)
	if err != nil {
		return nil, err
	}

	return dentry.BuildEntry(entry, nil), m.SaveEntry(ctx, parent, dentry.BuildEntry(entry, nil))
}

func (m *MockController) SaveEntry(ctx context.Context, parent, entry dentry.Entry) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	entry.Metadata().ID = utils.GenerateNewID()
	m.entries[entry.Metadata().ID] = entry
	return nil
}

func (m *MockController) DestroyEntry(ctx context.Context, parent, entry dentry.Entry, attr types.DestroyObjectAttr) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	delete(m.entries, entry.Metadata().ID)
	return nil
}

func (m *MockController) MirrorEntry(ctx context.Context, src, dstParent dentry.Entry, attr types.ObjectAttr) (dentry.Entry, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockController) ListEntryChildren(ctx context.Context, entry dentry.Entry) ([]dentry.Entry, error) {
	result := make([]dentry.Entry, 0)
	for oid, e := range m.entries {
		if e.Metadata().ParentID != entry.Metadata().ID {
			continue
		}
		if e.Metadata().ID == e.Metadata().ParentID {
			continue
		}
		result = append(result, m.entries[oid])
	}
	return result, nil
}

func (m *MockController) DeleteFileData(ctx context.Context, entry dentry.Entry) error {
	//TODO implement me
	panic("implement me")
}

func (m *MockController) FsInfo(ctx context.Context) controller.Info {
	return controller.Info{}
}

func NewMockController() controller.Controller {
	return &MockController{
		entries: map[int64]dentry.Entry{},
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

func mustGetNanaEntry(node *NanaNode, ctrl controller.Controller) dentry.Entry {
	result, err := ctrl.GetEntry(context.Background(), node.oid)
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

	s, _ := storage.NewStorage(storage.MemoryStorage, storage.MemoryStorage, config.Storage{})
	files.InitFileIoChain(cfg, s, make(chan struct{}))
	RegisterFailHandler(Fail)
	RunSpecs(t, "Fs Suite")
}
