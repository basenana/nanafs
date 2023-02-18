package workflow

import (
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/utils/logger"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var stopCh = make(chan struct{})

func TestWorkflow(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Workflow Suite")
}

var _ = BeforeSuite(func() {
	Expect(plugin.Init(config.Config{Plugin: config.Plugin{DummyPlugins: true}})).Should(BeNil())
	Expect(InitWorkflowRunner(stopCh)).Should(BeNil())
})
var _ = AfterSuite(func() {
	close(stopCh)
})
