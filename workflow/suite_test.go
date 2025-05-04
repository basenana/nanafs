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
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/types"
	"os"
	"testing"
	"time"

	"github.com/basenana/nanafs/pkg/friday"
	"github.com/basenana/nanafs/pkg/rule"

	testcfg "github.com/onsi/ginkgo/config"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/document"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/notify"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/utils/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	stopCh    = make(chan struct{})
	tempDir   string
	fsCore    core.Core
	docMgr    document.Manager
	mgr       Workflow
	namespace = types.DefaultNamespace

	bootCfg = config.Bootstrap{
		FS: &config.FS{},
		Storages: []config.Storage{{
			ID:   storage.MemoryStorage,
			Type: storage.MemoryStorage,
		}},
	}
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

	fsCore, err = core.New(memMeta, bootCfg)
	Expect(err).Should(BeNil())

	cfg := config.NewMockConfigLoader(bootCfg)
	err = cfg.SetSystemConfig(context.TODO(), config.WorkflowConfigGroup, "job_workdir", tempDir)
	Expect(err).Should(BeNil())

	docMgr, err = document.NewManager(memMeta, fsCore, cfg, friday.NewMockFriday())
	Expect(err).Should(BeNil())

	mgr, err = New(fsCore, docMgr, notify.NewNotify(memMeta), memMeta, cfg)
	Expect(err).Should(BeNil())

	ctx, canF := context.WithCancel(context.Background())
	go func() {
		<-stopCh
		canF()
	}()

	mgr.Start(ctx)
})

var _ = AfterSuite(func() {
	close(stopCh)
})
