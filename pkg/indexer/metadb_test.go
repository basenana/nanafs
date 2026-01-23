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

package indexer

import (
	"context"
	"path"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func newTestMetaDB(dbName string) (metastore.Meta, Indexer, error) {
	meta, err := metastore.NewMetaStorage(metastore.SqliteMeta, config.Meta{
		Type: config.SqliteMeta,
		Path: path.Join(workdir, dbName),
	})
	if err != nil {
		return nil, nil, err
	}
	idx, err := NewMetaDB(meta, NewSpaceTokenizer())
	if err != nil {
		return nil, nil, err
	}
	return meta, idx, nil
}

var _ = Describe("MetaDB UpdateURI", func() {
	Context("update document URI", func() {
		It("should update document URI successfully", func() {
			_, idx, err := newTestMetaDB("test_update_uri.db")
			Expect(err).Should(BeNil())

			doc := &types.IndexDocument{
				ID:      1,
				URI:     "/old/path",
				Title:   "Test Document",
				Content: "Test content",
			}
			err = idx.Index(context.TODO(), namespace, doc)
			Expect(err).Should(BeNil())

			err = idx.UpdateURI(context.TODO(), namespace, 1, "/new/path")
			Expect(err).Should(BeNil())

			results, err := idx.QueryLanguage(context.TODO(), namespace, "Test")
			Expect(err).Should(BeNil())
			Expect(len(results)).Should(Equal(1))
			Expect(results[0].URI).Should(Equal("/new/path"))
		})

		It("should return error for non-existent document", func() {
			_, idx, err := newTestMetaDB("test_update_uri_err.db")
			Expect(err).Should(BeNil())

			err = idx.UpdateURI(context.TODO(), namespace, 9999, "/new/path")
			Expect(err).ShouldNot(BeNil())
		})
	})
})

var _ = Describe("MetaDB DeleteChildren", func() {
	Context("delete children documents", func() {
		It("should delete all children documents recursively", func() {
			meta, idx, err := newTestMetaDB("test_delete_children.db")
			Expect(err).Should(BeNil())

			parentID := setupTestEntriesForDelete(meta, namespace)

			err = idx.DeleteChildren(context.TODO(), namespace, parentID)
			Expect(err).Should(BeNil())

			results, err := idx.QueryLanguage(context.TODO(), namespace, "child")
			Expect(err).Should(BeNil())
			Expect(len(results)).Should(Equal(0))
		})

		It("should not affect documents outside the parent", func() {
			meta, idx, err := newTestMetaDB("test_delete_partial.db")
			Expect(err).Should(BeNil())

			parentID := setupTestEntriesForDelete(meta, namespace)

			err = idx.DeleteChildren(context.TODO(), namespace, parentID+100)
			Expect(err).Should(BeNil())

			results, err := idx.QueryLanguage(context.TODO(), namespace, "doc1")
			Expect(err).Should(BeNil())
			Expect(len(results)).Should(Equal(1))

			results, err = idx.QueryLanguage(context.TODO(), namespace, "doc2")
			Expect(err).Should(BeNil())
			Expect(len(results)).Should(Equal(1))
		})
	})
})

var _ = Describe("MetaDB UpdateChildrenURI", func() {
	Context("update children URI recursively", func() {
		It("should update all children URIs", func() {
			meta, idx, err := newTestMetaDB("test_update_children.db")
			Expect(err).Should(BeNil())

			rootEn := initRootEntry()
			err = meta.CreateEntry(context.TODO(), namespace, 0, rootEn)
			Expect(err).Should(BeNil())

			parent := types.NewEntry("parent", types.GroupKind)
			err = meta.CreateEntry(context.TODO(), namespace, rootEn.ID, &parent)
			Expect(err).Should(BeNil())

			child1 := types.NewEntry("child1", types.RawKind)
			err = meta.CreateEntry(context.TODO(), namespace, parent.ID, &child1)
			Expect(err).Should(BeNil())
			meta.IndexDocument(context.TODO(), namespace, &types.IndexDocument{
				ID:      child1.ID,
				URI:     "/root/parent/child1",
				Title:   "Child One",
				Content: "Content of child1",
			}, NewSpaceTokenizer().Tokenize)

			err = idx.UpdateChildrenURI(context.TODO(), namespace, parent.ID, "/new/parent")
			Expect(err).Should(BeNil())

			results, err := idx.QueryLanguage(context.TODO(), namespace, "child1")
			Expect(err).Should(BeNil())
			Expect(len(results)).Should(Equal(1))
			Expect(results[0].URI).Should(Equal("/new/parent/child1"))
		})

		It("should update nested children URIs", func() {
			meta, idx, err := newTestMetaDB("test_update_nested.db")
			Expect(err).Should(BeNil())

			rootEn := initRootEntry()
			err = meta.CreateEntry(context.TODO(), namespace, 0, rootEn)
			Expect(err).Should(BeNil())

			parent := types.NewEntry("parent", types.GroupKind)
			err = meta.CreateEntry(context.TODO(), namespace, rootEn.ID, &parent)
			Expect(err).Should(BeNil())

			child2 := types.NewEntry("child2", types.GroupKind)
			err = meta.CreateEntry(context.TODO(), namespace, parent.ID, &child2)
			Expect(err).Should(BeNil())

			nested1 := types.NewEntry("nested1", types.RawKind)
			err = meta.CreateEntry(context.TODO(), namespace, child2.ID, &nested1)
			Expect(err).Should(BeNil())
			meta.IndexDocument(context.TODO(), namespace, &types.IndexDocument{
				ID:      nested1.ID,
				URI:     "/root/parent/child2/nested1",
				Title:   "Nested One",
				Content: "Content of nested1",
			}, NewSpaceTokenizer().Tokenize)

			err = idx.UpdateChildrenURI(context.TODO(), namespace, parent.ID, "/new/parent")
			Expect(err).Should(BeNil())

			results, err := idx.QueryLanguage(context.TODO(), namespace, "nested1")
			Expect(err).Should(BeNil())
			Expect(len(results)).Should(Equal(1))
			Expect(results[0].URI).Should(Equal("/new/parent/child2/nested1"))
		})

		It("should do nothing for empty group", func() {
			meta, idx, err := newTestMetaDB("test_update_empty.db")
			Expect(err).Should(BeNil())

			rootEn := initRootEntry()
			err = meta.CreateEntry(context.TODO(), namespace, 0, rootEn)
			Expect(err).Should(BeNil())

			emptyGroup := types.NewEntry("empty", types.GroupKind)
			err = meta.CreateEntry(context.TODO(), namespace, rootEn.ID, &emptyGroup)
			Expect(err).Should(BeNil())

			err = idx.UpdateChildrenURI(context.TODO(), namespace, emptyGroup.ID, "/new/empty")
			Expect(err).Should(BeNil())
		})
	})
})

func setupTestEntries(meta metastore.Meta, ns string) {
	rootEn := initRootEntry()
	err := meta.CreateEntry(context.TODO(), ns, 0, rootEn)
	if err != nil {
		return
	}

	parent := types.NewEntry("parent", types.GroupKind)
	err = meta.CreateEntry(context.TODO(), ns, rootEn.ID, &parent)
	if err != nil {
		return
	}

	child1 := types.NewEntry("child1", types.RawKind)
	err = meta.CreateEntry(context.TODO(), ns, parent.ID, &child1)
	if err != nil {
		return
	}
	meta.IndexDocument(context.TODO(), ns, &types.IndexDocument{
		ID:      child1.ID,
		URI:     "/root/parent/child1",
		Title:   "Child One",
		Content: "Content of child1",
	}, NewSpaceTokenizer().Tokenize)

	child2 := types.NewEntry("child2", types.GroupKind)
	err = meta.CreateEntry(context.TODO(), ns, parent.ID, &child2)
	if err != nil {
		return
	}

	nested1 := types.NewEntry("nested1", types.RawKind)
	err = meta.CreateEntry(context.TODO(), ns, child2.ID, &nested1)
	if err != nil {
		return
	}
	meta.IndexDocument(context.TODO(), ns, &types.IndexDocument{
		ID:      nested1.ID,
		URI:     "/root/parent/child2/nested1",
		Title:   "Nested One",
		Content: "Content of nested1",
	}, NewSpaceTokenizer().Tokenize)
}

func setupTestEntriesForDelete(meta metastore.Meta, ns string) int64 {
	rootEn := initRootEntry()
	_ = meta.CreateEntry(context.TODO(), ns, 0, rootEn)

	parent := types.NewEntry("parent", types.GroupKind)
	_ = meta.CreateEntry(context.TODO(), ns, rootEn.ID, &parent)

	child1 := types.NewEntry("doc1", types.RawKind)
	_ = meta.CreateEntry(context.TODO(), ns, parent.ID, &child1)
	meta.IndexDocument(context.TODO(), ns, &types.IndexDocument{
		ID:      child1.ID,
		URI:     "/root/parent/doc1",
		Title:   "Doc One",
		Content: "Content of doc1",
	}, NewSpaceTokenizer().Tokenize)

	child2 := types.NewEntry("child2", types.GroupKind)
	_ = meta.CreateEntry(context.TODO(), ns, parent.ID, &child2)

	nested1 := types.NewEntry("doc2", types.RawKind)
	_ = meta.CreateEntry(context.TODO(), ns, child2.ID, &nested1)
	meta.IndexDocument(context.TODO(), ns, &types.IndexDocument{
		ID:      nested1.ID,
		URI:     "/root/parent/child2/doc2",
		Title:   "Doc Two",
		Content: "Content of doc2",
	}, NewSpaceTokenizer().Tokenize)

	return parent.ID
}

func initRootEntry() *types.Entry {
	acc := &types.Access{
		Permissions: []types.Permission{
			types.PermOwnerRead,
			types.PermOwnerWrite,
			types.PermOwnerExec,
			types.PermGroupRead,
			types.PermGroupWrite,
			types.PermOthersRead,
		},
	}
	root, _ := types.InitNewEntry(nil, types.EntryAttr{Name: "root", Kind: types.GroupKind, Access: acc})
	root.ID = 1
	root.Namespace = types.DefaultNamespace
	return root
}
