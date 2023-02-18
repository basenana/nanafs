package workflow

import (
	"github.com/basenana/nanafs/pkg/types"
	"github.com/hyponet/eventbus/bus"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("TestWorkflowTrigger", func() {
	ps := types.PlugScope{
		PluginName: "dummy-process-plugin",
		Version:    "1.0",
		PluginType: types.TypeProcess,
		Parameters: map[string]string{},
	}

	Context("with a single workflow", func() {
		It("should be succeed", func() {
			bus.Publish("object.workflow.test-workflow.trigger", &types.WorkflowSpec{
				Name: "test-workflow",
				Rule: types.Rule{},
				Steps: []types.WorkflowStepSpec{{
					Name:   "dummy-1",
					Plugin: ps,
				}},
			})
			// TODO:
			Eventually(1, time.Minute, time.Second).Should(Equal(1))
		})
	})
})
