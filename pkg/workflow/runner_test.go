package workflow

import (
	"context"
	"github.com/basenana/go-flow/flow"
	"github.com/basenana/nanafs/pkg/plugin/common"
	"github.com/basenana/nanafs/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("TestWorkflowTrigger", func() {
	ps := types.PlugScope{
		PluginName: "test-process-plugin-succeed",
		Version:    "1.0",
		PluginType: types.TypeProcess,
		Parameters: map[string]string{},
	}
	caller.mockResponse(ps, func() (*common.Response, error) { return &common.Response{IsSucceed: true}, nil })

	Context("with a single workflow", func() {
		It("should be succeed", func() {
			job, err := runner.WorkFlowHandler(context.TODO(), &types.WorkflowSpec{
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

			Eventually(func() string {
				f, err := runner.GetFlow(flow.FID(job.Id))
				Expect(err).Should(BeNil())
				return string(f.GetStatus())
			}, time.Minute, time.Second).Should(Equal(string(flow.SucceedStatus)))
		})
	})
})
