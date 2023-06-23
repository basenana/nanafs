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
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/plugin/common"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"os"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	stopCh   = make(chan struct{})
	caller   = &pluginCaller{response: map[string]func() (*common.Response, error){}}
	tempDir  string
	entryMgr dentry.Manager
	mgr      Manager
)

func TestWorkflow(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()
	RegisterFailHandler(Fail)
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
	mgr, err = NewManager(entryMgr, memMeta, config.Workflow{Enable: true, JobWorkdir: tempDir})
	Expect(err).Should(BeNil())

	pluginCall = caller.call
})

var _ = AfterSuite(func() {
	close(stopCh)
})

type pluginCaller struct {
	response map[string]func() (*common.Response, error)
	mux      sync.Mutex
}

func (c *pluginCaller) mockResponse(ps types.PlugScope, getter func() (*common.Response, error)) {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.response[ps.PluginName] = getter
}

func (c *pluginCaller) call(ctx context.Context, ps types.PlugScope, req *common.Request) (*common.Response, error) {
	c.mux.Lock()
	defer c.mux.Unlock()
	respGetter, ok := c.response[ps.PluginName]
	if !ok {
		return nil, types.ErrNotFound
	}
	return respGetter()
}
