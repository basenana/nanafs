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
	"os"
	"time"

	"github.com/basenana/nanafs/utils/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("NewScheduler", func() {
	Context("with default values", func() {
		It("should use 10 workers by default", func() {
			scheduler := NewScheduler(nil, 0, 0, "")
			Expect(scheduler.workers).To(Equal(10))
		})

		It("should use 5 second interval by default", func() {
			scheduler := NewScheduler(nil, 0, 0, "")
			Expect(scheduler.interval).To(Equal(5 * time.Second))
		})

		It("should use file queue name by default", func() {
			scheduler := NewScheduler(nil, 0, 0, "")
			Expect(scheduler.queueName).To(Equal("file"))
		})
	})

	Context("with custom values", func() {
		It("should use custom workers count", func() {
			scheduler := NewScheduler(nil, 5, 0, "")
			Expect(scheduler.workers).To(Equal(5))
		})

		It("should use custom interval", func() {
			scheduler := NewScheduler(nil, 0, 10*time.Second, "")
			Expect(scheduler.interval).To(Equal(10 * time.Second))
		})

		It("should use custom queue name", func() {
			scheduler := NewScheduler(nil, 0, 0, "custom-queue")
			Expect(scheduler.queueName).To(Equal("custom-queue"))
		})
	})

	Context("with invalid values", func() {
		It("should use default workers for negative values", func() {
			scheduler := NewScheduler(nil, -1, 0, "")
			Expect(scheduler.workers).To(Equal(10))
		})

		It("should use default workers for zero", func() {
			scheduler := NewScheduler(nil, 0, 0, "")
			Expect(scheduler.workers).To(Equal(10))
		})

		It("should use default interval for zero", func() {
			scheduler := NewScheduler(nil, 0, 0, "")
			Expect(scheduler.interval).To(Equal(5 * time.Second))
		})
	})
})

var _ = Describe("Scheduler Stop", func() {
	It("should close stop channel when stopped", func() {
		scheduler := NewScheduler(nil, 1, time.Second, "")
		done := make(chan bool, 1)

		go func() {
			scheduler.Stop()
			done <- true
		}()

		Eventually(done).WithTimeout(time.Second).Should(Receive())
	})
})

var _ = Describe("Scheduler Run", func() {
	var ctrl *Controller

	BeforeEach(func() {
		ctrl = &Controller{
			runners: make(map[JobID]*runner),
			logger:  logger.NewLogger("test"),
			store:   memMeta,
			core:    testCore,
		}
	})

	It("should start specified number of workers", func() {
		scheduler := NewScheduler(ctrl, 3, time.Second, "")

		ctx, cancel := context.WithCancel(context.Background())
		go scheduler.Run(ctx)

		time.Sleep(100 * time.Millisecond)
		cancel()

		scheduler.Stop()
	})

	It("should not start workers when context already cancelled", func() {
		scheduler := NewScheduler(ctrl, 2, time.Second, "")

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		scheduler.Run(ctx)

		time.Sleep(100 * time.Millisecond)
		scheduler.Stop()
	})
})

var _ = Describe("Scheduler worker", func() {
	var ctrl *Controller

	BeforeEach(func() {
		ctrl = &Controller{
			runners: make(map[JobID]*runner),
			logger:  logger.NewLogger("test"),
			store:   memMeta,
			core:    testCore,
		}
	})

	It("should exit when context is cancelled", func() {
		scheduler := NewScheduler(ctrl, 1, time.Second, "")

		ctx, cancel := context.WithCancel(context.Background())

		go scheduler.Run(ctx)
		time.Sleep(50 * time.Millisecond)

		cancel()
		scheduler.Stop()
	})

	It("should exit when stop is called", func() {
		scheduler := NewScheduler(ctrl, 1, time.Second, "")

		done := make(chan bool, 1)

		go func() {
			scheduler.Run(context.Background())
			done <- true
		}()

		time.Sleep(50 * time.Millisecond)
		scheduler.Stop()

		Eventually(done).WithTimeout(time.Second).Should(Receive())
	})
})

var _ = Describe("Scheduler executeJob", func() {
	var (
		workdir string
		ctrl    *Controller
	)

	BeforeEach(func() {
		var err error
		workdir, err = os.MkdirTemp(tempDir, "exec-job-")
		Expect(err).To(BeNil())

		ctrl = &Controller{
			runners: make(map[JobID]*runner),
			logger:  logger.NewLogger("test"),
			store:   memMeta,
			core:    testCore,
		}
	})

	AfterEach(func() {
		if workdir != "" {
			os.RemoveAll(workdir)
		}
	})

	Context("with runner added after execute", func() {
		It("should add runner to controller", func() {
			job := newWorkflowJob("execute-test")

			scheduler := NewScheduler(ctrl, 1, time.Second, "")

			scheduler.executeJob(context.Background(), job)

			runner := ctrl.getRunner(job.Namespace, job.Id)
			Expect(runner).ToNot(BeNil())
		})
	})
})

var _ = Describe("Scheduler worker with short interval", func() {
	It("should poll frequently with short interval", func() {
		ctrl := &Controller{
			runners: make(map[JobID]*runner),
			logger:  logger.NewLogger("test"),
			store:   memMeta,
			core:    testCore,
		}
		scheduler := NewScheduler(ctrl, 1, 50*time.Millisecond, "")

		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan bool, 1)

		go func() {
			scheduler.Run(ctx)
			done <- true
		}()

		time.Sleep(200 * time.Millisecond)
		cancel()

		Eventually(done).WithTimeout(time.Second).Should(Receive())
	})
})
