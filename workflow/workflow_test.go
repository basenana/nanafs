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
	"time"

	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/workflow/jobrun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TestWorkflowManage", func() {
	var (
		ctx = context.TODO()
		wf  = &types.Workflow{
			Name:      "test-create-workflow-1",
			Namespace: namespace,
			Nodes: []types.WorkflowNode{
				{
					Name:  "step-1",
					Type:  "delay",
					Input: map[string]any{"delay": "5s"},
				},
			},
			QueueName: types.WorkflowQueueFile,
		}
	)
	Context("create a workflow", func() {
		It("should be succeed", func() {
			var err error
			wf, err = mgr.CreateWorkflow(ctx, namespace, wf)
			Expect(err).Should(BeNil())
			Expect(wf.Id).ShouldNot(BeEmpty())
		})
	})

	Context("query workflow", func() {
		It("get workflow should be succeed", func() {
			_, err := mgr.GetWorkflow(ctx, namespace, wf.Id)
			Expect(err).Should(BeNil())
		})
		It("list workflow should be succeed", func() {
			wfList, err := mgr.ListWorkflows(ctx, namespace)
			Expect(err).Should(BeNil())
			Expect(len(wfList) > 0).Should(BeTrue())
		})
	})

	Context("update a workflow", func() {
		var old *types.Workflow
		It("get workflow should be succeed", func() {
			var err error
			old, err = mgr.GetWorkflow(ctx, namespace, wf.Id)
			Expect(err).Should(BeNil())
		})
		It("update should be succeed", func() {
			old.Name = "test-update-workflow-1"
			newWf, err := mgr.UpdateWorkflow(ctx, namespace, old)
			Expect(err).Should(BeNil())
			Expect(newWf.Id).Should(Equal(old.Id))
			Expect(newWf.Name).Should(Equal("test-update-workflow-1"))
		})
	})

	Context("delete workflow", func() {
		It("delete workflow should be succeed", func() {
			err := mgr.DeleteWorkflow(ctx, namespace, wf.Id)
			Expect(err).Should(BeNil())
		})
		It("query deleted workflow should be error", func() {
			_, err := mgr.GetWorkflow(ctx, namespace, wf.Id)
			Expect(err).Should(Equal(types.ErrNotFound))
		})
	})
})

var _ = Describe("TestWorkflowJobManage", func() {
	var (
		ctx = context.TODO()
		wf  = &types.Workflow{
			Name:      "test-trigger-workflow-1",
			Namespace: namespace,
			Nodes: []types.WorkflowNode{
				{
					Name:  "step-1",
					Type:  "delay",
					Input: map[string]any{"delay": "2s"},
					Next:  "step-2",
				},
				{
					Name:  "step-2",
					Type:  "delay",
					Input: map[string]any{"delay": "2s"},
				},
			},
			QueueName: types.WorkflowQueueFile,
		}
		en    *types.Entry
		enURI = "/test_workflow.txt"
	)
	Context("trigger a workflow", func() {
		It("create dummy entry should be succeed", func() {
			var err error
			en, err = fsCore.CreateEntry(ctx, namespace, "/", types.EntryAttr{Name: "test_workflow.txt", Kind: types.RawKind})
			Expect(err).Should(BeNil())

			f, err := fsCore.Open(ctx, namespace, en.ID, types.OpenAttr{Write: true})
			Expect(err).Should(BeNil())
			_, err = f.WriteAt(ctx, []byte("Hello World!"), 0)
			Expect(err).Should(BeNil())
			Expect(f.Close(ctx)).Should(BeNil())
		})
		It("create workflow should be succeed", func() {
			var err error
			wf, err = mgr.CreateWorkflow(ctx, namespace, wf)
			Expect(err).Should(BeNil())
			Expect(wf.Id).ShouldNot(BeEmpty())
		})
	})

	Context("trigger a workflow", func() {
		var jobID string
		It("trigger workflow should be succeed", func() {
			job, err := mgr.TriggerWorkflow(ctx, namespace, wf.Id, types.WorkflowTarget{Entries: []string{enURI}}, JobAttr{})
			Expect(err).Should(BeNil())
			Expect(job.Id).ShouldNot(BeEmpty())
			jobID = job.Id
		})

		It("job should be succeed", func() {
			Eventually(func() string {
				jobList, err := mgr.ListJobs(ctx, namespace, wf.Id)
				Expect(err).Should(BeNil())

				for _, j := range jobList {
					if j.Id == jobID {
						return j.Status
					}
				}
				return ""
			}, time.Minute, time.Second).Should(Equal(jobrun.SucceedStatus))
		})
	})

	Context("pause workflow job", func() {
		var jobID string
		It("trigger workflow should be succeed", func() {
			job, err := mgr.TriggerWorkflow(ctx, namespace, wf.Id, types.WorkflowTarget{Entries: []string{enURI}}, JobAttr{})
			Expect(err).Should(BeNil())
			Expect(job.Id).ShouldNot(BeEmpty())
			jobID = job.Id

			Eventually(func() string {
				jobList, err := mgr.ListJobs(ctx, namespace, wf.Id)
				Expect(err).Should(BeNil())

				for _, j := range jobList {
					if j.Id == job.Id {
						return j.Status
					}
				}
				return ""
			}, time.Minute, time.Second).Should(Equal(string(jobrun.RunningStatus)))
		})

		It("pause job should be succeed", func() {
			Expect(mgr.PauseWorkflowJob(ctx, namespace, jobID)).Should(BeNil())
			Eventually(func() string {
				jobList, err := mgr.ListJobs(ctx, namespace, wf.Id)
				Expect(err).Should(BeNil())

				for _, j := range jobList {
					if j.Id == jobID {
						return j.Status
					}
				}
				return ""
			}, time.Minute, time.Second).Should(Equal(string(jobrun.PausedStatus)))
		})

		It("resume job should be succeed", func() {
			Expect(mgr.ResumeWorkflowJob(ctx, namespace, jobID)).Should(BeNil())
			Eventually(func() string {
				jobList, err := mgr.ListJobs(ctx, namespace, wf.Id)
				Expect(err).Should(BeNil())

				for _, j := range jobList {
					if j.Id == jobID {
						return j.Status
					}
				}
				return ""
			}, time.Minute, time.Second).Should(Equal(string(jobrun.RunningStatus)))
		})

		It("job should be succeed", func() {
			Eventually(func() string {
				jobList, err := mgr.ListJobs(ctx, namespace, wf.Id)
				Expect(err).Should(BeNil())

				for _, j := range jobList {
					if j.Id == jobID {
						return j.Status
					}
				}
				return ""
			}, time.Minute, time.Second).Should(Equal(string(jobrun.SucceedStatus)))
		})
	})

	Context("cancel workflow job", func() {
		var job *types.WorkflowJob
		It("trigger workflow should be succeed", func() {
			var err error
			job, err = mgr.TriggerWorkflow(ctx, namespace, wf.Id, types.WorkflowTarget{Entries: []string{enURI}}, JobAttr{})
			Expect(err).Should(BeNil())
			Expect(job.Id).ShouldNot(BeEmpty())

			Eventually(func() string {
				jobList, err := mgr.ListJobs(ctx, namespace, wf.Id)
				Expect(err).Should(BeNil())

				for _, j := range jobList {
					if j.Id == job.Id {
						return j.Status
					}
				}
				return ""
			}, time.Minute, time.Second).Should(Equal(string(jobrun.RunningStatus)))
		})

		It("pause job should be succeed", func() {
			Expect(mgr.PauseWorkflowJob(ctx, namespace, job.Id)).Should(BeNil())
			Eventually(func() string {
				jobList, err := mgr.ListJobs(ctx, namespace, wf.Id)
				Expect(err).Should(BeNil())

				for _, j := range jobList {
					if j.Id == job.Id {
						return j.Status
					}
				}
				return ""
			}, time.Minute, time.Second).Should(Equal(string(jobrun.PausedStatus)))
		})

		It("cancel job should be succeed", func() {
			Expect(mgr.CancelWorkflowJob(ctx, namespace, job.Id)).Should(BeNil())
			Eventually(func() string {
				jobList, err := mgr.ListJobs(ctx, namespace, wf.Id)
				Expect(err).Should(BeNil())

				for _, j := range jobList {
					if j.Id == job.Id {
						return j.Status
					}
				}
				return ""
			}, time.Minute, time.Second).Should(Equal(string(jobrun.CanceledStatus)))
		})
	})
})
