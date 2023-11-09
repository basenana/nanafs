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

package dentry

import (
	"context"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("TestRootEntryInit", func() {
	emptyObjectStore, err := metastore.NewMetaStorage(metastore.MemoryMeta, config.Meta{})
	Expect(err).Should(BeNil())

	Context("query root entry first time", func() {
		var mgr Manager
		It("init manager should be succeed", func() {
			mgr, _ = NewManager(emptyObjectStore, config.Config{FS: &config.FS{}, Storages: []config.Storage{{
				ID:   storage.MemoryStorage,
				Type: storage.MemoryStorage,
			}}})
		})
		It("feat root should be succeed", func() {
			_, err := mgr.Root(context.TODO())
			Expect(err).Should(BeNil())
		})
	})

	Context("query root entry again", func() {
		It("sleep 1s to wait", func() {
			time.Sleep(time.Second)
		})

		var mgr Manager
		It("init manager should be succeed", func() {
			mgr, _ = NewManager(emptyObjectStore, config.Config{FS: &config.FS{}, Storages: []config.Storage{{
				ID:   storage.MemoryStorage,
				Type: storage.MemoryStorage,
			}}})
		})
		It("feat root should be succeed", func() {
			r, err := mgr.Root(context.TODO())
			Expect(err).Should(BeNil())
			Expect(time.Since(r.CreatedAt) > time.Second).Should(BeTrue())

		})
	})
})

var _ = Describe("TestEntryManage", func() {
	var grp1ID int64
	Context("create group entry", func() {
		It("create should be succeed", func() {
			grp1, err := entryManager.CreateEntry(context.TODO(), root.ID, types.EntryAttr{
				Name:   "test_create_grp1",
				Kind:   types.GroupKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			grp1ID = grp1.ID
		})
		It("query should be succeed", func() {
			grp, err := entryManager.OpenGroup(context.TODO(), root.ID)
			Expect(err).Should(BeNil())
			_, err = grp.FindEntry(context.TODO(), "test_create_grp1")
			Expect(err).Should(BeNil())
		})
	})
	Context("create file entry", func() {
		It("create should be succeed", func() {
			_, err := entryManager.CreateEntry(context.TODO(), grp1ID, types.EntryAttr{
				Name:   "test_create_file1",
				Kind:   types.GroupKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())

			grp, err := entryManager.OpenGroup(context.TODO(), grp1ID)
			Expect(err).Should(BeNil())
			_, err = grp.FindEntry(context.TODO(), "test_create_file1")
			Expect(err).Should(BeNil())
		})
	})
	Context("delete file entry", func() {
		It("delete should be succeed", func() {
			grp, err := entryManager.OpenGroup(context.TODO(), grp1ID)
			Expect(err).Should(BeNil())
			file1, err := grp.FindEntry(context.TODO(), "test_create_file1")
			Expect(err).Should(BeNil())
			err = entryManager.RemoveEntry(context.TODO(), grp1ID, file1.ID)
			Expect(err).Should(BeNil())

			grp, err = entryManager.OpenGroup(context.TODO(), grp1ID)
			Expect(err).Should(BeNil())
			_, err = grp.FindEntry(context.TODO(), "test_create_file1")
			Expect(err).Should(Equal(types.ErrNotFound))
		})
	})
	Context("delete group entry", func() {
		It("delete should be succeed", func() {
			grp, err := entryManager.OpenGroup(context.TODO(), root.ID)
			Expect(err).Should(BeNil())
			_, err = grp.FindEntry(context.TODO(), "test_create_grp1")
			Expect(err).Should(BeNil())
			err = entryManager.RemoveEntry(context.TODO(), root.ID, grp1ID)
			Expect(err).Should(BeNil())
		})
		It("query should be failed", func() {
			grp, err := entryManager.OpenGroup(context.TODO(), root.ID)
			Expect(err).Should(BeNil())
			_, err = grp.FindEntry(context.TODO(), "test_create_grp1")
			Expect(err).Should(Equal(types.ErrNotFound))
		})
	})
})

var _ = Describe("TestMirrorEntryManage", func() {
	var (
		grp1        *types.Metadata
		sourceFile  = "test_mirror_grp1_file1"
		mirrorFile2 = "test_mirror_grp1_file2"
		mirrorFile3 = "test_mirror_grp1_file3"
	)
	Context("create group mirror entry mirror_grp1", func() {
		It("create should be succeed", func() {
			var err error
			grp1, err = entryManager.CreateEntry(context.TODO(), root.ID, types.EntryAttr{
				Name:   "test_mirror_grp1",
				Kind:   types.GroupKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())

			grp, err := entryManager.OpenGroup(context.TODO(), root.ID)
			Expect(err).Should(BeNil())
			_, err = grp.FindEntry(context.TODO(), "test_mirror_grp1")
			Expect(err).Should(BeNil())
		})
		It("should be file", func() {
			_, err := entryManager.MirrorEntry(context.TODO(), grp1.ID, root.ID, types.EntryAttr{
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
			_, err = entryManager.CreateEntry(context.TODO(), grp1.ID, types.EntryAttr{
				Name:   sourceFile,
				Kind:   types.RawKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())

		})
		It("mirror file should be succeed", func() {
			grp, err := entryManager.OpenGroup(context.TODO(), grp1.ID)
			Expect(err).Should(BeNil())
			sFile, err := grp.FindEntry(context.TODO(), sourceFile)
			Expect(err).Should(BeNil())

			mFile2, err := entryManager.MirrorEntry(context.TODO(), mustGetSourceEntry(sFile).ID, grp1.ID, types.EntryAttr{
				Name:   mirrorFile2,
				Kind:   types.RawKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			Expect(mustGetSourceEntry(mFile2).RefCount).Should(Equal(2))

			grp, err = entryManager.OpenGroup(context.TODO(), grp1.ID)
			Expect(err).Should(BeNil())
			sFile, err = grp.FindEntry(context.TODO(), sourceFile)
			Expect(err).Should(BeNil())
			Expect(mustGetSourceEntry(sFile).RefCount).Should(Equal(2))
		})
		It("mirror mirrored file should be succeed", func() {
			grp, err := entryManager.OpenGroup(context.TODO(), grp1.ID)
			Expect(err).Should(BeNil())
			mFile2, err := grp.FindEntry(context.TODO(), mirrorFile2)
			Expect(err).Should(BeNil())
			_, err = entryManager.MirrorEntry(context.TODO(), mustGetSourceEntry(mFile2).ID, grp1.ID, types.EntryAttr{
				Name:   mirrorFile3,
				Kind:   types.RawKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())

			grp, err = entryManager.OpenGroup(context.TODO(), grp1.ID)
			Expect(err).Should(BeNil())
			sFile, err := grp.FindEntry(context.TODO(), sourceFile)
			Expect(err).Should(BeNil())
			Expect(mustGetSourceEntry(sFile).RefCount).Should(Equal(3))
		})
		It("delete file should be succeed", func() {
			grp, err := entryManager.OpenGroup(context.TODO(), grp1.ID)
			Expect(err).Should(BeNil())
			sFile, err := grp.FindEntry(context.TODO(), sourceFile)
			Expect(err).Should(BeNil())

			err = entryManager.RemoveEntry(context.TODO(), grp1.ID, sFile.ID)
			Expect(err).Should(BeNil())

			grp, err = entryManager.OpenGroup(context.TODO(), grp1.ID)
			Expect(err).Should(BeNil())
			_, err = grp.FindEntry(context.TODO(), sourceFile)
			Expect(err).Should(Equal(types.ErrNotFound))

			grp, err = entryManager.OpenGroup(context.TODO(), grp1.ID)
			Expect(err).Should(BeNil())
			f2, err := grp.FindEntry(context.TODO(), mirrorFile2)
			Expect(err).Should(BeNil())
			Expect(mustGetSourceEntry(f2).RefCount).Should(Equal(2))
		})
		It("delete mirror file should be succeed", func() {
			grp, err := entryManager.OpenGroup(context.TODO(), grp1.ID)
			Expect(err).Should(BeNil())
			mFile2, err := grp.FindEntry(context.TODO(), mirrorFile2)
			Expect(err).Should(BeNil())
			err = entryManager.RemoveEntry(context.TODO(), grp1.ID, mFile2.ID)
			Expect(err).Should(BeNil())

			grp, err = entryManager.OpenGroup(context.TODO(), grp1.ID)
			Expect(err).Should(BeNil())
			_, err = grp.FindEntry(context.TODO(), mirrorFile2)
			Expect(err).Should(Equal(types.ErrNotFound))
			mFile3, err := grp.FindEntry(context.TODO(), mirrorFile3)
			Expect(err).Should(BeNil())
			Expect(mustGetSourceEntry(mFile3).RefCount).Should(Equal(1))
		})
		It("delete mirror file should be succeed", func() {
			grp, err := entryManager.OpenGroup(context.TODO(), grp1.ID)
			Expect(err).Should(BeNil())
			mirroredFile, err := grp.FindEntry(context.TODO(), mirrorFile3)
			Expect(err).Should(BeNil())

			err = entryManager.RemoveEntry(context.TODO(), grp1.ID, mirroredFile.ID)
			Expect(err).Should(BeNil())

			grp, err = entryManager.OpenGroup(context.TODO(), grp1.ID)
			Expect(err).Should(BeNil())
			_, err = grp.FindEntry(context.TODO(), mirrorFile3)
			Expect(err).Should(Equal(types.ErrNotFound))
		})
	})
})

var _ = Describe("TestChangeEntryParent", func() {
	var (
		grp1      *types.Metadata
		grp1File1 *types.Metadata
		grp1File2 *types.Metadata
		grp2File2 *types.Metadata
		grp2      *types.Metadata
	)

	Context("create grp and files", func() {
		It("create grp1 and files should be succeed", func() {
			var err error
			grp1, err = entryManager.CreateEntry(context.TODO(), root.ID, types.EntryAttr{
				Name:   "test_mv_grp1",
				Kind:   types.GroupKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			grp1File1, err = entryManager.CreateEntry(context.TODO(), grp1.ID, types.EntryAttr{
				Name:   "test_mv_grp1_file1",
				Kind:   types.RawKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			grp1File2, err = entryManager.CreateEntry(context.TODO(), grp1.ID, types.EntryAttr{
				Name:   "test_mv_grp1_file2",
				Kind:   types.RawKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
		})
		It("create grp2 and files should be succeed", func() {
			var err error
			grp2, err = entryManager.CreateEntry(context.TODO(), root.ID, types.EntryAttr{
				Name:   "test_mv_grp2",
				Kind:   types.GroupKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			grp2File2, err = entryManager.CreateEntry(context.TODO(), grp2.ID, types.EntryAttr{
				Name:   "test_mv_grp2_file2",
				Kind:   types.RawKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
		})
	})
	Context("mv file src grp1 and files", func() {
		It("should be succeed", func() {
			err := entryManager.ChangeEntryParent(context.TODO(), grp1File1.ID, nil, grp1.ID, grp2.ID, "test_mv_grp2_file1", types.ChangeParentAttr{})
			Expect(err).Should(BeNil())
		})
		It("has existed file should be failed", func() {
			err := entryManager.ChangeEntryParent(context.TODO(), grp1File2.ID, &grp2File2.ID, grp1.ID, grp2.ID, "test_mv_grp2_file2", types.ChangeParentAttr{})
			Expect(err).Should(Equal(types.ErrIsExist))
		})
		It("has existed file should be succeed if enable replace", func() {
			err := entryManager.ChangeEntryParent(context.TODO(), grp1File2.ID, &grp2File2.ID, grp1.ID, grp2.ID, "test_mv_grp2_file2", types.ChangeParentAttr{Replace: true})
			Expect(err).Should(BeNil())
		})
	})
})

func mustGetSourceEntry(entry *types.Metadata) *types.Metadata {
	var err error
	for entry.RefID != 0 && entry.RefID != entry.ID {
		entry, err = entryManager.GetEntry(context.TODO(), entry.RefID)
		if err != nil {
			panic(err)
		}
	}
	return entry
}

var accessPermissions = types.Access{
	Permissions: []types.Permission{
		types.PermOwnerRead,
		types.PermOwnerWrite,
		types.PermOwnerExec,
		types.PermGroupRead,
		types.PermGroupWrite,
		types.PermOthersRead,
	},
}
