package workflow

import (
	"context"
	"github.com/basenana/go-flow/flow"
	"github.com/basenana/nanafs/pkg/types"
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
			job, err := assembleWorkflowJob(&types.WorkflowSpec{
				Name: "test-workflow",
				Rule: types.Rule{},
				Steps: []types.WorkflowStepSpec{{
					Name:   "dummy-1",
					Plugin: ps,
				}},
			})
			if err != nil {
				Expect(err).Should(BeNil())
				return
			}

			go runner.triggerJob(context.TODO(), job)
			Eventually(func() string {
				f, err := runner.GetFlow(job.ID())
				Expect(err).Should(BeNil())
				return string(f.GetStatus())
			}, time.Minute, time.Second).Should(Equal(string(flow.SucceedStatus)))
		})
	})
})
