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
	"os"
	"testing"
	"time"

	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/types"

	testcfg "github.com/onsi/ginkgo/config"

	"github.com/basenana/nanafs/config"
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
	mgr       Workflow
	namespace = types.DefaultNamespace

	bootCfg = config.Bootstrap{
		FS: &config.FS{},
		Storages: []config.Storage{{
			ID:   storage.MemoryStorage,
			Type: storage.MemoryStorage,
		}},
		Workflow: config.Workflow{
			Enable: true,
		},
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
	bootCfg.CacheDir = tempDir
	bootCfg.CacheSize = 0
	bootCfg.Workflow.JobWorkdir = tempDir

	memMeta, err := metastore.NewMetaStorage(metastore.MemoryMeta, config.Meta{})
	Expect(err).Should(BeNil())

	fsCore, err = core.New(memMeta, bootCfg)
	Expect(err).Should(BeNil())

	err = fsCore.CreateNamespace(context.Background(), namespace)
	Expect(err).Should(BeNil())

	mgr, err = New(fsCore, notify.NewNotify(memMeta), memMeta, bootCfg.Workflow)
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
