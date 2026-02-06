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
		ctrl = &Controller{}
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
