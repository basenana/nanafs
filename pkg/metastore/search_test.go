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

package metastore

import (
	"context"

	"github.com/basenana/nanafs/pkg/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TestSearchOperation", func() {
	var sqlite = buildNewSqliteMetaStore("test_search.db")
	rootEn := InitRootEntry()
	Expect(sqlite.CreateEntry(context.TODO(), namespace, 0, rootEn)).Should(BeNil())

	BeforeEach(func() {
		sqlite.WithContext(context.TODO()).Exec("DELETE FROM documents")
	})

	Context("index and query documents", func() {
		It("should index document successfully", func() {
			doc := &types.IndexDocument{
				ID:        1001,
				URI:       "test://search-doc-1",
				Title:     "Test Document Title",
				Content:   "This is test content for search",
				CreateAt:  0,
				ChangedAt: 0,
			}
			err := sqlite.Index(context.TODO(), namespace, doc)
			Expect(err).Should(BeNil())
		})

		It("should index multiple documents", func() {
			docs := []*types.IndexDocument{
				{
					ID:        1002,
					URI:       "test://search-doc-2",
					Title:     "Golang Tutorial",
					Content:   "Learn Go programming language",
					CreateAt:  0,
					ChangedAt: 0,
				},
				{
					ID:        1003,
					URI:       "test://search-doc-3",
					Title:     "Database Systems",
					Content:   "SQL and NoSQL databases",
					CreateAt:  0,
					ChangedAt: 0,
				},
			}
			for _, doc := range docs {
				err := sqlite.Index(context.TODO(), namespace, doc)
				Expect(err).Should(BeNil())
			}
		})

		It("should query documents by keyword", func() {
			doc := &types.IndexDocument{
				ID:        1010,
				URI:       "test://search-doc-10",
				Title:     "Searchable Document",
				Content:   "Content with searchable keywords",
				CreateAt:  0,
				ChangedAt: 0,
			}
			err := sqlite.Index(context.TODO(), namespace, doc)
			Expect(err).Should(BeNil())

			results, err := sqlite.QueryLanguage(context.TODO(), namespace, "searchable")
			Expect(err).Should(BeNil())
			Expect(len(results)).Should(Equal(1))
			Expect(results[0].ID).Should(Equal(int64(1010)))
		})

		It("should query documents with multiple keywords", func() {
			docs := []*types.IndexDocument{
				{
					ID:        1020,
					URI:       "test://search-doc-20",
					Title:     "Go Programming",
					Content:   "Learning Golang",
					CreateAt:  0,
					ChangedAt: 0,
				},
				{
					ID:        1021,
					URI:       "test://search-doc-21",
					Title:     "Python Programming",
					Content:   "Learning Python",
					CreateAt:  0,
					ChangedAt: 0,
				},
			}
			for _, doc := range docs {
				err := sqlite.Index(context.TODO(), namespace, doc)
				Expect(err).Should(BeNil())
			}

			results, err := sqlite.QueryLanguage(context.TODO(), namespace, "Programming")
			Expect(err).Should(BeNil())
			Expect(len(results)).Should(Equal(2))
		})

		It("should return empty for no matches", func() {
			doc := &types.IndexDocument{
				ID:        1030,
				URI:       "test://search-doc-30",
				Title:     "Test",
				Content:   "Content",
				CreateAt:  0,
				ChangedAt: 0,
			}
			err := sqlite.Index(context.TODO(), namespace, doc)
			Expect(err).Should(BeNil())

			results, err := sqlite.QueryLanguage(context.TODO(), namespace, "NonExistentKeyword")
			Expect(err).Should(BeNil())
			Expect(len(results)).Should(Equal(0))
		})

		It("should return empty for empty query", func() {
			doc := &types.IndexDocument{
				ID:        1040,
				URI:       "test://search-doc-40",
				Title:     "Test",
				Content:   "Content",
				CreateAt:  0,
				ChangedAt: 0,
			}
			err := sqlite.Index(context.TODO(), namespace, doc)
			Expect(err).Should(BeNil())

			results, err := sqlite.QueryLanguage(context.TODO(), namespace, "")
			Expect(err).Should(BeNil())
			Expect(len(results)).Should(Equal(0))
		})

		It("should filter by namespace", func() {
			doc1 := &types.IndexDocument{
				ID:        1050,
				URI:       "test://search-doc-50",
				Title:     "Shared Title",
				Content:   "Content in ns1",
				CreateAt:  0,
				ChangedAt: 0,
			}
			doc2 := &types.IndexDocument{
				ID:        1051,
				URI:       "test://search-doc-51",
				Title:     "Shared Title",
				Content:   "Content in ns2",
				CreateAt:  0,
				ChangedAt: 0,
			}
			err := sqlite.Index(context.TODO(), namespace, doc1)
			Expect(err).Should(BeNil())
			err = sqlite.Index(context.TODO(), "other-namespace", doc2)
			Expect(err).Should(BeNil())

			results, err := sqlite.QueryLanguage(context.TODO(), namespace, "Shared")
			Expect(err).Should(BeNil())
			Expect(len(results)).Should(Equal(1))
			Expect(results[0].ID).Should(Equal(int64(1050)))
		})

		It("should search in both title and content", func() {
			doc := &types.IndexDocument{
				ID:        1070,
				URI:       "test://search-doc-70",
				Title:     "Hidden Word",
				Content:   "The secret word is visible",
				CreateAt:  0,
				ChangedAt: 0,
			}
			err := sqlite.Index(context.TODO(), namespace, doc)
			Expect(err).Should(BeNil())

			titleResults, err := sqlite.QueryLanguage(context.TODO(), namespace, "Hidden")
			Expect(err).Should(BeNil())
			Expect(len(titleResults)).Should(Equal(1))

			contentResults, err := sqlite.QueryLanguage(context.TODO(), namespace, "secret")
			Expect(err).Should(BeNil())
			Expect(len(contentResults)).Should(Equal(1))
		})
	})
})
