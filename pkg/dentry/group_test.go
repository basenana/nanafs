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
	"github.com/basenana/nanafs/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TestManageGroupEntry", func() {
	var (
		group1 Entry
		file1  Entry
	)
	Context("init group1", func() {
		It("init group should be succeed", func() {
			var err error
			group1, err = entryManager.CreateEntry(context.TODO(), root, EntryAttr{
				Name:   "test_group_manage_group1",
				Kind:   types.GroupKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
		})
	})
	Context("create file entry in group1", func() {
		It("create file1 should be succeed", func() {
			var err error
			file1, err = entryManager.CreateEntry(context.TODO(), group1, EntryAttr{
				Name:   "test_group_manage_file1",
				Kind:   types.GroupKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			Expect(file1).ShouldNot(BeNil())
		})
		It("create file2 should be succeed", func() {
			_, err := entryManager.CreateEntry(context.TODO(), group1, EntryAttr{
				Name:   "test_group_manage_file2",
				Kind:   types.GroupKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
		})
		It("create file1 should be existed", func() {
			_, err := entryManager.CreateEntry(context.TODO(), group1, EntryAttr{
				Name:   "test_group_manage_file1",
				Kind:   types.GroupKind,
				Access: accessPermissions,
			})
			Expect(err).Should(Equal(types.ErrIsExist))
		})
		It("rename file2 to file3 should be succeed", func() {
			file2, err := group1.Group().FindEntry(context.TODO(), "test_group_manage_file2")
			Expect(err).Should(BeNil())

			file2.Metadata().Name = "test_group_manage_file3"
			err = group1.Group().UpdateEntry(context.TODO(), file2)
			Expect(err).Should(BeNil())

			_, err = group1.Group().FindEntry(context.TODO(), "test_group_manage_file2")
			Expect(err).Should(Equal(types.ErrNotFound))
			_, err = group1.Group().FindEntry(context.TODO(), "test_group_manage_file3")
			Expect(err).Should(BeNil())
		})
		It("list file1 & file3 should be succeed", func() {
			entries, err := group1.Group().ListChildren(context.TODO())
			Expect(err).Should(BeNil())
			fileNames := map[string]bool{}
			for _, en := range entries {
				fileNames[en.Metadata().Name] = true
			}
			Expect(len(fileNames)).Should(Equal(2))
			Expect(fileNames["test_group_manage_file1"]).Should(BeTrue())
			Expect(fileNames["test_group_manage_file3"]).Should(BeTrue())

		})
		It("delete file1 & file3 should be succeed", func() {
			file1, err := group1.Group().FindEntry(context.TODO(), "test_group_manage_file1")
			Expect(err).Should(BeNil())
			file3, err := group1.Group().FindEntry(context.TODO(), "test_group_manage_file3")
			Expect(err).Should(BeNil())

			Expect(group1.Group().RemoveEntry(context.TODO(), file1)).Should(BeNil())
			Expect(group1.Group().RemoveEntry(context.TODO(), file3)).Should(BeNil())

			file1, err = group1.Group().FindEntry(context.TODO(), "test_group_manage_file1")
			Expect(err).Should(Equal(types.ErrNotFound))
			file3, err = group1.Group().FindEntry(context.TODO(), "test_group_manage_file3")
			Expect(err).Should(Equal(types.ErrNotFound))
		})
	})
})

var _ = Describe("TestExtGroupEntry", func() {
	var (
		extGrp       Group
		extGrpEn     Entry
		file1, file2 Entry
		err          error
	)
	Context("init ext group", func() {
		It("init group should be succeed", func() {
			var err error
			extGrpEn, err = entryManager.CreateEntry(context.TODO(), root, EntryAttr{
				Name:   "test_ext_memfs",
				Kind:   types.ExternalGroupKind,
				Access: accessPermissions,
				PlugScope: &types.PlugScope{
					PluginName: "memfs",
					Version:    "1.0",
					PluginType: types.TypeMirror,
				},
			})
			Expect(err).Should(BeNil())
			Expect(extGrpEn).ShouldNot(BeNil())

			extGrp = extGrpEn.Group()
			Expect(extGrp).ShouldNot(BeNil())
		})
	})

	Context("create file", func() {

		It("create file1.yaml should be succeed", func() {
			file1, err = entryManager.CreateEntry(context.TODO(), extGrpEn, EntryAttr{
				Name:   "file1.yaml",
				Kind:   types.RawKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			Expect(file1).ShouldNot(BeNil())
		})
		It("write and read file1.yaml should be succeed", func() {
			var f File
			f, err = entryManager.Open(context.TODO(), file1, Attr{Read: true, Write: true})
			Expect(err).Should(BeNil())

			_, err = f.WriteAt(context.TODO(), []byte("file1: hello world!"), 0)
			Expect(err).Should(BeNil())
			err = f.Close(context.TODO())
			Expect(err).Should(BeNil())

			f, err = entryManager.Open(context.TODO(), file1, Attr{Read: true, Write: true})
			Expect(err).Should(BeNil())

			var (
				buf   = make([]byte, 32)
				readN int64
			)
			readN, err = f.ReadAt(context.TODO(), buf, 0)
			Expect(err).Should(BeNil())
			Expect(string(buf[:readN])).Should(Equal("file1: hello world!"))
		})
		It("create file2.yaml should be succeed", func() {
			file2, err = entryManager.CreateEntry(context.TODO(), extGrpEn, EntryAttr{
				Name:   "file2.yaml",
				Kind:   types.RawKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			Expect(file1).ShouldNot(BeNil())
		})
		It("write and read file2.yaml should be succeed", func() {
			var f File
			f, err = entryManager.Open(context.TODO(), file2, Attr{Read: true, Write: true})
			Expect(err).Should(BeNil())

			_, err = f.WriteAt(context.TODO(), []byte("file2: hello world!"), 0)
			Expect(err).Should(BeNil())
			err = f.Close(context.TODO())
			Expect(err).Should(BeNil())

			f, err = entryManager.Open(context.TODO(), file2, Attr{Read: true, Write: true})
			Expect(err).Should(BeNil())

			var (
				buf   = make([]byte, 32)
				readN int64
			)
			readN, err = f.ReadAt(context.TODO(), buf, 0)
			Expect(err).Should(BeNil())
			Expect(string(buf[:readN])).Should(Equal("file2: hello world!"))
		})
		It("create file1.yaml should already existed", func() {
			_, err = entryManager.CreateEntry(context.TODO(), extGrpEn, EntryAttr{
				Name:   "file1.yaml",
				Kind:   types.RawKind,
				Access: accessPermissions,
			})
			Expect(err).Should(Equal(types.ErrIsExist))
		})
	})

	Context("delete file", func() {
		It("list files should be succeed", func() {
			extGrp = extGrpEn.Group()
			Expect(extGrp).ShouldNot(BeNil())
			child, err := extGrp.ListChildren(context.TODO())
			Expect(err).Should(BeNil())
			Expect(len(child)).Should(Equal(2))

			need := map[string]struct{}{"file1.yaml": {}, "file2.yaml": {}}
			for _, ch := range child {
				delete(need, ch.Metadata().Name)
			}
			Expect(len(need)).Should(Equal(0))
		})
		It("delete file2.yaml should be succeed", func() {
			err = entryManager.RemoveEntry(context.TODO(), extGrpEn, file2)
			Expect(err).Should(BeNil())
		})
		It("list files and file2.yaml should gone", func() {
			extGrp = extGrpEn.Group()
			Expect(extGrp).ShouldNot(BeNil())

			child, err := extGrp.ListChildren(context.TODO())
			Expect(err).Should(BeNil())
			Expect(len(child)).Should(Equal(1))

			need := map[string]struct{}{"file1.yaml": {}}
			for _, ch := range child {
				delete(need, ch.Metadata().Name)
			}
			Expect(len(need)).Should(Equal(0))
		})
	})

	Context("sync file", func() {
		It("insert sync_file1.yaml to memfs should be succeed", func() {
		})
		It("list files should contain sync_file1.yaml", func() {
		})
		It("insert sync_file2.yaml to memfs should be succeed", func() {
		})
		It("read sync_file2.yaml should be succeed", func() {
		})
	})

	Context("mv file", func() {
		It("rename file1.yaml in same ext group should be succeed", func() {
		})
		It("create test_ext_group_file_1.yaml in root dir should be succeed", func() {
		})
		It("mv /test_ext_group_file_1.yaml to ext group should be succeed", func() {
		})
	})

	Context("ext group dir", func() {
		It("create dir1 should be succeed", func() {
		})
		It("create dir1/file1.yaml should be succeed", func() {
		})
		It("delete dir1 should be failed(dir not empty)", func() {
		})
		It("delete dir1/file1.yaml should be succeed", func() {
		})
		It("delete dir1 should be succeed", func() {
		})
	})

})
