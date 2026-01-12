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

package dispatch

import (
	"context"
	"time"

	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/workflow/jobrun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
)

var _ = Describe("workflowExecutor.cleanUpFinishJobs", func() {
	It("should delete expired succeed jobs", func() {
		w := &workflowExecutor{
			recorder: testMeta,
			logger:   zap.NewNop().Sugar(),
		}

		// Create workflow first
		wf := &types.Workflow{
			Id:        "test-workflow-1",
			Namespace: namespace,
		}
		_ = testMeta.SaveWorkflow(context.TODO(), namespace, wf)

		// Create expired succeed job
		expiredSucceedJob := &types.WorkflowJob{
			Id:        "expired-succeed-1",
			Workflow:  "test-workflow-1",
			Namespace: namespace,
			Status:    jobrun.SucceedStatus,
			FinishAt:  time.Now().Add(-48 * time.Hour), // 2 days ago
		}
		_ = testMeta.SaveWorkflowJob(context.TODO(), namespace, expiredSucceedJob)

		err := w.cleanUpFinishJobs(context.TODO())
		Expect(err).Should(BeNil())

		// Verify job was deleted
		_, err = testMeta.GetWorkflowJob(context.TODO(), namespace, expiredSucceedJob.Id)
		Expect(err).ShouldNot(BeNil())
	})

	It("should not delete non-expired succeed jobs", func() {
		w := &workflowExecutor{
			recorder: testMeta,
			logger:   zap.NewNop().Sugar(),
		}

		// Create workflow first (required by SaveWorkflowJob)
		wf := &types.Workflow{
			Id:        "test-workflow",
			Namespace: namespace,
		}
		_ = testMeta.SaveWorkflow(context.TODO(), namespace, wf)

		// Create fresh succeed job (finished 1 hour ago)
		freshSucceedJob := &types.WorkflowJob{
			Id:        "fresh-succeed-1",
			Workflow:  "test-workflow",
			Namespace: namespace,
			Status:    jobrun.SucceedStatus,
			FinishAt:  time.Now().Add(-time.Hour),
		}
		_ = testMeta.SaveWorkflowJob(context.TODO(), namespace, freshSucceedJob)

		// Verify job exists before cleanup
		jobBefore, err := testMeta.GetWorkflowJob(context.TODO(), namespace, freshSucceedJob.Id)
		Expect(err).Should(BeNil(), "job should exist before cleanup")
		Expect(jobBefore).ShouldNot(BeNil())

		err = w.cleanUpFinishJobs(context.TODO())
		Expect(err).Should(BeNil())

		// Verify job was NOT deleted
		job, err := testMeta.GetWorkflowJob(context.TODO(), namespace, freshSucceedJob.Id)
		Expect(err).Should(BeNil())
		Expect(job).ShouldNot(BeNil())
	})

	It("should delete expired failed jobs", func() {
		w := &workflowExecutor{
			recorder: testMeta,
			logger:   zap.NewNop().Sugar(),
		}

		// Create workflow first
		wf := &types.Workflow{
			Id:        "test-workflow-2",
			Namespace: namespace,
		}
		_ = testMeta.SaveWorkflow(context.TODO(), namespace, wf)

		// Create expired failed job (8 days ago)
		expiredFailedJob := &types.WorkflowJob{
			Id:        "expired-failed-1",
			Workflow:  "test-workflow-2",
			Namespace: namespace,
			Status:    jobrun.FailedStatus,
			FinishAt:  time.Now().Add(-8 * 24 * time.Hour),
		}
		_ = testMeta.SaveWorkflowJob(context.TODO(), namespace, expiredFailedJob)

		err := w.cleanUpFinishJobs(context.TODO())
		Expect(err).Should(BeNil())

		// Verify job was deleted
		_, err = testMeta.GetWorkflowJob(context.TODO(), namespace, expiredFailedJob.Id)
		Expect(err).ShouldNot(BeNil())
	})

	It("should handle mixed expired and non-expired jobs", func() {
		w := &workflowExecutor{
			recorder: testMeta,
			logger:   zap.NewNop().Sugar(),
		}

		// Create workflow first
		wf := &types.Workflow{
			Id:        "test-workflow-3",
			Namespace: namespace,
		}
		_ = testMeta.SaveWorkflow(context.TODO(), namespace, wf)

		// Create expired succeed job
		expiredJob := &types.WorkflowJob{
			Id:        "mixed-expired",
			Workflow:  "test-workflow-3",
			Namespace: namespace,
			Status:    jobrun.SucceedStatus,
			FinishAt:  time.Now().Add(-48 * time.Hour),
		}
		_ = testMeta.SaveWorkflowJob(context.TODO(), namespace, expiredJob)

		// Create fresh succeed job
		freshJob := &types.WorkflowJob{
			Id:        "mixed-fresh",
			Workflow:  "test-workflow-3",
			Namespace: namespace,
			Status:    jobrun.SucceedStatus,
			FinishAt:  time.Now().Add(-time.Hour),
		}
		_ = testMeta.SaveWorkflowJob(context.TODO(), namespace, freshJob)

		err := w.cleanUpFinishJobs(context.TODO())
		Expect(err).Should(BeNil())

		// Expired job should be deleted
		_, err = testMeta.GetWorkflowJob(context.TODO(), namespace, expiredJob.Id)
		Expect(err).ShouldNot(BeNil())

		// Fresh job should remain
		job, err := testMeta.GetWorkflowJob(context.TODO(), namespace, freshJob.Id)
		Expect(err).Should(BeNil())
		Expect(job).ShouldNot(BeNil())
	})
})
