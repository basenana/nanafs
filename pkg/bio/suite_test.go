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
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	chunkStore storage.ChunkStore
	dataStore  storage.Storage
	fakeObj    *types.Object
)

func TestBIO(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()
	RegisterFailHandler(Fail)
	RunSpecs(t, "BIO Suite")
}

var _ = BeforeSuite(func() {
	memMeta, err := storage.NewMetaStorage(storage.MemoryStorage, config.Meta{})
	Expect(err).Should(BeNil())
	chunkStore = memMeta

	memData, err := storage.NewStorage(storage.MemoryStorage, storage.MemoryStorage, config.Storage{})
	Expect(err).Should(BeNil())
	dataStore = memData

	fakeObj = &types.Object{
		Metadata: types.NewMetadata("test.file", types.RawKind),
	}
})