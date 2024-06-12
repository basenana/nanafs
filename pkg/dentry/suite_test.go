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
	"context"
	"os"
	"testing"

	"github.com/basenana/nanafs/pkg/plugin/buildin"
	"github.com/basenana/nanafs/pkg/rule"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	metaStoreObj metastore.Meta
	entryManager Manager
	entryMgr     *manager
	root         *types.Metadata

	workdir string
)

func TestDEntry(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()
	RegisterFailHandler(Fail)

	var err error
	workdir, err = os.MkdirTemp(os.TempDir(), "ut-nanafs-dentry-")
	Expect(err).Should(BeNil())
	t.Logf("unit test workdir on: %s", workdir)
	storage.InitLocalCache(config.Bootstrap{CacheDir: workdir, CacheSize: 1})

	RunSpecs(t, "DEntry Suite")
}

var _ = BeforeSuite(func() {
	memMeta, err := metastore.NewMetaStorage(metastore.MemoryMeta, config.Meta{})
	Expect(err).Should(BeNil())
	metaStoreObj = memMeta
	entryManager, _ = NewManager(metaStoreObj, config.Bootstrap{FS: &config.FS{}, Storages: []config.Storage{{
		ID:   storage.MemoryStorage,
		Type: storage.MemoryStorage,
	}}})
	storages := make(map[string]storage.Storage)
	storages[storage.MemoryStorage], err = storage.NewStorage(
		storage.MemoryStorage,
		storage.MemoryStorage,
		config.Storage{ID: storage.MemoryStorage, Type: storage.MemoryStorage},
	)
	Expect(err).Should(BeNil())

	// init rule based query
	rule.InitQuery(memMeta)

	entryMgr = &manager{
		store:     metaStoreObj,
		metastore: metaStoreObj,
		storages:  storages,
		eventQ:    make(chan *entryEvent, 8),
		logger:    logger.NewLogger("entryManager"),
	}

	// init root
	root, err = entryManager.Root(context.TODO())
	Expect(err).Should(BeNil())

	cfgLoader := config.NewFakeConfigLoader(config.Bootstrap{})

	// init plugin
	err = plugin.Init(buildin.Services{}, cfgLoader)
	Expect(err).Should(BeNil())
})
