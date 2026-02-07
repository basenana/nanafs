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

package jobrun

import (
	"context"

	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Controller Start", func() {
	var (
		ctx  context.Context
		ctrl *Controller
	)

	BeforeEach(func() {
		ctx = context.Background()
		ctrl = &Controller{
			store:  memMeta,
			logger: logger.NewLogger("flow"),
		}
	})

	Context("with scheduler not started", func() {
		It("should create and start scheduler", func() {
			Expect(ctrl.scheduler).To(BeNil())

			ctrl.Start(ctx)

			Expect(ctrl.scheduler).ToNot(BeNil())
			Expect(ctrl.scheduler.workers).To(Equal(10))
		})
	})

	Context("with scheduler already started", func() {
		It("should not create another scheduler", func() {
			ctrl.Start(ctx)
			firstScheduler := ctrl.scheduler

			ctrl.Start(ctx)

			Expect(ctrl.scheduler).To(Equal(firstScheduler))
		})
	})
})

var _ = Describe("Controller recoverRunningJobs", func() {
	var (
		ctx  context.Context
		ctrl *Controller
	)

	BeforeEach(func() {
		ctx = context.Background()
		ctrl = &Controller{
			store:  memMeta,
			logger: logger.NewLogger("flow"),
		}
	})

	AfterEach(func() {
		// Cleanup jobs created during test
		_ = memMeta.DeleteWorkflowJobs(ctx, "test-wf-recover-1", "test-wf-recover-2")
	})

	Context("with running jobs exist", func() {
		It("should reset running jobs to initializing status", func() {
			// Create workflow first (required by SaveWorkflowJob)
			wf := &types.Workflow{
				Id:        "test-wf-recover",
				Namespace: namespace,
				Name:      "Test Workflow",
				Nodes: []types.WorkflowNode{
					{Name: "step1", Type: "test"},
				},
				QueueName: types.WorkflowQueueFile,
			}
			err := memMeta.SaveWorkflow(ctx, namespace, wf)
			Expect(err).To(BeNil())

			// Create job with running status
			job := &types.WorkflowJob{
				Id:        "test-wf-recover-1",
				Namespace: namespace,
				Workflow:  "test-wf-recover",
				Status:    RunningStatus,
				QueueName: types.WorkflowQueueFile,
			}
			err = memMeta.SaveWorkflowJob(ctx, namespace, job)
			Expect(err).To(BeNil())

			// Verify job is saved with running status
			saved, err := memMeta.GetWorkflowJob(ctx, namespace, job.Id)
			Expect(err).To(BeNil())
			Expect(saved.Status).To(Equal(RunningStatus))

			// Call recoverRunningJobs
			err = ctrl.recoverRunningJobs(ctx)
			Expect(err).To(BeNil())

			// Verify job status is reset to initializing
			recovered, err := memMeta.GetWorkflowJob(ctx, namespace, job.Id)
			Expect(err).To(BeNil())
			Expect(recovered.Status).To(Equal(InitializingStatus))
		})

		It("should handle multiple running jobs", func() {
			// Create workflow
			wf := &types.Workflow{
				Id:        "test-wf-recover",
				Namespace: namespace,
				Name:      "Test Workflow",
				Nodes: []types.WorkflowNode{
					{Name: "step1", Type: "test"},
				},
				QueueName: types.WorkflowQueueFile,
			}
			err := memMeta.SaveWorkflow(ctx, namespace, wf)
			Expect(err).To(BeNil())

			// Create multiple jobs with running status
			jobs := []*types.WorkflowJob{
				{Id: "test-wf-recover-1", Namespace: namespace, Workflow: "test-wf-recover", Status: RunningStatus, QueueName: types.WorkflowQueueFile},
				{Id: "test-wf-recover-2", Namespace: namespace, Workflow: "test-wf-recover", Status: RunningStatus, QueueName: types.WorkflowQueueFile},
			}
			for _, j := range jobs {
				err = memMeta.SaveWorkflowJob(ctx, namespace, j)
				Expect(err).To(BeNil())
			}

			// Verify all jobs have running status
			for _, j := range jobs {
				saved, err := memMeta.GetWorkflowJob(ctx, namespace, j.Id)
				Expect(err).To(BeNil())
				Expect(saved.Status).To(Equal(RunningStatus))
			}

			// Recover running jobs
			err = ctrl.recoverRunningJobs(ctx)
			Expect(err).To(BeNil())

			// Verify all jobs are reset to initializing
			for _, j := range jobs {
				recovered, err := memMeta.GetWorkflowJob(ctx, namespace, j.Id)
				Expect(err).To(BeNil())
				Expect(recovered.Status).To(Equal(InitializingStatus))
			}
		})
	})
})

var _ = Describe("Controller getRunner and addRunner", func() {
	var (
		ctrl *Controller
	)

	BeforeEach(func() {
		ctrl = &Controller{
			runners: make(map[JobID]*runner),
		}
	})

	Context("addRunner and getRunner", func() {
		It("should store and retrieve runner", func() {
			job := newWorkflowJob("runner-test")

			ctrl.addRunner(job, &runner{
				namespace: job.Namespace,
				workflow:  job.Workflow,
				job:       job.Id,
				runner:    nil,
			})

			retrieved := ctrl.getRunner(job.Namespace, job.Id)
			Expect(retrieved).ToNot(BeNil())
		})
	})

	Context("getRunner for non-existent job", func() {
		It("should return nil", func() {
			retrieved := ctrl.getRunner("non-existent", "job")
			Expect(retrieved).To(BeNil())
		})
	})
})

var _ = Describe("Controller Shutdown", func() {
	var (
		ctrl *Controller
	)

	BeforeEach(func() {
		ctrl = &Controller{
			runners: make(map[JobID]*runner),
		}
	})

	Context("with no scheduler", func() {
		It("should return nil without error", func() {
			err := ctrl.Shutdown()
			Expect(err).To(BeNil())
		})
	})
})
