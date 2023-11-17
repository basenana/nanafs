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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("TestJobPauseAndResume", func() {
	var (
		ctx = context.TODO()
		job = &types.WorkflowJob{
			Id:       "TestJobControl-test-runner-1",
			Workflow: "fake-workflow-1",
			Target:   types.WorkflowTarget{},
			Steps: []types.WorkflowJobStep{
				{StepName: "mock-step-1"},
				{StepName: "mock-step-2"},
				{StepName: "mock-step-3"},
				{StepName: "mock-step-4"},
			},
		}
		ctrl *Controller
	)

	Context("runner start", func() {
		It("init runner should be succeed", func() {
			ctrl = NewJobController(recorder, notifyImpl)
			err := recorder.SaveWorkflowJob(ctx, job)
			Expect(err).Should(BeNil())
		})
		It("start should be succeed", func() {
			Expect(ctrl.TriggerJob(job.Id, time.Hour)).Should(BeNil())
		})
		It("wait first step status should be succeed", func() {
			Eventually(func() string {
				jobs, err := recorder.ListWorkflowJob(ctx, types.JobFilter{JobID: "TestJobControl-test-runner-1"})
				Expect(err).Should(BeNil())

				if len(jobs) == 1 {
					return jobs[0].Steps[0].Status

				}
				return ""
			}, time.Second*30, time.Second).Should(Equal(SucceedStatus))
		})
		It("pause should be succeed", func() {
			Expect(ctrl.PauseJob(job.Id)).Should(BeNil())
			Eventually(func() string {
				jobs, err := recorder.ListWorkflowJob(ctx, types.JobFilter{JobID: "TestJobControl-test-runner-1"})
				Expect(err).Should(BeNil())

				if len(jobs) == 1 {
					return jobs[0].Status

				}
				return ""
			}, time.Second*30, time.Second).Should(Equal(PausedStatus))
		})
		It("resume should be succeed", func() {
			Expect(ctrl.ResumeJob(job.Id)).Should(BeNil())
			Eventually(func() string {
				jobs, err := recorder.ListWorkflowJob(ctx, types.JobFilter{JobID: "TestJobControl-test-runner-1"})
				Expect(err).Should(BeNil())

				if len(jobs) == 1 {
					return jobs[0].Status

				}
				return ""
			}, time.Second*30, time.Second).Should(Equal(RunningStatus))
		})
		It("job status should be succeed", func() {
			Eventually(func() string {
				jobs, err := recorder.ListWorkflowJob(ctx, types.JobFilter{JobID: "TestJobControl-test-runner-1"})
				Expect(err).Should(BeNil())

				if len(jobs) == 1 {
					return jobs[0].Status

				}
				return ""
			}, time.Second*30, time.Second).Should(Equal(SucceedStatus))
		})
	})
})

var _ = Describe("TestJobCancel", func() {
	var (
		ctx = context.TODO()
		job = &types.WorkflowJob{
			Id:       "TestJobControl-test-runner-2",
			Workflow: "fake-workflow-1",
			Target:   types.WorkflowTarget{},
			Steps: []types.WorkflowJobStep{
				{StepName: "mock-step-1"},
				{StepName: "mock-step-2"},
				{StepName: "mock-step-3"},
				{StepName: "mock-step-4"},
			},
		}
		ctrl *Controller
	)

	Context("runner start", func() {
		It("init runner should be succeed", func() {
			ctrl = NewJobController(recorder, notifyImpl)
			err := recorder.SaveWorkflowJob(ctx, job)
			Expect(err).Should(BeNil())
		})
		It("start should be succeed", func() {
			Expect(ctrl.TriggerJob(job.Id, time.Hour)).Should(BeNil())
		})
		It("wait first step status should be succeed", func() {
			Eventually(func() string {
				jobs, err := recorder.ListWorkflowJob(ctx, types.JobFilter{JobID: "TestJobControl-test-runner-2"})
				Expect(err).Should(BeNil())

				if len(jobs) == 1 {
					return jobs[0].Steps[0].Status

				}
				return ""
			}, time.Second*30, time.Second).Should(Equal(SucceedStatus))
		})
		It("pause should be succeed", func() {
			Expect(ctrl.PauseJob(job.Id)).Should(BeNil())
			Eventually(func() string {
				jobs, err := recorder.ListWorkflowJob(ctx, types.JobFilter{JobID: "TestJobControl-test-runner-2"})
				Expect(err).Should(BeNil())

				if len(jobs) == 1 {
					return jobs[0].Status

				}
				return ""
			}, time.Second*30, time.Second).Should(Equal(PausedStatus))
		})
		It("cancel should be succeed", func() {
			Expect(ctrl.CancelJob(job.Id)).Should(BeNil())
			Eventually(func() string {
				jobs, err := recorder.ListWorkflowJob(ctx, types.JobFilter{JobID: "TestJobControl-test-runner-2"})
				Expect(err).Should(BeNil())

				if len(jobs) == 1 {
					return jobs[0].Status

				}
				return ""
			}, time.Second*30, time.Second).Should(Equal(CanceledStatus))
		})
	})
})
