package plugin

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TestRunOnePlugin", func() {
	var (
		meta   = NewMockMeta()
		stopCh = make(chan struct{})
		r      *runtime
	)
	BeforeEach(func() {
		r = newPluginRuntime(meta, stopCh)
		Expect(nil).Should(BeNil())
	})
	Describe("test source plugin", func() {
		Context("open a file", func() {
			var pRun *run
			It("should be ok", func() {
				pRun = &run{
					id:     "test-dummy-1",
					p:      &dummyPlugin{},
					obj:    nil,
					params: map[string]string{},
					stopCh: make(chan struct{}),
				}
			})
			It("should be ok", func() {
				r.runOnePlugin(pRun)
			})
		})
	})
	close(stopCh)
})
