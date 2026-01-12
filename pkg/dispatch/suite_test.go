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

package dispatch

import (
	"context"
	"testing"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/notify"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/basenana/nanafs/workflow"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	namespace  = types.DefaultNamespace
	testMeta   metastore.Meta
	testNotify *notify.Notify

	fsCore      core.Core
	workflowMgr workflow.Workflow

	bootCfg = config.Bootstrap{
		FS: &config.FS{},
		Storages: []config.Storage{{
			ID:   storage.MemoryStorage,
			Type: storage.MemoryStorage,
		}},
		Workflow: config.Workflow{
			JobWorkdir: "/tmp",
		},
	}
)

func TestDispatch(t *testing.T) {
	RegisterFailHandler(Fail)

	logger.InitLogger()
	defer logger.Sync()
	logger.SetDebug(true)

	RunSpecs(t, "Dispatch Suite")
}

var _ = BeforeEach(func() {
	// Create fresh memory meta for each test
	memMeta, err := metastore.NewMetaStorage(metastore.MemoryMeta, config.Meta{})
	Expect(err).Should(BeNil())
	testMeta = memMeta

	fsCore, err = core.New(memMeta, bootCfg)
	Expect(err).Should(BeNil())

	err = fsCore.CreateNamespace(context.Background(), namespace)
	Expect(err).Should(BeNil())

	testNotify = notify.NewNotify(testMeta)
	workflowMgr, err = workflow.New(fsCore, testNotify, memMeta, bootCfg.Workflow)
	Expect(err).Should(BeNil())
})
