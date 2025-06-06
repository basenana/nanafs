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

package core

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
)

var _ = Describe("TestRootEntryInit", func() {
	emptyObjectStore, err := metastore.NewMetaStorage(metastore.MemoryMeta, config.Meta{})
	Expect(err).Should(BeNil())

	Context("query root entry first time", func() {
		var mgr Core
		It("init core should be succeed", func() {
			mgr, err = New(emptyObjectStore, bootCfg)
			Expect(err).Should(BeNil())
		})
		It("feat root should be succeed", func() {
			_, err := mgr.FSRoot(context.TODO())
			Expect(err).Should(BeNil())
		})
	})

	Context("query root entry again", func() {
		It("sleep 1s to wait", func() {
			time.Sleep(time.Second)
		})

		var mgr Core
		It("init core should be succeed", func() {
			mgr, err = New(emptyObjectStore, bootCfg)
			Expect(err).Should(BeNil())
		})
		It("fetch root should be succeed", func() {
			r, err := mgr.FSRoot(context.TODO())
			Expect(err).Should(BeNil())
			Expect(time.Since(r.CreatedAt) > time.Second).Should(BeTrue())

		})
	})
})

var _ = Describe("TestEntryManage", func() {
	var grp1ID int64
	ctx := context.TODO()
	Context("create group entry", func() {
		It("create should be succeed", func() {
			grp1, err := fsCore.CreateEntry(ctx, namespace, root.ID, types.EntryAttr{
				Name:   "test_create_grp1",
				Kind:   types.GroupKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			grp1ID = grp1.ID
		})
		It("query should be succeed", func() {
			grp, err := fsCore.OpenGroup(ctx, namespace, root.ID)
			Expect(err).Should(BeNil())
			_, err = groupHasChildEntry(grp, "test_create_grp1")
			Expect(err).Should(BeNil())
		})
	})
	Context("create file entry", func() {
		It("create should be succeed", func() {
			_, err := fsCore.CreateEntry(ctx, namespace, grp1ID, types.EntryAttr{
				Name:   "test_create_file1",
				Kind:   types.GroupKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())

			grp, err := fsCore.OpenGroup(ctx, namespace, grp1ID)
			Expect(err).Should(BeNil())
			_, err = groupHasChildEntry(grp, "test_create_file1")
			Expect(err).Should(BeNil())
		})
	})
	Context("delete file entry", func() {
		It("delete should be succeed", func() {
			grp, err := fsCore.OpenGroup(ctx, namespace, grp1ID)
			Expect(err).Should(BeNil())
			file1, err := groupHasChildEntry(grp, "test_create_file1")
			Expect(err).Should(BeNil())
			err = fsCore.RemoveEntry(ctx, namespace, grp1ID, file1.ID, "test_create_file1", types.DeleteEntry{})
			Expect(err).Should(BeNil())

			grp, err = fsCore.OpenGroup(ctx, namespace, grp1ID)
			Expect(err).Should(BeNil())
			_, err = groupHasChildEntry(grp, "test_create_file1")
			Expect(err).Should(Equal(types.ErrNotFound))
		})
	})
	Context("delete group entry", func() {
		It("delete should be succeed", func() {
			grp, err := fsCore.OpenGroup(ctx, namespace, root.ID)
			Expect(err).Should(BeNil())
			_, err = groupHasChildEntry(grp, "test_create_grp1")
			Expect(err).Should(BeNil())
			err = fsCore.RemoveEntry(ctx, namespace, root.ID, grp1ID, "test_create_grp1", types.DeleteEntry{})
			Expect(err).Should(BeNil())
		})
		It("query should be failed", func() {
			grp, err := fsCore.OpenGroup(ctx, namespace, root.ID)
			Expect(err).Should(BeNil())
			_, err = groupHasChildEntry(grp, "test_create_grp1")
			Expect(err).Should(Equal(types.ErrNotFound))
		})
	})
})

var _ = Describe("TestMirrorEntryManage", func() {
	var (
		ctx                  = context.TODO()
		grp1, sFile1, mFile2 *types.Entry
		sourceFile           = "test_mirror_grp1_file1"
		mirrorFile2          = "test_mirror_grp1_file2"
		mirrorFile3          = "test_mirror_grp1_file3"
	)
	Context("create group mirror entry mirror_grp1", func() {
		It("create should be succeed", func() {
			var err error
			grp1, err = fsCore.CreateEntry(ctx, namespace, root.ID, types.EntryAttr{
				Name:   "test_mirror_grp1",
				Kind:   types.GroupKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())

			grp, err := fsCore.OpenGroup(ctx, namespace, root.ID)
			Expect(err).Should(BeNil())
			_, err = groupHasChildEntry(grp, "test_mirror_grp1")
			Expect(err).Should(BeNil())
		})
		It("should be file", func() {
			_, err := fsCore.MirrorEntry(ctx, namespace, grp1.ID, root.ID, types.EntryAttr{
				Name:   "mirror_grp1",
				Kind:   types.GroupKind,
				Access: accessPermissions,
			})
			Expect(err).Should(Equal(types.ErrIsGroup))
		})
	})
	Context("create file entry in test_mirror_grp1", func() {
		It("create should be succeed", func() {
			var err error
			sFile1, err = fsCore.CreateEntry(ctx, namespace, grp1.ID, types.EntryAttr{
				Name:   sourceFile,
				Kind:   types.RawKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())

			child, err := fsCore.ListChildren(ctx, namespace, grp1.ID)
			Expect(err).Should(BeNil())

			_, err = groupHasChild(child, sourceFile)
			Expect(err).Should(BeNil())
		})
		It("mirror file should be succeed", func() {
			var err error
			mFile2, err = fsCore.MirrorEntry(ctx, namespace, sFile1.ID, grp1.ID, types.EntryAttr{
				Name:   mirrorFile2,
				Kind:   types.RawKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			Expect(mustGetEntry(mFile2).RefCount).Should(Equal(2))

			child, err := fsCore.ListChildren(ctx, namespace, grp1.ID)
			Expect(err).Should(BeNil())
			_, err = groupHasChild(child, mirrorFile2)

			Expect(mustGetEntry(sFile1).RefCount).Should(Equal(2))
		})
		It("mirror mirrored file should be succeed", func() {
			child, err := fsCore.ListChildren(ctx, namespace, grp1.ID)
			Expect(err).Should(BeNil())
			_, err = groupHasChild(child, mirrorFile2)
			Expect(err).Should(BeNil())
			_, err = fsCore.MirrorEntry(ctx, namespace, mFile2.ID, grp1.ID, types.EntryAttr{
				Name:   mirrorFile3,
				Kind:   types.RawKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())

			child, err = fsCore.ListChildren(ctx, namespace, grp1.ID)
			Expect(err).Should(BeNil())

			mFile3ID, err := groupHasChild(child, mirrorFile3)
			Expect(err).Should(BeNil())
			Expect(mFile3ID).Should(Equal(mFile2.ID))

			_, err = groupHasChild(child, sourceFile)
			Expect(err).Should(BeNil())

			Expect(mustGetEntry(mFile2).RefCount).Should(Equal(3))
		})
		It("delete file should be succeed", func() {
			child, err := fsCore.ListChildren(ctx, namespace, grp1.ID)
			Expect(err).Should(BeNil())
			_, err = groupHasChild(child, sourceFile)

			err = fsCore.RemoveEntry(ctx, namespace, grp1.ID, sFile1.ID, sourceFile, types.DeleteEntry{})
			Expect(err).Should(BeNil())

			child, err = fsCore.ListChildren(ctx, namespace, grp1.ID)
			Expect(err).Should(BeNil())

			_, err = groupHasChild(child, sourceFile)
			Expect(err).Should(Equal(types.ErrNotFound))

			_, err = groupHasChild(child, mirrorFile2)
			Expect(err).Should(BeNil())

			Expect(mustGetEntry(mFile2).RefCount).Should(Equal(2))
		})
		It("delete mirror file should be succeed", func() {
			chList, err := fsCore.ListChildren(ctx, namespace, grp1.ID)
			Expect(err).Should(BeNil())
			_, err = groupHasChild(chList, mirrorFile2)
			Expect(err).Should(BeNil())

			err = fsCore.RemoveEntry(ctx, namespace, grp1.ID, mFile2.ID, mirrorFile2, types.DeleteEntry{})
			Expect(err).Should(BeNil())

			chList, err = fsCore.ListChildren(ctx, namespace, grp1.ID)
			Expect(err).Should(BeNil())
			_, err = groupHasChild(chList, mirrorFile2)
			Expect(err).Should(Equal(types.ErrNotFound))
			Expect(mustGetEntry(sFile1).RefCount).Should(Equal(1))
		})
		It("delete mirror file should be succeed", func() {
			chList, err := fsCore.ListChildren(ctx, namespace, grp1.ID)
			Expect(err).Should(BeNil())
			_, err = groupHasChild(chList, mirrorFile3)
			Expect(err).Should(BeNil())

			err = fsCore.RemoveEntry(ctx, namespace, grp1.ID, sFile1.ID, mirrorFile3, types.DeleteEntry{})
			Expect(err).Should(BeNil())

			chList, err = fsCore.ListChildren(ctx, namespace, grp1.ID)
			Expect(err).Should(BeNil())

			_, err = groupHasChild(chList, mirrorFile3)
			Expect(err).Should(Equal(types.ErrNotFound))
		})
	})
})

var _ = Describe("TestChangeEntryParent", func() {
	var (
		ctx       = context.TODO()
		grp1      *types.Entry
		grp1File1 *types.Entry
		grp1File2 *types.Entry
		grp2File2 *types.Entry
		grp2      *types.Entry
	)

	Context("create grp and files", func() {
		It("create grp1 and files should be succeed", func() {
			var err error
			grp1, err = fsCore.CreateEntry(ctx, namespace, root.ID, types.EntryAttr{
				Name:   "test_mv_grp1",
				Kind:   types.GroupKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			grp1File1, err = fsCore.CreateEntry(ctx, namespace, grp1.ID, types.EntryAttr{
				Name:   "test_mv_grp1_file1",
				Kind:   types.RawKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			grp1File2, err = fsCore.CreateEntry(ctx, namespace, grp1.ID, types.EntryAttr{
				Name:   "test_mv_grp1_file2",
				Kind:   types.RawKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
		})
		It("create grp2 and files should be succeed", func() {
			var err error
			grp2, err = fsCore.CreateEntry(ctx, namespace, root.ID, types.EntryAttr{
				Name:   "test_mv_grp2",
				Kind:   types.GroupKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			grp2File2, err = fsCore.CreateEntry(ctx, namespace, grp2.ID, types.EntryAttr{
				Name:   "test_mv_grp2_file2",
				Kind:   types.RawKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
		})
	})
	Context("mv file src grp1 and files", func() {
		It("should be succeed", func() {
			err := fsCore.ChangeEntryParent(ctx, namespace, grp1File1.ID, nil, grp1.ID, grp2.ID, grp1File1.Name, "test_mv_grp2_file1", types.ChangeParentAttr{})
			Expect(err).Should(BeNil())
		})
		It("has existed file should be failed", func() {
			err := fsCore.ChangeEntryParent(ctx, namespace, grp1File2.ID, &grp2File2.ID, grp1.ID, grp2.ID, grp1File2.Name, "test_mv_grp2_file2", types.ChangeParentAttr{})
			Expect(err).Should(Equal(types.ErrIsExist))
		})
		It("has existed file should be succeed if enable replace", func() {
			err := fsCore.ChangeEntryParent(ctx, namespace, grp1File2.ID, &grp2File2.ID, grp1.ID, grp2.ID, grp1File2.Name, "test_mv_grp2_file2", types.ChangeParentAttr{Replace: true})
			Expect(err).Should(BeNil())
		})
	})
})

func mustGetEntry(entry *types.Entry) *types.Entry {
	var (
		ctx = context.TODO()
		err error
	)
	entry, err = fsCore.GetEntry(ctx, namespace, entry.ID)
	if err != nil {
		panic(err)
	}
	return entry
}

var accessPermissions = &types.Access{
	Permissions: []types.Permission{
		types.PermOwnerRead,
		types.PermOwnerWrite,
		types.PermOwnerExec,
		types.PermGroupRead,
		types.PermGroupWrite,
		types.PermOthersRead,
	},
}

func groupHasChildEntry(grp Group, entryName string) (*types.Entry, error) {
	children, err := grp.ListChildren(context.TODO())
	if err != nil {
		return nil, err
	}
	for _, child := range children {
		if child.Name == entryName {
			return child, nil
		}
	}
	return nil, types.ErrNotFound
}

func groupHasChild(childList []*types.Child, entryName string) (int64, error) {
	for _, child := range childList {
		if child.Name == entryName {
			return child.ChildID, nil
		}
	}
	return 0, types.ErrNotFound
}
