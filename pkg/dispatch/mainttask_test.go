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
	root, _ := fsCore.NamespaceRoot(ctx, namespace)
	entry, _ := fsCore.CreateEntry(ctx, namespace, root.ID, types.EntryAttr{
		Name: name,
		Kind: types.RawKind,
	})
	return entry
}

// makeOrphan creates an orphan entry with ref_count=0 by creating then removing from parent
func makeOrphan(ctx context.Context, name string) *types.Entry {
	root, _ := fsCore.NamespaceRoot(ctx, namespace)

	// Create entry with fsCore (ref_count=1)
	entry, _ := fsCore.CreateEntry(ctx, namespace, root.ID, types.EntryAttr{
		Name: name,
		Kind: types.RawKind,
	})

	// Remove from parent to set ref_count=0 (ChangedAt=now)
	_ = fsCore.RemoveEntry(ctx, namespace, root.ID, entry.ID, entry.Name, types.DeleteEntry{})

	// Sleep so ChangedAt becomes old enough for our 1-second threshold test
	time.Sleep(time.Second)

	GinkgoT().Logf("Created orphan: ID=%d, RefCount=0", entry.ID)
	return entry
}

var _ = Describe("entryCleanExecutor.scanOrphanEntriesTask", func() {
	It("should create cleanup task for orphan entries", func() {
		ctx := context.TODO()

		// Create orphan entry (ref_count=0, ChangedAt is old due to sleep in makeOrphan)
		orphanEntry := makeOrphan(ctx, "orphan")
		Expect(orphanEntry).ShouldNot(BeNil())

		// Use 1-second threshold (entry is already old due to sleep in makeOrphan)
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

		// Verify cleanup task was created for OUR entry
		tasks, err := testMeta.ListTask(ctx, maintainTaskIDEntryCleanup, types.ScheduledTaskFilter{RefID: orphanEntry.ID})
		Expect(err).Should(BeNil())
		Expect(len(tasks)).Should(Equal(1), "should create task for our orphan entry")
		Expect(tasks[0].RefID).Should(Equal(orphanEntry.ID))
	})

	It("should not create duplicate task", func() {
		ctx := context.TODO()

		// Create orphan entry
		orphanEntry := makeOrphan(ctx, "orphan2")
		Expect(orphanEntry).ShouldNot(BeNil())

		// Use 1-second threshold
		e := &entryCleanExecutor{
			maintainExecutor: &maintainExecutor{
				recorder:  testMeta,
				metastore: testMeta,
				logger:    zap.NewNop().Sugar(),
			},
			orphanThreshold: time.Second,
		}

		// Create existing task for OUR entry
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

		// Scan should not create duplicate for OUR entry
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

		// Create existing task
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
			RefType:   "entry",
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

		// Create existing task
		existingTask := &types.ScheduledTask{
			Namespace:      namespace,
			TaskID:         maintainTaskIDChunkCompact,
			Status:         types.ScheduledTaskWait,
			RefType:        "entry",
			RefID:          testEntry.ID,
			CreatedTime:    time.Now(),
			ExpirationTime: time.Now().Add(time.Hour),
		}
		_ = testMeta.SaveTask(ctx, existingTask)

		evt := &types.Event{
			Type:      events.ActionTypeCompact,
			Namespace: namespace,
			RefType:   "entry",
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
			Type:      events.ActionTypeDestroy,
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
				Data:      types.NewEventDataFromEntry(testEntry),
			},
		}

		err := ce.execute(ctx, task)
		Expect(err).Should(BeNil())
	})
})

var _ = Describe("entryCleanExecutor.execute", func() {
	It("should succeed when cleanup completes", func() {
		e := &maintainExecutor{
			core:     fsCore,
			recorder: testMeta,
			logger:   zap.NewNop().Sugar(),
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
				Data:      types.NewEventDataFromEntry(testEntry),
			},
		}

		err := ee.execute(ctx, task)
		Expect(err).Should(BeNil())

		// Verify entry was destroyed
		_, err = fsCore.GetEntry(ctx, namespace, testEntry.ID)
		Expect(err).ShouldNot(BeNil())
	})
})
