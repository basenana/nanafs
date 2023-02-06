package plugin

import (
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/plugin/buildin"
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
	// init plugin registry
	Expect(Init(config.Config{})).Should(BeNil())

	// register dummy plugin
	dummySourcePlugin := buildin.InitDummySourcePlugin()
	dummyProcessPlugin := buildin.InitDummyProcessPlugin()
	dummyMirrorPlugin := buildin.InitDummyMirrorPlugin()

	pluginRegistry.mux.Lock()
	pluginRegistry.plugins[dummySourcePlugin.Name()] = &pluginInfo{
		Plugin:  dummySourcePlugin,
		buildIn: true,
	}
	pluginRegistry.plugins[dummyProcessPlugin.Name()] = &pluginInfo{
		Plugin:  dummyProcessPlugin,
		buildIn: true,
	}
	pluginRegistry.plugins[dummyMirrorPlugin.Name()] = &pluginInfo{
		Plugin:  dummyMirrorPlugin,
		buildIn: true,
	}
	pluginRegistry.mux.Unlock()
})
