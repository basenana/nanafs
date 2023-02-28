package plugin

import (
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/utils/logger"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestPlugin(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Plugin Suite")
}

var _ = BeforeSuite(func() {
	// init plugin plugins
	mem, _ := storage.NewMetaStorage(storage.MemoryStorage, config.Meta{})
	Expect(Init(config.Config{Plugin: config.Plugin{DummyPlugins: true}}, mem)).Should(BeNil())
})
