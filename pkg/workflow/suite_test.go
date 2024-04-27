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
	"context"
	"github.com/basenana/nanafs/pkg/plugin/buildin"
	"os"
	"testing"
	"time"

	"github.com/basenana/nanafs/pkg/rule"

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

	memMeta, err := metastore.NewMetaStorage(metastore.MemoryMeta, config.Meta{})
	Expect(err).Should(BeNil())

	rule.InitQuery(memMeta)

	storage.InitLocalCache(config.Bootstrap{CacheDir: tempDir, CacheSize: 1})
	entryMgr, err = dentry.NewManager(memMeta, config.Bootstrap{
		FS: &config.FS{},
		Storages: []config.Storage{{
			ID:   storage.MemoryStorage,
			Type: storage.MemoryStorage,
		}},
	})
	Expect(err).Should(BeNil())

	cfg := config.NewFakeConfigLoader(config.Bootstrap{})
	err = cfg.SetSystemConfig(context.TODO(), config.WorkflowConfigGroup, "enable", true)
	Expect(err).Should(BeNil())
	err = cfg.SetSystemConfig(context.TODO(), config.WorkflowConfigGroup, "job_workdir", tempDir)
	Expect(err).Should(BeNil())

	docMgr, err = document.NewManager(memMeta, entryMgr, cfg)
	Expect(err).Should(BeNil())

	// init plugin
	Expect(plugin.Init(buildin.Services{}, cfg)).Should(BeNil())

	mgr, err = NewManager(entryMgr, docMgr, notify.NewNotify(memMeta), memMeta, cfg)
	Expect(err).Should(BeNil())

	go mgr.Start(stopCh)
})

var _ = AfterSuite(func() {
	close(stopCh)
})
