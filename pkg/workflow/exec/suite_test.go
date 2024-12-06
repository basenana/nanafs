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

package exec

import (
	"context"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/plugin/buildin"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	tempDir  string
	targetID int64
	entryMgr dentry.Manager

	loCfg = Config{}
)

func TestExec(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Workflow Exec Suite")
}

var _ = BeforeSuite(func() {
	var err error
	tempDir, err = os.MkdirTemp(os.TempDir(), "ut-nanafs-exec-")
	Expect(err).Should(BeNil())

	loCfg.Enable = true
	loCfg.JobWorkdir = tempDir

	memMeta, err := metastore.NewMetaStorage(metastore.MemoryMeta, config.Meta{})
	Expect(err).Should(BeNil())

	storage.InitLocalCache(config.Bootstrap{CacheDir: tempDir, CacheSize: 1})
	entryMgr, err = dentry.NewManager(memMeta, config.Bootstrap{
		FS: &config.FS{},
		Storages: []config.Storage{{
			ID:   storage.MemoryStorage,
			Type: storage.MemoryStorage,
		}},
	})
	Expect(err).Should(BeNil())

	// init mocked file
	root, err := entryMgr.Root(context.TODO())
	Expect(err).Should(BeNil())
	txtFile, err := entryMgr.CreateEntry(context.TODO(), root.ID, types.EntryAttr{
		Name: "target.txt", Kind: types.RawKind})
	Expect(err).Should(BeNil())
	targetID = txtFile.ID

	f, err := entryMgr.Open(context.TODO(), txtFile.ID, types.OpenAttr{Write: true})
	Expect(err).Should(BeNil())
	_, err = f.WriteAt(context.TODO(), []byte("Hello World!"), 0)
	Expect(err).Should(BeNil())
	Expect(f.Close(context.TODO())).Should(BeNil())

	// init plugin
	cfgLoader := config.NewFakeConfigLoader(config.Bootstrap{})
	Expect(plugin.Init(buildin.Services{}, cfgLoader)).Should(BeNil())
})
