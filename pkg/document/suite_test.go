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

package document

import (
	"context"
	"github.com/basenana/nanafs/pkg/core"
	"os"
	"testing"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/friday"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	docManager *manager
	coreMgr    core.Core
	workdir    string
	root       *types.Entry
	namespace  = types.DefaultNamespace
	bootCfg    = config.Bootstrap{
		FS: &config.FS{},
		Storages: []config.Storage{{
			ID:   storage.MemoryStorage,
			Type: storage.MemoryStorage,
		}},
	}
)

func TestDocument(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()
	RegisterFailHandler(Fail)

	var err error
	workdir, err = os.MkdirTemp(os.TempDir(), "ut-nanafs-doc-")
	Expect(err).Should(BeNil())
	t.Logf("unit test workdir on: %s", workdir)
	bootCfg.CacheDir = workdir
	bootCfg.CacheSize = 0

	RunSpecs(t, "Document Suite")
}

var _ = BeforeSuite(func() {
	memMeta, err := metastore.NewMetaStorage(metastore.MemoryMeta, config.Meta{})
	Expect(err).Should(BeNil())

	c, err := core.New(memMeta, bootCfg)
	Expect(err).Should(BeNil())
	docManager = &manager{
		recorder: memMeta,
		core:     c,
		friday:   friday.NewMockFriday(),
		logger:   logger.NewLogger("doc"),
	}
	coreMgr = c

	// init root
	root, err = c.FSRoot(context.TODO())
	Expect(err).Should(BeNil())
})
