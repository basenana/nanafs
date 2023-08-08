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
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	objStore     metastore.ObjectStore
	entryManager Manager
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
	storage.InitLocalCache(config.Config{CacheDir: workdir, CacheSize: 1})

	RunSpecs(t, "DEntry Suite")
}

var _ = BeforeSuite(func() {
	memMeta, err := metastore.NewMetaStorage(storage.MemoryStorage, config.Meta{})
	Expect(err).Should(BeNil())
	objStore = memMeta
	entryManager, _ = NewManager(objStore, config.Config{FS: &config.FS{}, Storages: []config.Storage{{
		ID:   storage.MemoryStorage,
		Type: storage.MemoryStorage,
	}}})

	// init root
	root, err = entryManager.Root(context.TODO())
	Expect(err).Should(BeNil())

	// init plugin
	err = plugin.Init(&config.Plugin{}, memMeta)
	Expect(err).Should(BeNil())
})
