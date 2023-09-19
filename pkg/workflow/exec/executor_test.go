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

package exec

import (
	"context"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/pkg/workflow/jobrun"
	"github.com/basenana/nanafs/utils/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	"time"
)

var _ = Describe("TestLocalExecutor", func() {
	var (
		ctx  = context.TODO()
		step = types.WorkflowJobStep{
			StepName: "delay 1s",
			Plugin: &types.PlugScope{
				PluginName: "delay",
				Version:    "1.0",
				PluginType: types.TypeProcess,
				Action:     "delay",
				Parameters: map[string]string{"delay": time.Second.String()},
			},
		}
		job      *types.WorkflowJob
		executor jobrun.Executor
	)

	Context("create a workflow executor", func() {
		It("init executir should be succeed", func() {
			job = &types.WorkflowJob{
				Id:       "test-executor-1",
				Workflow: "mock-workflow-1",
				Target:   types.WorkflowTarget{EntryID: targetID},
				Steps:    []types.WorkflowJobStep{step},
			}

			executor = &localExecutor{
				job: job, entryMgr: entryMgr, config: loCfg,
				logger: logger.NewLogger("localExecutor").With(zap.String("job", job.Id)),
			}
		})
		It("setup should be succeed", func() {
			Expect(executor.Setup(ctx)).Should(BeNil())
		})
	})

	Context("exec workflow", func() {
		It("exec should be succeed", func() {
			Expect(executor.DoOperation(ctx, step)).Should(BeNil())
		})
	})

	Context("finish workflow job exec", func() {
		It("teardown should be succeed", func() {
			executor.Teardown(ctx)
		})
	})
})
