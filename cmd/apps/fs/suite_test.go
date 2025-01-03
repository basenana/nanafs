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
	"testing"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/friday"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
)

var (
	cfg = config.Bootstrap{
		FS: &config.FS{Owner: config.FSOwner{Uid: 0, Gid: 0}},
		Storages: []config.Storage{
			{ID: storage.MemoryStorage, Type: storage.MemoryStorage},
		},
	}
)

func NewMockController() controller.Controller {
	m, _ := metastore.NewMetaStorage(metastore.MemoryMeta, config.Meta{})
	ctrl, _ := controller.New(config.NewFakeConfigLoader(cfg), m, friday.NewMockFriday())
	return ctrl
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

func mustGetNanaEntry(node *NanaNode, ctrl controller.Controller) *types.Entry {
	result, err := ctrl.GetEntry(context.Background(), node.entryID)
	if err != nil {
		panic(err)
	}
	return result
}

func TestFs(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Fs Suite")
}
