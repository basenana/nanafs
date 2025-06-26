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
	"path"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/types"
)

var _ = Describe("TestSqliteObjectOperation", func() {
	var sqlite = buildNewSqliteMetaStore("test_object.db")
	// init root
	rootEn := InitRootEntry()
	Expect(sqlite.CreateEntry(context.TODO(), namespace, 0, rootEn)).Should(BeNil())

	Context("create a new file entry", func() {
		It("should be succeed", func() {
			en, err := types.InitNewEntry(rootEn, types.EntryAttr{
				Name: "test-new-en-1",
				Kind: types.RawKind,
			})
			Expect(err).Should(BeNil())

			err = sqlite.CreateEntry(context.TODO(), namespace, rootEn.ID, en)
			Expect(err).Should(BeNil())

			fetchObj, err := sqlite.GetEntry(context.TODO(), namespace, en.ID)
			Expect(err).Should(BeNil())
			Expect(fetchObj.Name).Should(Equal("test-new-en-1"))
		})
	})

	Context("update a exist file entry", func() {
		It("should be succeed", func() {
			en, err := types.InitNewEntry(rootEn, types.EntryAttr{
				Name: "test-update-en-1",
				Kind: types.RawKind,
			})
			Expect(err).Should(BeNil())

			err = sqlite.CreateEntry(context.TODO(), namespace, rootEn.ID, en)
			Expect(err).Should(BeNil())

			en.Name = "test-update-en-2"
			err = sqlite.UpdateEntry(context.TODO(), namespace, en)
			Expect(err).Should(BeNil())

			newObj, err := sqlite.GetEntry(context.TODO(), namespace, en.ID)
			Expect(err).Should(BeNil())
			Expect(newObj.Name).Should(Equal("test-update-en-2"))
		})
	})

	Context("delete a exist file entry", func() {
		It("should be succeed", func() {
			en, err := types.InitNewEntry(rootEn, types.EntryAttr{
				Name: "test-delete-en-1",
				Kind: types.RawKind,
			})
			Expect(err).Should(BeNil())

			err = sqlite.CreateEntry(context.TODO(), namespace, rootEn.ID, en, nil)
			Expect(err).Should(BeNil())

			_, err = sqlite.FindEntry(context.TODO(), namespace, rootEn.ID, en.Name)
			Expect(err).Should(BeNil())

			Expect(sqlite.RemoveEntry(context.TODO(), namespace, rootEn.ID, en.ID, "test-delete-en-1", types.DeleteEntry{})).Should(BeNil())

			_, err = sqlite.FindEntry(context.TODO(), namespace, rootEn.ID, en.Name)
			Expect(err).Should(Equal(types.ErrNotFound))
		})
	})

	Context("create a new group entry", func() {
		It("should be succeed", func() {
			en, err := types.InitNewEntry(rootEn, types.EntryAttr{
				Name: "test-new-group-1",
				Kind: types.GroupKind,
			})
			Expect(err).Should(BeNil())

			err = sqlite.CreateEntry(context.TODO(), namespace, rootEn.ID, en, nil)
			Expect(err).Should(BeNil())

			fetchEn, err := sqlite.GetEntry(context.TODO(), namespace, en.ID)
			Expect(err).Should(BeNil())
			Expect(fetchEn.Name).Should(Equal("test-new-group-1"))
			Expect(string(fetchEn.Kind)).Should(Equal(string(types.GroupKind)))
		})
	})
})

var _ = Describe("TestSqliteGroupOperation", func() {
	var sqlite = buildNewSqliteMetaStore("test_group.db")
	// init root
	rootEn := InitRootEntry()
	Expect(sqlite.CreateEntry(context.TODO(), namespace, 0, rootEn)).Should(BeNil())

	group1, err := types.InitNewEntry(rootEn, types.EntryAttr{
		Name: "test-new-group-1",
		Kind: types.GroupKind,
	})
	Expect(err).Should(BeNil())
	Expect(sqlite.CreateEntry(context.TODO(), namespace, rootEn.ID, group1)).Should(BeNil())

	group2, err := types.InitNewEntry(rootEn, types.EntryAttr{
		Name: "test-new-group-2",
		Kind: types.GroupKind,
	})
	Expect(err).Should(BeNil())
	Expect(sqlite.CreateEntry(context.TODO(), namespace, rootEn.ID, group2)).Should(BeNil())

	Context("list a group entry all children", func() {
		It("create group file should be succeed", func() {
			for i := 0; i < 4; i++ {
				en, err := types.InitNewEntry(group1, types.EntryAttr{Name: "test-file-en-1", Kind: types.RawKind})
				Expect(err).Should(BeNil())
				Expect(sqlite.CreateEntry(context.TODO(), namespace, group1.ID, en)).Should(BeNil())
			}

			en, err := types.InitNewEntry(group1, types.EntryAttr{Name: "test-dev-en-1", Kind: types.BlkDevKind})
			Expect(err).Should(BeNil())
			Expect(sqlite.CreateEntry(context.TODO(), namespace, group1.ID, en)).Should(BeNil())
		})

		It("list new file entry should be succeed", func() {
			chList, err := sqlite.ListChildren(context.TODO(), namespace, group1.ID)
			Expect(err).Should(BeNil())

			Expect(len(chList)).Should(Equal(5))
		})
	})

	Context("change a exist file entry parent group", func() {
		var targetEn *types.Entry
		It("create new file be succeed", func() {
			targetEn, err = types.InitNewEntry(group1, types.EntryAttr{Name: "test-mv-src-raw-obj-1", Kind: types.RawKind})
			Expect(err).Should(BeNil())
			Expect(sqlite.CreateEntry(context.TODO(), namespace, group1.ID, targetEn)).Should(BeNil())
		})

		It("should be succeed", func() {
			err = sqlite.ChangeEntryParent(context.TODO(), namespace, targetEn.ID, group1.ID, group2.ID, targetEn.Name, targetEn.Name, types.ChangeParentAttr{})
			Expect(err).Should(BeNil())

			chList, err := sqlite.ListChildren(context.TODO(), namespace, group2.ID)
			Expect(err).Should(BeNil())

			Expect(len(chList)).Should(Equal(1))
		})

		It("query target file entry in old group should not found", func() {
			chList, err := sqlite.ListChildren(context.TODO(), namespace, group1.ID)
			Expect(err).Should(BeNil())

			exist := false
			for _, ch := range chList {
				if ch.Name == targetEn.Name {
					exist = true
				}
			}
			Expect(exist).Should(Equal(false))
		})
		It("query target file entry in new group should be found", func() {
			chList, err := sqlite.ListChildren(context.TODO(), namespace, group2.ID)
			Expect(err).Should(BeNil())

			exist := false
			for _, ch := range chList {
				if ch.Name == targetEn.Name {
					exist = true
				}
			}
			Expect(exist).Should(Equal(true))
		})
	})

	Context("mirror a exist file entry to other group", func() {
		group3, err := types.InitNewEntry(rootEn, types.EntryAttr{
			Name: "test-mirror-group-1",
			Kind: types.GroupKind,
		})
		Expect(err).Should(BeNil())
		Expect(sqlite.CreateEntry(context.TODO(), namespace, rootEn.ID, group3)).Should(BeNil())

		group4, err := types.InitNewEntry(rootEn, types.EntryAttr{
			Name: "test-mirror-group-2",
			Kind: types.GroupKind,
		})
		Expect(err).Should(BeNil())
		Expect(sqlite.CreateEntry(context.TODO(), namespace, rootEn.ID, group4)).Should(BeNil())

		var srcEN *types.Entry
		It("create new file be succeed", func() {
			srcEN, err = types.InitNewEntry(group1, types.EntryAttr{Name: "test-src-raw-obj-1", Kind: types.RawKind})
			Expect(err).Should(BeNil())
			Expect(sqlite.CreateEntry(context.TODO(), namespace, group1.ID, srcEN)).Should(BeNil())
		})

		It("create mirror entry should be succeed", func() {
			err := sqlite.MirrorEntry(context.TODO(), namespace, srcEN.ID, "test-mirror-dst-file-2", rootEn.ID)
			Expect(err).Should(BeNil())
		})

		It("filter mirror entry should be succeed", func() {
			chList, err := sqlite.ListChildren(context.TODO(), namespace, rootEn.ID)
			Expect(err).Should(BeNil())

			found := false
			for _, ch := range chList {
				if ch.Name == "test-mirror-dst-file-2" {
					found = true
				}
			}
			Expect(found).Should(BeTrue())
		})
	})
})

func InitRootEntry() *types.Entry {
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

func buildNewSqliteMetaStore(dbName string) *sqlMetaStore {
	result, err := newSqliteMetaStore(config.Meta{
		Type: SqliteMeta,
		Path: path.Join(workdir, dbName),
	})
	Expect(err).Should(BeNil())
	return result
}
