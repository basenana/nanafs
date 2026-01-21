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

package search

import (
	"context"
	"os"
	"path"
	"testing"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"

	"github.com/glebarez/sqlite"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gorm.io/gorm"
)

const SqliteMeta = "sqlite"

var (
	testWorkDir string
	testDB      *gorm.DB
	testNs1     = "test-namespace-1"
	testNs2     = "test-namespace-2"
)

func TestSqliteSearch(t *testing.T) {
	RegisterFailHandler(Fail)

	logger.InitLogger()
	defer logger.Sync()
	logger.SetDebug(true)

	var err error
	testWorkDir, err = os.MkdirTemp(os.TempDir(), "ut-nanafs-search-")
	Expect(err).Should(BeNil())
	t.Logf("search test workdir on: %s", testWorkDir)

	RunSpecs(t, "Sqlite Search Suite")
}

var _ = BeforeSuite(func() {
	db, err := gorm.Open(sqlite.Open(path.Join(testWorkDir, "test.db")), &gorm.Config{})
	Expect(err).Should(BeNil())
	testDB = db

	// Create main table
	mainTable := `CREATE TABLE IF NOT EXISTS documents (
		id INTEGER PRIMARY KEY,
		uri TEXT NOT NULL,
		namespace TEXT NOT NULL,
		title TEXT,
		content TEXT,
		created_at INTEGER,
		changed_at INTEGER
	)`
	err = db.Exec(mainTable).Error
	Expect(err).Should(BeNil())

	// Create composite index on (uri, namespace)
	err = db.Exec(`CREATE INDEX IF NOT EXISTS sltdoc_uri_ns_idx ON documents(uri, namespace)`).Error
	Expect(err).Should(BeNil())

	// Create FTS5 virtual table with Unicode tokenization
	// Use contentless FTS5 for manual index management
	ftsTable := `CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(
		title, content,
		tokenize='unicode61'
	)`
	err = db.Exec(ftsTable).Error
	Expect(err).Should(BeNil())
})

var _ = Describe("SqliteIndexDocument", func() {
	Context("index a document", func() {
		It("should index document successfully", func() {
			doc := &types.IndexDocument{
				ID:        1,
				URI:       "test://doc1",
				Title:     "Hello World",
				Content:   "This is a test document",
				CreateAt:  0,
				ChangedAt: 0,
			}
			err := SqliteIndexDocument(context.TODO(), testDB, testNs1, doc)
			Expect(err).Should(BeNil())
		})

		It("should index multiple documents", func() {
			docs := []*types.IndexDocument{
				{
					ID:        2,
					URI:       "test://doc2",
					Title:     "Golang Tutorial",
					Content:   "Learn Go programming language",
					CreateAt:  0,
					ChangedAt: 0,
				},
				{
					ID:        3,
					URI:       "test://doc3",
					Title:     "Database Systems",
					Content:   "SQL and NoSQL databases",
					CreateAt:  0,
					ChangedAt: 0,
				},
			}
			for _, doc := range docs {
				err := SqliteIndexDocument(context.TODO(), testDB, testNs1, doc)
				Expect(err).Should(BeNil())
			}
		})

		It("should return error for empty id", func() {
			doc := &types.IndexDocument{
				ID:        0,
				URI:       "test://doc-no-id",
				Title:     "Test",
				Content:   "Content",
				CreateAt:  0,
				ChangedAt: 0,
			}
			err := SqliteIndexDocument(context.TODO(), testDB, testNs1, doc)
			Expect(err).ShouldNot(BeNil())
		})

		It("should return error for empty uri", func() {
			doc := &types.IndexDocument{
				ID:        100,
				URI:       "",
				Title:     "Test",
				Content:   "Content",
				CreateAt:  0,
				ChangedAt: 0,
			}
			err := SqliteIndexDocument(context.TODO(), testDB, testNs1, doc)
			Expect(err).ShouldNot(BeNil())
		})
	})
})

var _ = Describe("SqliteQueryLanguage", func() {
	BeforeEach(func() {
		testDB.Exec("DELETE FROM documents")
	})

	Context("search documents", func() {
		It("should find documents matching single keyword", func() {
			docs := []*types.IndexDocument{
				{
					ID:        10,
					URI:       "test://doc10",
					Title:     "Hello World",
					Content:   "First document",
					CreateAt:  0,
					ChangedAt: 0,
				},
				{
					ID:        11,
					URI:       "test://doc11",
					Title:     "Goodbye World",
					Content:   "Second document",
					CreateAt:  0,
					ChangedAt: 0,
				},
			}
			for _, doc := range docs {
				err := SqliteIndexDocument(context.TODO(), testDB, testNs1, doc)
				Expect(err).Should(BeNil())
			}

			results, err := SqliteQueryLanguage(context.TODO(), testDB, testNs1, "Hello")
			Expect(err).Should(BeNil())
			Expect(len(results)).Should(Equal(1))
			Expect(results[0].ID).Should(Equal(int64(10)))
		})

		It("should find documents matching multiple keywords", func() {
			docs := []*types.IndexDocument{
				{
					ID:        20,
					URI:       "test://doc20",
					Title:     "Go Programming",
					Content:   "Learning Golang",
					CreateAt:  0,
					ChangedAt: 0,
				},
				{
					ID:        21,
					URI:       "test://doc21",
					Title:     "Python Programming",
					Content:   "Learning Python",
					CreateAt:  0,
					ChangedAt: 0,
				},
			}
			for _, doc := range docs {
				SqliteIndexDocument(context.TODO(), testDB, testNs1, doc)
			}

			results, err := SqliteQueryLanguage(context.TODO(), testDB, testNs1, "Programming")
			Expect(err).Should(BeNil())
			Expect(len(results)).Should(Equal(2))
		})

		It("should search in both title and content", func() {
			docs := []*types.IndexDocument{
				{
					ID:        30,
					URI:       "test://doc30",
					Title:     "Database",
					Content:   "PostgreSQL is great",
					CreateAt:  0,
					ChangedAt: 0,
				},
				{
					ID:        31,
					URI:       "test://doc31",
					Title:     "PostgreSQL Guide",
					Content:   "Database tutorial",
					CreateAt:  0,
					ChangedAt: 0,
				},
			}
			for _, doc := range docs {
				SqliteIndexDocument(context.TODO(), testDB, testNs1, doc)
			}

			results, err := SqliteQueryLanguage(context.TODO(), testDB, testNs1, "PostgreSQL")
			Expect(err).Should(BeNil())
			Expect(len(results)).Should(Equal(2))
		})

		It("should return empty for no matches", func() {
			doc := &types.IndexDocument{
				ID:        40,
				URI:       "test://doc40",
				Title:     "Test Document",
				Content:   "Some content",
				CreateAt:  0,
				ChangedAt: 0,
			}
			SqliteIndexDocument(context.TODO(), testDB, testNs1, doc)

			results, err := SqliteQueryLanguage(context.TODO(), testDB, testNs1, "NonExistent")
			Expect(err).Should(BeNil())
			Expect(len(results)).Should(Equal(0))
		})

		It("should return empty for empty query", func() {
			doc := &types.IndexDocument{
				ID:        50,
				URI:       "test://doc50",
				Title:     "Test",
				Content:   "Content",
				CreateAt:  0,
				ChangedAt: 0,
			}
			SqliteIndexDocument(context.TODO(), testDB, testNs1, doc)

			results, err := SqliteQueryLanguage(context.TODO(), testDB, testNs1, "")
			Expect(err).Should(BeNil())
			Expect(len(results)).Should(Equal(0))
		})

		It("should filter by namespace", func() {
			doc1 := &types.IndexDocument{
				ID:        60,
				URI:       "test://doc60",
				Title:     "Shared Title",
				Content:   "Content in ns1",
				CreateAt:  0,
				ChangedAt: 0,
			}
			doc2 := &types.IndexDocument{
				ID:        61,
				URI:       "test://doc61",
				Title:     "Shared Title",
				Content:   "Content in ns2",
				CreateAt:  0,
				ChangedAt: 0,
			}
			SqliteIndexDocument(context.TODO(), testDB, testNs1, doc1)
			SqliteIndexDocument(context.TODO(), testDB, testNs2, doc2)

			results1, err := SqliteQueryLanguage(context.TODO(), testDB, testNs1, "Shared")
			Expect(err).Should(BeNil())
			Expect(len(results1)).Should(Equal(1))
			Expect(results1[0].ID).Should(Equal(int64(60)))

			results2, err := SqliteQueryLanguage(context.TODO(), testDB, testNs2, "Shared")
			Expect(err).Should(BeNil())
			Expect(len(results2)).Should(Equal(1))
			Expect(results2[0].ID).Should(Equal(int64(61)))
		})
	})
})

var _ = Describe("Search package interface", func() {
	BeforeEach(func() {
		testDB.Exec("DELETE FROM documents")
	})

	Context("IndexDocument through interface", func() {
		It("should use sqlite implementation", func() {
			doc := &types.IndexDocument{
				ID:        80,
				URI:       "test://doc80",
				Title:     "Interface Test",
				Content:   "Testing interface",
				CreateAt:  0,
				ChangedAt: 0,
			}
			err := IndexDocument(context.TODO(), testDB, testNs1, doc)
			Expect(err).Should(BeNil())
		})
	})

	Context("QueryLanguage through interface", func() {
		It("should use sqlite implementation", func() {
			doc := &types.IndexDocument{
				ID:        90,
				URI:       "test://doc90",
				Title:     "Interface Query",
				Content:   "Query testing",
				CreateAt:  0,
				ChangedAt: 0,
			}
			SqliteIndexDocument(context.TODO(), testDB, testNs1, doc)

			results, err := QueryLanguage(context.TODO(), testDB, testNs1, "Interface")
			Expect(err).Should(BeNil())
			Expect(len(results)).Should(Equal(1))
		})
	})
})

var _ = Describe("Config based store", func() {
	BeforeEach(func() {
		testDB.Exec("DELETE FROM documents")
	})

	Context("new sqlite metastore", func() {
		It("should create store successfully", func() {
			store, err := newSqliteSearchStore(config.Meta{
				Type: "sqlite",
				Path: path.Join(testWorkDir, "store.db"),
			})
			Expect(err).Should(BeNil())
			Expect(store).ShouldNot(BeNil())
		})
	})
})

func newSqliteSearchStore(meta config.Meta) (*sqliteSearchStore, error) {
	db, err := gorm.Open(sqlite.Open(meta.Path), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return &sqliteSearchStore{db: db}, nil
}

type sqliteSearchStore struct {
	db *gorm.DB
}

var _ = Describe("SqliteDeleteDocument", func() {
	BeforeEach(func() {
		testDB.Exec("DELETE FROM documents")
	})

	Context("delete documents", func() {
		It("should delete document successfully", func() {
			doc := &types.IndexDocument{
				ID:        200,
				URI:       "test://doc200",
				Title:     "Delete Test",
				Content:   "Content to delete",
				CreateAt:  0,
				ChangedAt: 0,
			}
			err := SqliteIndexDocument(context.TODO(), testDB, testNs1, doc)
			Expect(err).Should(BeNil())

			err = SqliteDeleteDocument(context.TODO(), testDB, testNs1, 200)
			Expect(err).Should(BeNil())

			results, err := SqliteQueryLanguage(context.TODO(), testDB, testNs1, "Delete")
			Expect(err).Should(BeNil())
			Expect(len(results)).Should(Equal(0))
		})

		It("should return error for non-existent document", func() {
			err := SqliteDeleteDocument(context.TODO(), testDB, testNs1, 9999)
			Expect(err).ShouldNot(BeNil())
		})

		It("should not delete document from other namespace", func() {
			doc := &types.IndexDocument{
				ID:        210,
				URI:       "test://doc210",
				Title:     "Namespace Test",
				Content:   "Content",
				CreateAt:  0,
				ChangedAt: 0,
			}
			SqliteIndexDocument(context.TODO(), testDB, testNs1, doc)

			err := SqliteDeleteDocument(context.TODO(), testDB, testNs2, 210)
			Expect(err).ShouldNot(BeNil())
		})
	})
})

var _ = Describe("SqliteIndexDocument Upsert", func() {
	BeforeEach(func() {
		testDB.Exec("DELETE FROM documents")
		testDB.Exec("DELETE FROM documents_fts")
	})

	Context("update existing document", func() {
		It("should update document content", func() {
			doc := &types.IndexDocument{
				ID:        300,
				URI:       "test://doc300",
				Title:     "Old Document",
				Content:   "Original Content",
				CreateAt:  0,
				ChangedAt: 0,
			}
			SqliteIndexDocument(context.TODO(), testDB, testNs1, doc)

			// Update the document
			doc.Content = "Updated Content"
			SqliteIndexDocument(context.TODO(), testDB, testNs1, doc)

			results, err := SqliteQueryLanguage(context.TODO(), testDB, testNs1, "Updated")
			Expect(err).Should(BeNil())
			Expect(len(results)).Should(Equal(1))

			results2, err := SqliteQueryLanguage(context.TODO(), testDB, testNs1, "Original")
			Expect(err).Should(BeNil())
			Expect(len(results2)).Should(Equal(0))
		})

		It("should update document title", func() {
			doc := &types.IndexDocument{
				ID:        310,
				URI:       "test://doc310",
				Title:     "Old Title",
				Content:   "Some content",
				CreateAt:  0,
				ChangedAt: 0,
			}
			SqliteIndexDocument(context.TODO(), testDB, testNs1, doc)

			// Update the title
			doc.Title = "New Title"
			SqliteIndexDocument(context.TODO(), testDB, testNs1, doc)

			results, err := SqliteQueryLanguage(context.TODO(), testDB, testNs1, "New")
			Expect(err).Should(BeNil())
			Expect(len(results)).Should(Equal(1))

			results2, err := SqliteQueryLanguage(context.TODO(), testDB, testNs1, "Old")
			Expect(err).Should(BeNil())
			Expect(len(results2)).Should(Equal(0))
		})
	})
})
