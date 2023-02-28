package workflow

import (
	"context"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/plugin/common"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	stopCh = make(chan struct{})
	caller = &pluginCaller{response: map[string]func() (*common.Response, error){}}
	runner *Runner
	mgr    Manager
)

func TestWorkflow(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Workflow Suite")
}

var _ = BeforeSuite(func() {
	memMeta, err := storage.NewMetaStorage(storage.MemoryStorage, config.Meta{})
	Expect(err).Should(BeNil())
	mgr, err = NewManager(memMeta.PluginRecorder(types.PlugScope{}))
	Expect(err).Should(BeNil())

	runner = mgr.(*manager).runner
	Expect(runner.Start(stopCh)).Should(BeNil())

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
