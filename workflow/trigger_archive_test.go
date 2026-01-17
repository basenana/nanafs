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

	"github.com/basenana/nanafs/pkg/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

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
