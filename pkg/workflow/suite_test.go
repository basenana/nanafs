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

package workflow

import (
	"os"
	"testing"
	"time"

	testcfg "github.com/onsi/ginkgo/config"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/document"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/notify"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/utils/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	stopCh   = make(chan struct{})
	tempDir  string
	entryMgr dentry.Manager
	docMgr   document.Manager
	mgr      Manager
)

func TestWorkflow(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()
	RegisterFailHandler(Fail)
	testcfg.DefaultReporterConfig.SlowSpecThreshold = time.Minute.Seconds()
	RunSpecs(t, "Workflow Suite")
}

var _ = BeforeSuite(func() {
	var err error
	tempDir, err = os.MkdirTemp(os.TempDir(), "ut-nanafs-wf-")
	Expect(err).Should(BeNil())

	memMeta, err := metastore.NewMetaStorage(storage.MemoryStorage, config.Meta{})
	Expect(err).Should(BeNil())

	storage.InitLocalCache(config.Config{CacheDir: tempDir, CacheSize: 1})
	entryMgr, err = dentry.NewManager(memMeta, config.Config{
		FS: &config.FS{},
		Storages: []config.Storage{{
			ID:   storage.MemoryStorage,
			Type: storage.MemoryStorage,
		}},
	})
	Expect(err).Should(BeNil())

	docMgr, err = document.NewManager(memMeta)
	Expect(err).Should(BeNil())

	// init plugin
	Expect(plugin.Init(&config.Plugin{})).Should(BeNil())

	mgr, err = NewManager(entryMgr, docMgr, notify.NewNotify(memMeta), memMeta, config.Workflow{Enable: true, JobWorkdir: tempDir}, config.FUSE{})
	Expect(err).Should(BeNil())
})

var _ = AfterSuite(func() {
	close(stopCh)
})
