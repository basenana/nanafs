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

package workflow

import (
	"context"
	"time"

	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TestTriggers", func() {
	var (
		mockMgr *mockWorkflow
		t       *triggers
		ctx     context.Context
	)

	BeforeEach(func() {
		logger.InitLogger()
		mockMgr = newMockWorkflow()
		mockMgr.triggerCalled = false
		t = initTriggers(mockMgr, fsCore, testMeta)
		t.timerTool = mockTimerTool{}
		ctx = context.Background()
	})

	Describe("handleWorkflowUpdate", func() {
		It("should add workflow with interval trigger", func() {
			wf := &types.Workflow{
				Id:        "wf-1",
				Namespace: "test-ns",
				Name:      "test-workflow",
				Enable:    true,
				Trigger: types.WorkflowTrigger{
					Interval: utils.ToPtr(5),
					LocalFileWatch: &types.WorkflowLocalFileWatch{
						Event: events.ActionTypeCreate,
					},
				},
			}

			t.handleWorkflowUpdate(wf, false)
			Expect(t.getTriggerConfig("test-ns", "wf-1")).ShouldNot(BeNil())
		})

		It("should skip update when hash unchanged", func() {
			wf := &types.Workflow{
				Id:        "wf-2",
				Namespace: "test-ns",
				Name:      "test-workflow-2",
				Enable:    true,
				Trigger: types.WorkflowTrigger{
					Interval: utils.ToPtr(5),
				},
			}

			t.handleWorkflowUpdate(wf, false)
			t.handleWorkflowUpdate(wf, false)
			Expect(t.getTriggerConfig("test-ns", "wf-2")).ShouldNot(BeNil())
		})

		It("should remove trigger when workflow is disabled", func() {
			wf := &types.Workflow{
				Id:        "wf-3",
				Namespace: "test-ns",
				Name:      "test-workflow-3",
				Enable:    true,
				Trigger: types.WorkflowTrigger{
					Interval: utils.ToPtr(10),
				},
			}

			t.handleWorkflowUpdate(wf, false)
			Expect(t.getTriggerConfig("test-ns", "wf-3")).ShouldNot(BeNil())

			wf.Enable = false
			t.handleWorkflowUpdate(wf, false)
			Expect(t.getTriggerConfig("test-ns", "wf-3")).Should(BeNil())
		})

		It("should not add trigger when no triggers configured", func() {
			wf := &types.Workflow{
				Id:        "wf-4",
				Namespace: "test-ns",
				Name:      "test-workflow-4",
				Enable:    true,
				Trigger:   types.WorkflowTrigger{},
			}

			t.handleWorkflowUpdate(wf, false)
			Expect(t.getTriggerConfig("test-ns", "wf-4")).Should(BeNil())
		})

		It("should remove trigger when workflow is deleted", func() {
			wf := &types.Workflow{
				Id:        "wf-5",
				Namespace: "test-ns",
				Name:      "test-workflow-5",
				Enable:    true,
				Trigger: types.WorkflowTrigger{
					Interval: utils.ToPtr(15),
				},
			}

			t.handleWorkflowUpdate(wf, false)
			Expect(t.getTriggerConfig("test-ns", "wf-5")).ShouldNot(BeNil())

			t.handleWorkflowUpdate(wf, true)
			Expect(t.getTriggerConfig("test-ns", "wf-5")).Should(BeNil())
		})
	})

	Describe("runRSSWorkflow", func() {
		It("should trigger workflow for rss groups", func() {
			// create rss group with properties
			rssGroup, err := fsCore.CreateEntry(ctx, namespace, "/", types.EntryAttr{
				Name: "rss-group",
				Kind: types.GroupKind,
				GroupProperties: &types.GroupProperties{
					Source: "rss",
					RSS: &types.GroupRSS{
						Feed: "https://example.com/feed.xml",
					},
				},
			})
			Expect(err).Should(BeNil())
			Expect(rssGroup).ShouldNot(BeNil())

			wf := &types.Workflow{
				Id:        "wf-rss-trigger",
				Namespace: namespace,
				Name:      "rss-trigger-workflow",
				Enable:    true,
				Trigger: types.WorkflowTrigger{
					RSS: &types.WorkflowRssTrigger{},
				},
			}
			t.handleWorkflowUpdate(wf, false)

			t.runRSSWorkflow(ctx, namespace, "wf-rss-trigger", wf.Trigger.RSS)
			Expect(mockMgr.triggerCalled).Should(BeTrue())
		})
	})
})

var _ = Describe("RSS Group Archive", func() {
	var (
		ctx       context.Context
		t         *triggers
		namespace = types.DefaultNamespace
	)

	BeforeEach(func() {
		ctx = context.Background()
		t = initTriggers(mgr, fsCore, testMeta)
		t.timerTool = mockTimerTool{}
	})

	Describe("extractPublishYear", func() {
		It("should extract year from PublishAt", func() {
			// Arrange
			entry, err := fsCore.CreateEntry(ctx, namespace, "/", types.EntryAttr{
				Name: "article-2024.md",
				Kind: types.TextKind,
			})
			Expect(err).Should(BeNil())

			docProps := types.DocumentProperties{
				Title:     "Test Article",
				PublishAt: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC).Unix(),
			}
			err = testMeta.UpdateEntryProperties(ctx, namespace, types.PropertyTypeDocument, entry.ID, docProps)
			Expect(err).Should(BeNil())

			// Act
			fullEntry, _ := fsCore.GetEntry(ctx, namespace, entry.ID)
			year := t.extractPublishYear(ctx, namespace, fullEntry)

			// Assert
			Expect(year).To(Equal(2024))
		})

		It("should fallback to CreatedAt when PublishAt is zero", func() {
			// Arrange
			entry, err := fsCore.CreateEntry(ctx, namespace, "/", types.EntryAttr{
				Name: "article-no-date.md",
				Kind: types.TextKind,
			})
			Expect(err).Should(BeNil())

			// Act
			fullEntry, _ := fsCore.GetEntry(ctx, namespace, entry.ID)
			year := t.extractPublishYear(ctx, namespace, fullEntry)

			// Assert
			Expect(year).To(Equal(time.Now().Year()))
		})
	})

	Describe("archiveRSSGroupEntries", func() {
		It("should skip archive when last archive was recent", func() {
			// Arrange
			rssGroup, err := fsCore.CreateEntry(ctx, namespace, "/", types.EntryAttr{
				Name: "RSS-RecentArchive",
				Kind: types.GroupKind,
				GroupProperties: &types.GroupProperties{
					Source: "rss",
					RSS: &types.GroupRSS{
						Feed:           "https://example.com/feed.xml",
						LastArchivedAt: time.Now().Add(-24 * time.Hour).Unix(),
					},
				},
			})
			Expect(err).Should(BeNil())

			// Act
			gp := &types.GroupProperties{
				Source: "rss",
				RSS: &types.GroupRSS{
					Feed:           "https://example.com/feed.xml",
					LastArchivedAt: time.Now().Add(-24 * time.Hour).Unix(),
				},
			}
			t.archiveRSSGroupEntries(ctx, namespace, "/RSS-RecentArchive", rssGroup, gp)

			year2024Entry, _, _ := fsCore.GetEntryByPath(ctx, namespace, "/RSS-RecentArchive/2024")
			Expect(year2024Entry).Should(BeNil())
		})

		It("should handle when year dir already exists", func() {
			// Arrange
			rssGroup, err := fsCore.CreateEntry(ctx, namespace, "/", types.EntryAttr{
				Name: "RSS-ExistingYearDir",
				Kind: types.GroupKind,
				GroupProperties: &types.GroupProperties{
					Source: "rss",
					RSS: &types.GroupRSS{
						Feed:           "https://example.com/feed.xml",
						LastArchivedAt: time.Now().Add(-40 * 24 * time.Hour).Unix(),
					},
				},
			})
			Expect(err).Should(BeNil())

			_, err = fsCore.CreateEntry(ctx, namespace, "/RSS-ExistingYearDir", types.EntryAttr{
				Name: "2024",
				Kind: types.GroupKind,
			})
			Expect(err).Should(BeNil())

			article, err := fsCore.CreateEntry(ctx, namespace, "/RSS-ExistingYearDir", types.EntryAttr{
				Name: "article.md",
				Kind: types.TextKind,
			})
			Expect(err).Should(BeNil())

			err = testMeta.UpdateEntryProperties(ctx, namespace, types.PropertyTypeDocument, article.ID, types.DocumentProperties{
				Title:     "Article",
				PublishAt: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC).Unix(),
			})
			Expect(err).Should(BeNil())

			gp := &types.GroupProperties{
				Source: "rss",
				RSS: &types.GroupRSS{
					Feed:           "https://example.com/feed.xml",
					LastArchivedAt: time.Now().Add(-40 * 24 * time.Hour).Unix(),
				},
			}
			t.archiveRSSGroupEntries(ctx, namespace, "/RSS-ExistingYearDir", rssGroup, gp)

			articleEntry, _, _ := fsCore.GetEntryByPath(ctx, namespace, "/RSS-ExistingYearDir/article.md")
			Expect(articleEntry).Should(BeNil())

			movedArticle, _, _ := fsCore.GetEntryByPath(ctx, namespace, "/RSS-ExistingYearDir/2024/article.md")
			Expect(movedArticle).ShouldNot(BeNil())
		})
	})
})

// Mock implementations

type mockWorkflow struct {
	Workflow
	triggerCalled bool
}

func newMockWorkflow() *mockWorkflow {
	return &mockWorkflow{}
}

func (m *mockWorkflow) TriggerWorkflow(ctx context.Context, namespace string, wfId string, tgt types.WorkflowTarget, attr JobAttr) (*types.WorkflowJob, error) {
	m.triggerCalled = true
	return &types.WorkflowJob{
		Id:        "job-1",
		Namespace: namespace,
		Workflow:  wfId,
	}, nil
}

type mockTimerTool struct{}

func (m mockTimerTool) NewTimer(d time.Duration) *time.Ticker {
	return time.NewTicker(time.Duration(d.Minutes()) * time.Second)
}

func (m mockTimerTool) ResetTimer(min int, t *time.Ticker) {
	t.Reset(time.Duration(min) * time.Second)
}
