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
	"github.com/basenana/go-flow"
	"github.com/basenana/nanafs/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Task", func() {
	var (
		job  *types.WorkflowJob
		step *types.WorkflowJobNode
		task *Task
	)

	BeforeEach(func() {
		job = &types.WorkflowJob{
			Id:        "test-job-1",
			Namespace: "test-ns",
		}
		step = &types.WorkflowJobNode{
			WorkflowNode: types.WorkflowNode{
				Name: "test-step",
				Type: "test",
				Next: "next-step",
			},
			BranchNext: "",
			Status:     "",
			Message:    "",
		}
		task = &Task{job: job, step: step}
	})

	It("should implement flow.Task interface", func() {
		Expect(task).ToNot(BeNil())
	})

	It("should return step name", func() {
		Expect(task.GetName()).To(Equal("test-step"))
	})

	It("should return step status", func() {
		step.Status = "running"
		Expect(task.GetStatus()).To(Equal("running"))
	})

	It("should set step status", func() {
		task.SetStatus("completed")
		Expect(step.Status).To(Equal("completed"))
	})

	It("should return step message", func() {
		step.Message = "test message"
		Expect(task.GetMessage()).To(Equal("test message"))
	})

	It("should set step message", func() {
		task.SetMessage("updated message")
		Expect(step.Message).To(Equal("updated message"))
	})

	It("should return branch next", func() {
		step.BranchNext = "conditional-branch"
		Expect(task.GetBranchNext()).To(Equal("conditional-branch"))
	})

	It("should set branch next", func() {
		task.SetBranchNext("new-branch")
		Expect(step.BranchNext).To(Equal("new-branch"))
	})

	It("should work as flow.Task", func() {
		var _ flow.Task = task
	})
})

var _ = Describe("JobID", func() {
	Context("with valid flow ID", func() {
		It("should parse namespace and id from flow ID", func() {
			jid := NewJobID("namespace.job123")
			Expect(jid.namespace).To(Equal("namespace"))
			Expect(jid.id).To(Equal("job123"))
		})

		It("should handle flow ID without namespace prefix", func() {
			jid := NewJobID("job456")
			Expect(jid.namespace).To(Equal("job456"))
			Expect(jid.id).To(Equal(""))
		})
	})

	Context("with FlowID method", func() {
		It("should return combined flow ID", func() {
			jid := JobID{namespace: "ns", id: "job789"}
			Expect(jid.FlowID()).To(Equal("ns.job789"))
		})
	})
})

var _ = Describe("Coordinator", func() {
	var (
		coord *coordinator
	)

	BeforeEach(func() {
		coord = &coordinator{
			next:       make(map[string]string),
			crt:        "",
			isFinished: false,
		}
	})

	It("should implement flow.Coordinator interface", func() {
		Expect(coord).ToNot(BeNil())
	})

	Context("NewTask", func() {
		It("should set initial current task", func() {
			job := &types.WorkflowJob{
				Id:        "test-job",
				Namespace: "test-ns",
			}
			step := &types.WorkflowJobNode{
				WorkflowNode: types.WorkflowNode{
					Name: "step1",
					Next: "step2",
				},
			}
			task := &Task{job: job, step: step}

			coord.NewTask(task)
			Expect(coord.crt).To(Equal("step1"))
			Expect(coord.next["step1"]).To(Equal("step2"))
		})
	})

	Context("UpdateTask", func() {
		It("should advance to next task on success", func() {
			job := &types.WorkflowJob{
				Id:        "test-job",
				Namespace: "test-ns",
			}
			step1 := &types.WorkflowJobNode{
				WorkflowNode: types.WorkflowNode{
					Name: "step1",
					Next: "step2",
				},
				Status: SucceedStatus,
			}
			step2 := &types.WorkflowJobNode{
				WorkflowNode: types.WorkflowNode{
					Name: "step2",
					Next: "",
				},
			}

			task1 := &Task{job: job, step: step1}
			task2 := &Task{job: job, step: step2}

			coord.NewTask(task1)
			coord.NewTask(task2)

			coord.crt = "step1"
			coord.UpdateTask(task1)

			Expect(coord.crt).To(Equal("step2"))
		})

		It("should use branch next when set", func() {
			job := &types.WorkflowJob{
				Id:        "test-job",
				Namespace: "test-ns",
			}
			step := &types.WorkflowJobNode{
				WorkflowNode: types.WorkflowNode{
					Name:     "condition-step",
					Next:     "default-next",
					Branches: map[string]string{"true": "branch-true"},
				},
				BranchNext: "branch-true",
				Status:     SucceedStatus,
			}
			task := &Task{job: job, step: step}

			coord.NewTask(task)
			coord.crt = "condition-step"
			task.SetBranchNext("branch-true")
			coord.UpdateTask(task)

			Expect(coord.crt).To(Equal("branch-true"))
		})

		It("should not advance for failed task", func() {
			job := &types.WorkflowJob{
				Id:        "test-job",
				Namespace: "test-ns",
			}
			step := &types.WorkflowJobNode{
				WorkflowNode: types.WorkflowNode{
					Name: "step1",
					Next: "step2",
				},
				Status: FailedStatus,
			}
			task := &Task{job: job, step: step}

			coord.NewTask(task)
			coord.crt = "step1"
			coord.UpdateTask(task)

			Expect(coord.crt).To(Equal("step1"))
		})

		It("should not advance for running task", func() {
			job := &types.WorkflowJob{
				Id:        "test-job",
				Namespace: "test-ns",
			}
			step := &types.WorkflowJobNode{
				WorkflowNode: types.WorkflowNode{
					Name: "step1",
					Next: "step2",
				},
				Status: RunningStatus,
			}
			task := &Task{job: job, step: step}

			coord.NewTask(task)
			coord.crt = "step1"
			coord.UpdateTask(task)

			Expect(coord.crt).To(Equal("step1"))
		})
	})

	Context("NextBatch", func() {
		It("should return next task when not finished", func() {
			coord.crt = "step1"
			nextTasks, err := coord.NextBatch(nil)
			Expect(err).To(BeNil())
			Expect(nextTasks).To(Equal([]string{"step1"}))
			Expect(coord.isFinished).To(BeFalse())
		})

		It("should return nil and mark finished when no current task", func() {
			coord.crt = ""
			nextTasks, err := coord.NextBatch(nil)
			Expect(err).To(BeNil())
			Expect(nextTasks).To(BeNil())
			Expect(coord.isFinished).To(BeTrue())
		})
	})

	Context("Finished", func() {
		It("should return false initially", func() {
			coord.isFinished = false
			Expect(coord.Finished()).To(BeFalse())
		})

		It("should return true when marked finished", func() {
			coord.isFinished = true
			Expect(coord.Finished()).To(BeTrue())
		})
	})

	Context("HandleFail", func() {
		It("should mark as finished and return FailAndInterrupt", func() {
			result := coord.HandleFail(nil, nil)
			Expect(coord.isFinished).To(BeTrue())
			Expect(result).To(Equal(flow.FailAndInterrupt))
		})
	})
})
