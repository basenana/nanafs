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

package v1

import (
	"context"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/basenana/nanafs/cmd/apps/apis/rest/common"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
)

var (
	dep       *common.Depends
	testMeta  metastore.Meta
	testCfg   config.Bootstrap
	testRoute *MockRouter
)

type MockRouter struct {
	Entries map[string]*EntryInfo
}

func (m *MockRouter) GetEntry(uri string) *EntryInfo {
	return m.Entries[uri]
}

func TestRestV1API(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()
	RegisterFailHandler(Fail)
	RunSpecs(t, "REST V1 Suite")
}

var _ = BeforeSuite(func() {
	memMeta, err := metastore.NewMetaStorage(metastore.MemoryMeta, config.Meta{})
	Expect(err).Should(BeNil())
	testMeta = memMeta

	workdir, err := os.MkdirTemp(os.TempDir(), "ut-nanafs-rest-")
	Expect(err).Should(BeNil())

	testCfg = config.Bootstrap{
		API:       config.FsApi{Noauth: true},
		FS:        &config.FS{Owner: config.FSOwner{Uid: 0, Gid: 0}, Writeback: false},
		Meta:      config.Meta{Type: metastore.MemoryMeta},
		Storages:  []config.Storage{{ID: "test-memory-0", Type: storage.MemoryStorage}},
		CacheDir:  workdir,
		CacheSize: 0,
	}

	cl := config.NewMockConfigLoader(testCfg)
	dep, err = common.InitDepends(cl, memMeta)
	Expect(err).Should(BeNil())

	// init root
	_, err = dep.Core.FSRoot(context.TODO())
	Expect(err).Should(BeNil())

	// init default namespace
	err = dep.Core.CreateNamespace(context.TODO(), types.DefaultNamespace)
	Expect(err).Should(BeNil())

	testRoute = &MockRouter{
		Entries: make(map[string]*EntryInfo),
	}
})
