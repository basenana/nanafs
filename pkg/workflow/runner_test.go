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
	"github.com/basenana/go-flow/flow"
	"github.com/basenana/nanafs/pkg/plugin/common"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/google/uuid"
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
	WID := uuid.New().String()

	Context("with a single workflow", func() {
		It("should be succeed", func() {
			job, err := assembleWorkflowJob(&types.WorkflowSpec{
				Id:   WID,
				Name: "test-workflow",
				Rule: types.Rule{},
				Steps: []types.WorkflowStepSpec{{
					Name:   "dummy-1",
					Plugin: ps,
				}},
			}, 0)
			Expect(err).Should(BeNil())

			runner.triggerJob(context.TODO(), job)
			Eventually(func() string {
				jobs, err := mgr.ListJobs(context.TODO(), job.Workflow)
				Expect(err).Should(BeNil())
				for _, j := range jobs {
					if j.Id == job.Id {
						return j.Status
					}
				}
				return ""
			}, time.Minute, time.Second).Should(Equal(string(flow.SucceedStatus)))
		})
	})
})
