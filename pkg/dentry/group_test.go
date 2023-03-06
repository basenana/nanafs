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
				Dev:    0,
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
				Dev:    0,
				Kind:   types.GroupKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			Expect(file1).ShouldNot(BeNil())
		})
		It("create file2 should be succeed", func() {
			_, err := entryManager.CreateEntry(context.TODO(), group1, EntryAttr{
				Name:   "test_group_manage_file2",
				Dev:    0,
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

			file2.Object().Name = "test_group_manage_file3"
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

			Expect(group1.Group().DestroyEntry(context.TODO(), file1)).Should(BeNil())
			Expect(group1.Group().DestroyEntry(context.TODO(), file3)).Should(BeNil())

			file1, err = group1.Group().FindEntry(context.TODO(), "test_group_manage_file1")
			Expect(err).Should(Equal(types.ErrNotFound))
			file3, err = group1.Group().FindEntry(context.TODO(), "test_group_manage_file3")
			Expect(err).Should(Equal(types.ErrNotFound))
		})
	})
})
