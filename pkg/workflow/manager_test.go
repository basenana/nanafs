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

var _ = Describe("TestWorkflowManage", func() {
	wf := &types.WorkflowSpec{
		Name: "test-create-workflow-1",
		Rule: types.Rule{},
		Steps: []types.WorkflowStepSpec{
			{
				Name: "step-1",
				Plugin: types.PlugScope{
					PluginName: "test-process-plugin-succeed",
					Version:    "1.0",
					PluginType: types.TypeProcess,
					Parameters: map[string]string{},
				},
			},
		},
	}

	Context("create a workflow", func() {
		It("should be succeed", func() {
			var err error
			wf, err = mgr.SaveWorkflow(context.TODO(), wf)
			Expect(err).Should(BeNil())
			Expect(wf.Id).ShouldNot(BeEmpty())
		})
	})

	Context("query workflow", func() {
		It("get workflow should be succeed", func() {
			_, err := mgr.GetWorkflow(context.TODO(), wf.Id)
			Expect(err).Should(BeNil())
		})
		It("list workflow should be succeed", func() {
			wfList, err := mgr.ListWorkflows(context.TODO())
			Expect(err).Should(BeNil())
			Expect(len(wfList) > 0).Should(BeTrue())
		})
	})

	Context("delete workflow", func() {
		It("delete workflow should be succeed", func() {
			Expect(mgr.DeleteWorkflow(context.TODO(), wf.Id)).Should(BeNil())
		})
		It("query deleted workflow should be error", func() {
			_, err := mgr.GetWorkflow(context.TODO(), wf.Id)
			Expect(err).Should(Equal(types.ErrNotFound))
		})
	})
})

var _ = Describe("TestWorkflowJobManage", func() {
	ps := types.PlugScope{
		PluginName: "test-sleep-plugin-succeed",
		Version:    "1.0",
		PluginType: types.TypeProcess,
		Parameters: map[string]string{},
	}

	wf := &types.WorkflowSpec{
		Name: "test-trigger-workflow-1",
		Rule: types.Rule{},
		Steps: []types.WorkflowStepSpec{
			{Name: "step-1", Plugin: ps},
			{Name: "step-2", Plugin: ps},
		},
	}
	caller.mockResponse(ps, func() (*common.Response, error) {
		time.Sleep(time.Second)
		return &common.Response{IsSucceed: true}, nil
	})
	Context("trigger a workflow", func() {
		It("create workflow should be succeed", func() {
			var err error
			wf, err = mgr.SaveWorkflow(context.TODO(), wf)
			Expect(err).Should(BeNil())
			Expect(wf.Id).ShouldNot(BeEmpty())
		})
	})

	Context("trigger a workflow", func() {
		var job types.WorkflowJob
		It("trigger workflow should be succeed", func() {
			var err error
			job, err = mgr.TriggerWorkflow(context.TODO(), wf.Id)
			Expect(err).Should(BeNil())
			Expect(job.Id).ShouldNot(BeEmpty())
		})

		It("job should be succeed", func() {
			Eventually(func() string {
				jobList, err := mgr.ListJobs(context.TODO(), wf.Id)
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
		var job types.WorkflowJob
		It("trigger workflow should be succeed", func() {
			var err error
			job, err = mgr.TriggerWorkflow(context.TODO(), wf.Id)
			Expect(err).Should(BeNil())
			Expect(job.Id).ShouldNot(BeEmpty())

			Eventually(func() string {
				jobList, err := mgr.ListJobs(context.TODO(), wf.Id)
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
			Expect(mgr.PauseWorkflowJob(context.TODO(), job.Id)).Should(BeNil())
			Eventually(func() string {
				jobList, err := mgr.ListJobs(context.TODO(), wf.Id)
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
			Expect(mgr.ResumeWorkflowJob(context.TODO(), job.Id)).Should(BeNil())
			Eventually(func() string {
				jobList, err := mgr.ListJobs(context.TODO(), wf.Id)
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
				jobList, err := mgr.ListJobs(context.TODO(), wf.Id)
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
		var job types.WorkflowJob
		It("trigger workflow should be succeed", func() {
			var err error
			job, err = mgr.TriggerWorkflow(context.TODO(), wf.Id)
			Expect(err).Should(BeNil())
			Expect(job.Id).ShouldNot(BeEmpty())

			Eventually(func() string {
				jobList, err := mgr.ListJobs(context.TODO(), wf.Id)
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
			Expect(mgr.PauseWorkflowJob(context.TODO(), job.Id)).Should(BeNil())
			Eventually(func() string {
				jobList, err := mgr.ListJobs(context.TODO(), wf.Id)
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
			Expect(mgr.CancelWorkflowJob(context.TODO(), job.Id)).Should(BeNil())
			Eventually(func() string {
				jobList, err := mgr.ListJobs(context.TODO(), wf.Id)
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
