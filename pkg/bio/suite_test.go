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

package bio

import (
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/utils/logger"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	chunkStore metastore.ChunkStore
	entryStore metastore.DEntry
	dataStore  storage.Storage

	workdir string
)

func TestBIO(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()
	RegisterFailHandler(Fail)

	var err error
	workdir, err = os.MkdirTemp(os.TempDir(), "ut-nanafs-bio-")
	Expect(err).Should(BeNil())
	t.Logf("unit test workdir on: %s", workdir)
	storage.InitLocalCache(config.Config{CacheDir: workdir, CacheSize: 1})

	memMeta, err := metastore.NewMetaStorage(storage.MemoryStorage, config.Meta{})
	Expect(err).Should(BeNil())
	chunkStore = memMeta
	entryStore = memMeta

	memData, err := storage.NewStorage(storage.MemoryStorage, storage.MemoryStorage, config.Storage{})
	Expect(err).Should(BeNil())
	dataStore = memData

	RunSpecs(t, "BIO Suite")
}
