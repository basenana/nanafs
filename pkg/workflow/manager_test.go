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
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("TestWorkflowManage", func() {
	var (
		ctx = context.TODO()
		wf  = &types.WorkflowSpec{
			Name: "test-create-workflow-1",
			Rule: types.Rule{},
			Steps: []types.WorkflowStepSpec{
				{
					Name: "step-1",
					Plugin: &types.PlugScope{
						PluginName: "delay",
						Version:    "1.0",
						Action:     "delay",
						PluginType: types.TypeProcess,
						Parameters: map[string]string{"delay": "1s"},
					},
				},
			},
		}
	)
	Context("create a workflow", func() {
		It("should be succeed", func() {
			var err error
			wf, err = mgr.CreateWorkflow(ctx, wf)
			Expect(err).Should(BeNil())
			Expect(wf.Id).ShouldNot(BeEmpty())
		})
	})

	Context("query workflow", func() {
		It("get workflow should be succeed", func() {
			_, err := mgr.GetWorkflow(ctx, wf.Id)
			Expect(err).Should(BeNil())
		})
		It("list workflow should be succeed", func() {
			wfList, err := mgr.ListWorkflows(ctx)
			Expect(err).Should(BeNil())
			Expect(len(wfList) > 0).Should(BeTrue())
		})
	})

	Context("update a workflow", func() {
		var old *types.WorkflowSpec
		It("get workflow should be succeed", func() {
			var err error
			old, err = mgr.GetWorkflow(ctx, wf.Id)
			Expect(err).Should(BeNil())
		})
		It("update should be succeed", func() {
			old.Name = "test-update-workflow-1"
			newWf, err := mgr.UpdateWorkflow(ctx, old)
			Expect(err).Should(BeNil())
			Expect(newWf.Id).Should(Equal(old.Id))
			Expect(newWf.Name).Should(Equal("test-update-workflow-1"))
		})
	})

	Context("delete workflow", func() {
		It("delete workflow should be succeed", func() {
			Expect(mgr.DeleteWorkflow(ctx, wf.Id)).Should(BeNil())
		})
		It("query deleted workflow should be error", func() {
			_, err := mgr.GetWorkflow(ctx, wf.Id)
			Expect(err).Should(Equal(types.ErrNotFound))
		})
	})
})

var _ = Describe("TestWorkflowJobManage", func() {
	var (
		ctx = context.TODO()
		ps  = &types.PlugScope{
			PluginName: "delay",
			Version:    "1.0",
			PluginType: types.TypeProcess,
			Action:     "delay",
			Parameters: map[string]string{"delay": "1s"},
		}
		wf = &types.WorkflowSpec{
			Name: "test-trigger-workflow-1",
			Rule: types.Rule{},
			Steps: []types.WorkflowStepSpec{
				{Name: "step-1", Plugin: ps},
				{Name: "step-2", Plugin: ps},
			},
		}
		en dentry.Entry
	)
	Context("trigger a workflow", func() {
		It("create dummy entry should be succeed", func() {
			root, err := entryMgr.Root(ctx)
			Expect(err).Should(BeNil())
			en, err = entryMgr.CreateEntry(ctx, root, dentry.EntryAttr{Name: "test_workflow.txt", Kind: types.RawKind, Access: root.Metadata().Access})
			Expect(err).Should(BeNil())

			f, err := entryMgr.Open(ctx, en, dentry.Attr{Write: true})
			Expect(err).Should(BeNil())
			_, err = f.WriteAt(ctx, []byte("Hello World!"), 0)
			Expect(err).Should(BeNil())
			Expect(f.Close(ctx)).Should(BeNil())
		})
		It("create workflow should be succeed", func() {
			var err error
			wf, err = mgr.CreateWorkflow(ctx, wf)
			Expect(err).Should(BeNil())
			Expect(wf.Id).ShouldNot(BeEmpty())
		})
	})

	Context("trigger a workflow", func() {
		var job *types.WorkflowJob
		It("trigger workflow should be succeed", func() {
			var err error
			job, err = mgr.TriggerWorkflow(ctx, wf.Id, en.Metadata().ID, JobAttr{})
			Expect(err).Should(BeNil())
			Expect(job.Id).ShouldNot(BeEmpty())
		})

		It("job should be succeed", func() {
			Eventually(func() string {
				jobList, err := mgr.ListJobs(ctx, wf.Id)
				Expect(err).Should(BeNil())

				for _, j := range jobList {
					if j.Id == job.Id {
						return j.Status
					}
				}
				return ""
			}, time.Minute, time.Second).Should(Equal(string(flow.SucceedStatus)))
		})
	})

	Context("pause workflow job", func() {
		var job *types.WorkflowJob
		It("trigger workflow should be succeed", func() {
			var err error
			job, err = mgr.TriggerWorkflow(ctx, wf.Id, en.Metadata().ID, JobAttr{})
			Expect(err).Should(BeNil())
			Expect(job.Id).ShouldNot(BeEmpty())

			Eventually(func() string {
				jobList, err := mgr.ListJobs(ctx, wf.Id)
				Expect(err).Should(BeNil())

				for _, j := range jobList {
					if j.Id == job.Id {
						return j.Status
					}
				}
				return ""
			}, time.Minute, time.Second).Should(Equal(string(flow.RunningStatus)))
		})

		It("pause job should be succeed", func() {
			Expect(mgr.PauseWorkflowJob(ctx, job.Id)).Should(BeNil())
			Eventually(func() string {
				jobList, err := mgr.ListJobs(ctx, wf.Id)
				Expect(err).Should(BeNil())

				for _, j := range jobList {
					if j.Id == job.Id {
						return j.Status
					}
				}
				return ""
			}, time.Minute, time.Second).Should(Equal(string(flow.PausedStatus)))
		})

		It("resume job should be succeed", func() {
			Expect(mgr.ResumeWorkflowJob(ctx, job.Id)).Should(BeNil())
			Eventually(func() string {
				jobList, err := mgr.ListJobs(ctx, wf.Id)
				Expect(err).Should(BeNil())

				for _, j := range jobList {
					if j.Id == job.Id {
						return j.Status
					}
				}
				return ""
			}, time.Minute, time.Second).Should(Equal(string(flow.RunningStatus)))
		})

		It("job should be succeed", func() {
			Eventually(func() string {
				jobList, err := mgr.ListJobs(ctx, wf.Id)
				Expect(err).Should(BeNil())

				for _, j := range jobList {
					if j.Id == job.Id {
						return j.Status
					}
				}
				return ""
			}, time.Minute, time.Second).Should(Equal(string(flow.SucceedStatus)))
		})
	})

	Context("cancel workflow job", func() {
		var job *types.WorkflowJob
		It("trigger workflow should be succeed", func() {
			var err error
			job, err = mgr.TriggerWorkflow(ctx, wf.Id, en.Metadata().ID, JobAttr{})
			Expect(err).Should(BeNil())
			Expect(job.Id).ShouldNot(BeEmpty())

			Eventually(func() string {
				jobList, err := mgr.ListJobs(ctx, wf.Id)
				Expect(err).Should(BeNil())

				for _, j := range jobList {
					if j.Id == job.Id {
						return j.Status
					}
				}
				return ""
			}, time.Minute, time.Second).Should(Equal(string(flow.RunningStatus)))
		})

		It("pause job should be succeed", func() {
			Expect(mgr.PauseWorkflowJob(ctx, job.Id)).Should(BeNil())
			Eventually(func() string {
				jobList, err := mgr.ListJobs(ctx, wf.Id)
				Expect(err).Should(BeNil())

				for _, j := range jobList {
					if j.Id == job.Id {
						return j.Status
					}
				}
				return ""
			}, time.Minute, time.Second).Should(Equal(string(flow.PausedStatus)))
		})

		It("cancel job should be succeed", func() {
			Expect(mgr.CancelWorkflowJob(ctx, job.Id)).Should(BeNil())
			Eventually(func() string {
				jobList, err := mgr.ListJobs(ctx, wf.Id)
				Expect(err).Should(BeNil())

				for _, j := range jobList {
					if j.Id == job.Id {
						return j.Status
					}
				}
				return ""
			}, time.Minute, time.Second).Should(Equal(string(flow.CanceledStatus)))
		})
	})
})
