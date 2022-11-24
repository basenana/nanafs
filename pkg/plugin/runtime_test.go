package plugin

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/plugin/adaptors"
	"github.com/basenana/nanafs/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sync/atomic"
)

var _ = Describe("TestRunOnePlugin", func() {
	var (
		meta    = adaptors.NewMockMeta()
		stopCh  = make(chan struct{})
		testDir *types.Object
		r       *runtime
	)

	root, err := meta.GetObject(context.TODO(), dentry.RootObjectID)
	Expect(err).Should(BeNil())

	var dirCount int32 = 1
	BeforeEach(func() {
		r = newPluginRuntime(meta, stopCh)
		crt := atomic.LoadInt32(&dirCount)
		for !atomic.CompareAndSwapInt32(&dirCount, crt, crt+1) {
			crt = atomic.LoadInt32(&dirCount)
		}

		testDir, err = types.InitNewObject(root, types.ObjectAttr{
			Name:   fmt.Sprintf("test-run-one-plugin-%d", crt),
			Kind:   types.SmartGroupKind,
			Access: root.Access,
		})
		Expect(err).Should(BeNil())
		Expect(meta.SaveObject(context.TODO(), root, testDir)).Should(BeNil())
	})

	Describe("test source plugin", func() {
		Context("dummy test", func() {
			It("should be ok", func() {
				pRun := &run{
					id:     "test-dummy-1",
					p:      &dummyPlugin{},
					obj:    testDir,
					params: map[string]string{},
					stopCh: make(chan struct{}),
				}
				r.runOnePlugin(pRun)

			})
		})
	})

	go func() {
		close(stopCh)
	}()
	// test runner closeable
	<-stopCh
})
