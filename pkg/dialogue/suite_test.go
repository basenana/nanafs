/*
  Copyright 2024 NanaFS Authors.

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

package dialogue

import (
	"context"
	"os"
	"testing"

	"github.com/basenana/nanafs/pkg/plugin/buildin"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	roomManager *manager
	entryMgr    dentry.Manager

	workdir string
	root    *types.Metadata
)

func TestDocument(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()
	RegisterFailHandler(Fail)

	var err error
	workdir, err = os.MkdirTemp(os.TempDir(), "ut-nanafs-room-")
	Expect(err).Should(BeNil())
	t.Logf("unit test workdir on: %s", workdir)
	storage.InitLocalCache(config.Bootstrap{CacheDir: workdir, CacheSize: 1})

	RunSpecs(t, "Room Suite")
}

var _ = BeforeSuite(func() {
	memMeta, err := metastore.NewMetaStorage(metastore.MemoryMeta, config.Meta{})
	Expect(err).Should(BeNil())
	entryMgr, err = dentry.NewManager(memMeta, config.Bootstrap{
		FS: &config.FS{},
		Storages: []config.Storage{{
			ID:   storage.MemoryStorage,
			Type: storage.MemoryStorage,
		}},
	})
	Expect(err).Should(BeNil())
	roomManager = &manager{
		logger:   logger.NewLogger("room"),
		recorder: memMeta,
		entryMgr: entryMgr,
	}

	cfgLoader := config.NewFakeConfigLoader(config.Bootstrap{})
	err = plugin.Init(buildin.Services{}, cfgLoader)
	Expect(err).Should(BeNil())

	// init root
	root, err = entryMgr.Root(context.TODO())
	Expect(err).Should(BeNil())
})