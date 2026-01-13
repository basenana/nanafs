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
	"fmt"
	"time"

	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
)

var _ = Describe("Dispatcher.dispatch", func() {
	BeforeEach(func() {
		// Each test uses fresh testMeta from suite
	})

	It("should transition from Wait to Succeed", func() {
		exec := &mockExecutor{shouldFail: false}
		d := &Dispatcher{
			recorder:  testMeta,
			executors: map[string]executor{"test.task": exec},
			logger:    zap.NewNop().Sugar(),
			notify:    testNotify,
		}

		task := newTestTask("test.task", types.ScheduledTaskWait)
		err := testMeta.SaveTask(context.TODO(), task)
		Expect(err).Should(BeNil())

		err = d.dispatch(context.TODO(), "test.task", exec, task)
		Expect(err).Should(BeNil())
		Expect(task.Status).Should(Equal(types.ScheduledTaskSucceed))
		Expect(task.Result).Should(Equal("succeed"))
	})

	It("should transition from Wait to Failed", func() {
		exec := &mockExecutor{shouldFail: true}
		d := &Dispatcher{
			recorder:  testMeta,
			executors: map[string]executor{"test.task": exec},
			logger:    zap.NewNop().Sugar(),
			notify:    testNotify,
		}

		task := newTestTask("test.task", types.ScheduledTaskWait)
		err := testMeta.SaveTask(context.TODO(), task)
		Expect(err).Should(BeNil())

		err = d.dispatch(context.TODO(), "test.task", exec, task)
		Expect(err).Should(BeNil())
		Expect(task.Status).Should(Equal(types.ScheduledTaskFailed))
		Expect(task.Result).ShouldNot(BeEmpty())
	})

	It("should transition back to Wait on retry", func() {
		exec := &mockExecutor{shouldRetry: true}
		d := &Dispatcher{
			recorder:  testMeta,
			executors: map[string]executor{"test.task": exec},
			logger:    zap.NewNop().Sugar(),
			notify:    testNotify,
		}

		task := newTestTask("test.task", types.ScheduledTaskWait)
		err := testMeta.SaveTask(context.TODO(), task)
		Expect(err).Should(BeNil())

		err = d.dispatch(context.TODO(), "test.task", exec, task)
		Expect(err).Should(BeNil())
		Expect(task.Status).Should(Equal(types.ScheduledTaskWait))
	})
})

var _ = Describe("Dispatcher.findRunnableTasks", func() {
	It("should include tasks with past ExecutionTime", func() {
		d := &Dispatcher{
			recorder:  testMeta,
			executors: map[string]executor{},
			logger:    zap.NewNop().Sugar(),
		}

		task := newTestTask("test.task", types.ScheduledTaskWait)
		task.ExecutionTime = time.Now().Add(-time.Minute)
		_ = testMeta.SaveTask(context.TODO(), task)

		runnable, err := d.findRunnableTasks(context.TODO(), "test.task")
		Expect(err).Should(BeNil())
		Expect(len(runnable)).Should(Equal(1))
		Expect(runnable[0].ID).Should(Equal(task.ID))
	})

	It("should exclude tasks with future ExecutionTime", func() {
		d := &Dispatcher{
			recorder:  testMeta,
			executors: map[string]executor{},
			logger:    zap.NewNop().Sugar(),
		}

		task := newTestTask("test.task", types.ScheduledTaskWait)
		task.ExecutionTime = time.Now().Add(time.Hour)
		_ = testMeta.SaveTask(context.TODO(), task)

		runnable, err := d.findRunnableTasks(context.TODO(), "test.task")
		Expect(err).Should(BeNil())
		Expect(len(runnable)).Should(Equal(0))
	})
})

var _ = Describe("Dispatcher.registerRoutineTask", func() {
	It("should register task to correct hour slots", func() {
		d := &Dispatcher{
			routines: [24][]routineTask{},
			logger:   zap.NewNop().Sugar(),
		}

		d.registerRoutineTask(6, func(ctx context.Context) error { return nil })

		// periodH=6 should register to hours: 0, 6, 12, 18
		for i := 0; i < 24; i++ {
			if i%6 == 0 {
				Expect(len(d.routines[i])).Should(Equal(1), "hour %d should have 1 task", i)
			} else {
				Expect(len(d.routines[i])).Should(Equal(0), "hour %d should have 0 tasks", i)
			}
		}
	})
})

// mockExecutor for testing
type mockExecutor struct {
	shouldFail  bool
	shouldRetry bool
}

func (m *mockExecutor) execute(ctx context.Context, task *types.ScheduledTask) error {
	if m.shouldFail {
		return fmt.Errorf("test error")
	}
	if m.shouldRetry {
		return ErrNeedRetry
	}
	return nil
}

// newTestTask creates a test task with given taskID and status
func newTestTask(taskID string, status string) *types.ScheduledTask {
	return &types.ScheduledTask{
		Namespace:      namespace,
		TaskID:         taskID,
		Status:         status,
		RefType:        "entry",
		RefID:          100,
		CreatedTime:    time.Now(),
		ExecutionTime:  time.Now().Add(-time.Minute),
		ExpirationTime: time.Now().Add(time.Hour),
		Event: types.Event{
			Type:      events.ActionTypeRemove,
			Namespace: namespace,
			RefType:   "entry",
			RefID:     100,
		},
	}
}
