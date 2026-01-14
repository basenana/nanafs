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

	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
)

func createTestEntry(ctx context.Context, name string) *types.Entry {
	entry, _ := fsCore.CreateEntry(ctx, namespace, "/", types.EntryAttr{
		Name: name,
		Kind: types.RawKind,
	})
	return entry
}

func makeOrphan(ctx context.Context, name string) *types.Entry {
	entry, _ := fsCore.CreateEntry(ctx, namespace, "/", types.EntryAttr{
		Name: name,
		Kind: types.RawKind,
	})
	_ = fsCore.RemoveEntry(ctx, namespace, "/"+name, types.DeleteEntry{})
	time.Sleep(time.Second)
	GinkgoT().Logf("Created orphan: ID=%d, RefCount=0", entry.ID)
	return entry
}

var _ = Describe("entryCleanExecutor.scanOrphanEntriesTask", func() {
	It("should create cleanup task for orphan entries", func() {
		ctx := context.TODO()
		orphanEntry := makeOrphan(ctx, "orphan")
		Expect(orphanEntry).ShouldNot(BeNil())

		e := &entryCleanExecutor{
			maintainExecutor: &maintainExecutor{
				recorder:  testMeta,
				metastore: testMeta,
				logger:    zap.NewNop().Sugar(),
			},
			orphanThreshold: time.Second,
		}

		err := e.scanOrphanEntriesTask(ctx)
		Expect(err).Should(BeNil())

		tasks, err := testMeta.ListTask(ctx, maintainTaskIDEntryCleanup, types.ScheduledTaskFilter{RefID: orphanEntry.ID})
		Expect(err).Should(BeNil())
		Expect(len(tasks)).Should(Equal(1), "should create task for our orphan entry")
		Expect(tasks[0].RefID).Should(Equal(orphanEntry.ID))
	})

	It("should not create duplicate task", func() {
		ctx := context.TODO()
		orphanEntry := makeOrphan(ctx, "orphan2")
		Expect(orphanEntry).ShouldNot(BeNil())

		e := &entryCleanExecutor{
			maintainExecutor: &maintainExecutor{
				recorder:  testMeta,
				metastore: testMeta,
				logger:    zap.NewNop().Sugar(),
			},
			orphanThreshold: time.Second,
		}

		existingTask := &types.ScheduledTask{
			Namespace:      namespace,
			TaskID:         maintainTaskIDEntryCleanup,
			Status:         types.ScheduledTaskWait,
			RefType:        "entry",
			RefID:          orphanEntry.ID,
			CreatedTime:    time.Now(),
			ExpirationTime: time.Now().Add(time.Hour),
		}
		_ = testMeta.SaveTask(ctx, existingTask)

		err := e.scanOrphanEntriesTask(ctx)
		Expect(err).Should(BeNil())

		tasks, _ := testMeta.ListTask(ctx, maintainTaskIDEntryCleanup, types.ScheduledTaskFilter{RefID: orphanEntry.ID})
		Expect(len(tasks)).Should(Equal(1))
	})
})

var _ = Describe("entryCleanExecutor.createCleanupTask", func() {
	It("should create task when not exists", func() {
		ctx := context.TODO()
		testEntry := createTestEntry(ctx, "test-entry")

		e := &entryCleanExecutor{
			maintainExecutor: &maintainExecutor{
				recorder:  testMeta,
				metastore: testMeta,
				logger:    zap.NewNop().Sugar(),
			},
		}

		err := e.createCleanupTask(ctx, testEntry)
		Expect(err).Should(BeNil())

		tasks, err := testMeta.ListTask(ctx, maintainTaskIDEntryCleanup, types.ScheduledTaskFilter{RefID: testEntry.ID})
		Expect(err).Should(BeNil())
		Expect(len(tasks)).Should(Equal(1))
		Expect(tasks[0].Status).Should(Equal(types.ScheduledTaskWait))
	})

	It("should not create task when already exists", func() {
		ctx := context.TODO()
		testEntry := createTestEntry(ctx, "test-entry2")

		existingTask := &types.ScheduledTask{
			Namespace:      namespace,
			TaskID:         maintainTaskIDEntryCleanup,
			Status:         types.ScheduledTaskWait,
			RefType:        "entry",
			RefID:          testEntry.ID,
			CreatedTime:    time.Now(),
			ExpirationTime: time.Now().Add(time.Hour),
		}
		_ = testMeta.SaveTask(ctx, existingTask)

		e := &entryCleanExecutor{
			maintainExecutor: &maintainExecutor{
				recorder:  testMeta,
				metastore: testMeta,
				logger:    zap.NewNop().Sugar(),
			},
		}

		err := e.createCleanupTask(ctx, testEntry)
		Expect(err).Should(BeNil())

		tasks, _ := testMeta.ListTask(ctx, maintainTaskIDEntryCleanup, types.ScheduledTaskFilter{RefID: testEntry.ID})
		Expect(len(tasks)).Should(Equal(1))
	})
})

var _ = Describe("compactExecutor.handleEvent", func() {
	It("should create task when event matches", func() {
		e := &maintainExecutor{
			recorder:  testMeta,
			metastore: testMeta,
			logger:    zap.NewNop().Sugar(),
		}
		ce := &compactExecutor{maintainExecutor: e}

		ctx := context.TODO()
		testEntry := createTestEntry(ctx, "compact-test")

		evt := &types.Event{
			Type:      events.ActionTypeCompact,
			Namespace: namespace,
			RefType:   "file",
			RefID:     testEntry.ID,
		}

		err := ce.handleEvent(evt)
		Expect(err).Should(BeNil())

		tasks, err := testMeta.ListTask(ctx, maintainTaskIDChunkCompact, types.ScheduledTaskFilter{RefID: evt.RefID})
		Expect(err).Should(BeNil())
		Expect(len(tasks)).Should(Equal(1))
		Expect(tasks[0].Status).Should(Equal(types.ScheduledTaskWait))
	})

	It("should not create task when already exists", func() {
		e := &maintainExecutor{
			recorder:  testMeta,
			metastore: testMeta,
			logger:    zap.NewNop().Sugar(),
		}
		ce := &compactExecutor{maintainExecutor: e}

		ctx := context.TODO()
		testEntry := createTestEntry(ctx, "compact-test2")

		existingTask := &types.ScheduledTask{
			Namespace:      namespace,
			TaskID:         maintainTaskIDChunkCompact,
			Status:         types.ScheduledTaskWait,
			RefType:        "file",
			RefID:          testEntry.ID,
			CreatedTime:    time.Now(),
			ExpirationTime: time.Now().Add(time.Hour),
		}
		_ = testMeta.SaveTask(ctx, existingTask)

		evt := &types.Event{
			Type:      events.ActionTypeCompact,
			Namespace: namespace,
			RefType:   "file",
			RefID:     testEntry.ID,
		}

		err := ce.handleEvent(evt)
		Expect(err).Should(BeNil())

		tasks, _ := testMeta.ListTask(ctx, maintainTaskIDChunkCompact, types.ScheduledTaskFilter{RefID: evt.RefID})
		Expect(len(tasks)).Should(Equal(1))
	})
})

var _ = Describe("entryCleanExecutor.handleEvent", func() {
	It("should create task when event matches", func() {
		e := &maintainExecutor{
			recorder:  testMeta,
			metastore: testMeta,
			logger:    zap.NewNop().Sugar(),
		}
		ee := &entryCleanExecutor{maintainExecutor: e}

		ctx := context.TODO()
		testEntry := createTestEntry(ctx, "destroy-test")

		evt := &types.Event{
			Type:      events.ActionTypeRemove,
			Namespace: namespace,
			RefType:   "entry",
			RefID:     testEntry.ID,
		}

		err := ee.handleEvent(evt)
		Expect(err).Should(BeNil())

		tasks, err := testMeta.ListTask(ctx, maintainTaskIDEntryCleanup, types.ScheduledTaskFilter{RefID: evt.RefID})
		Expect(err).Should(BeNil())
		Expect(len(tasks)).Should(Equal(1))
		Expect(tasks[0].Status).Should(Equal(types.ScheduledTaskWait))
	})
})

var _ = Describe("compactExecutor.execute", func() {
	It("should succeed when compact completes", func() {
		e := &maintainExecutor{
			core:     fsCore,
			recorder: testMeta,
			logger:   zap.NewNop().Sugar(),
		}
		ce := &compactExecutor{maintainExecutor: e}

		ctx := context.TODO()
		testEntry := createTestEntry(ctx, "compact-exec-test")

		task := &types.ScheduledTask{
			Namespace: namespace,
			TaskID:    maintainTaskIDChunkCompact,
			Status:    types.ScheduledTaskWait,
			Event: types.Event{
				Namespace: namespace,
				RefType:   "entry",
				RefID:     testEntry.ID,
				Data: types.EventData{
					ID:      testEntry.ID,
					Kind:    testEntry.Kind,
					IsGroup: testEntry.IsGroup,
				},
			},
		}

		err := ce.execute(ctx, task)
		Expect(err).Should(BeNil())
	})
})

var _ = Describe("entryCleanExecutor.execute", func() {
	It("should succeed when cleanup completes", func() {
		e := &maintainExecutor{
			core:      fsCore,
			metastore: testMeta,
			recorder:  testMeta,
			logger:    zap.NewNop().Sugar(),
		}
		ee := &entryCleanExecutor{maintainExecutor: e}

		ctx := context.TODO()
		testEntry := createTestEntry(ctx, "clean-exec-test")

		task := &types.ScheduledTask{
			Namespace: namespace,
			TaskID:    maintainTaskIDEntryCleanup,
			Status:    types.ScheduledTaskWait,
			Event: types.Event{
				Namespace: namespace,
				RefType:   "entry",
				RefID:     testEntry.ID,
				Data: types.EventData{
					ID:      testEntry.ID,
					Kind:    testEntry.Kind,
					IsGroup: testEntry.IsGroup,
				},
			},
		}

		err := ee.execute(ctx, task)
		Expect(err).Should(BeNil())

		_, err = testMeta.GetEntry(ctx, namespace, testEntry.ID)
		Expect(err).ShouldNot(BeNil())
	})
})
