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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/basenana/nanafs/pkg/types"
)

var _ = Describe("TestManageGroupEntry", func() {
	var (
		ctx    = context.TODO()
		group1 *types.Entry
		file1  *types.Entry
	)
	Context("init group1", func() {
		It("init group should be succeed", func() {
			var (
				err error
			)
			group1, err = fsCore.CreateEntry(ctx, namespace, "/", types.EntryAttr{
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
			file1, err = fsCore.CreateEntry(ctx, namespace, "/test_group_manage_group1", types.EntryAttr{
				Name:   "test_group_manage_file1",
				Kind:   types.GroupKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			Expect(file1).ShouldNot(BeNil())
		})
		It("create file2 should be succeed", func() {
			_, err := fsCore.CreateEntry(ctx, namespace, "/test_group_manage_group1", types.EntryAttr{
				Name:   "test_group_manage_file2",
				Kind:   types.GroupKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
		})
		It("create file1 should be existed", func() {
			_, err := fsCore.CreateEntry(ctx, namespace, "/test_group_manage_group1", types.EntryAttr{
				Name:   "test_group_manage_file1",
				Kind:   types.GroupKind,
				Access: accessPermissions,
			})
			Expect(err).Should(Equal(types.ErrIsExist))
		})
		It("rename file2 to file3 should be succeed", func() {
			chList, err := fsCore.ListChildren(ctx, namespace, group1.ID)
			Expect(err).Should(BeNil())
			file2, err := groupHasChild(chList, "test_group_manage_file2")
			Expect(err).Should(BeNil())

			newName := "test_group_manage_file3"
			opt := types.ChangeParentAttr{}
			err = fsCore.ChangeEntryParent(ctx, namespace, file2, nil, group1.ID, group1.ID, "test_group_manage_file2", newName, opt)
			Expect(err).Should(BeNil())

			chList, err = fsCore.ListChildren(ctx, namespace, group1.ID)
			Expect(err).Should(BeNil())
			_, err = groupHasChild(chList, "test_group_manage_file2")
			Expect(err).Should(Equal(types.ErrNotFound))
			_, err = groupHasChild(chList, "test_group_manage_file3")
			Expect(err).Should(BeNil())
		})
		It("list file1 & file3 should be succeed", func() {
			grp, err := fsCore.OpenGroup(ctx, namespace, group1.ID)
			Expect(err).Should(BeNil())
			entries, err := grp.ListChildren(ctx)
			Expect(err).Should(BeNil())
			fileNames := map[string]bool{}
			for _, en := range entries {
				fileNames[en.Name] = true
			}
			Expect(len(fileNames)).Should(Equal(2))
			Expect(fileNames["test_group_manage_file1"]).Should(BeTrue())
			Expect(fileNames["test_group_manage_file3"]).Should(BeTrue())

		})
		It("delete file1 & file3 should be succeed", func() {
			chList, err := fsCore.ListChildren(ctx, namespace, group1.ID)
			Expect(err).Should(BeNil())
			_, err = groupHasChild(chList, "test_group_manage_file1")
			Expect(err).Should(BeNil())
			_, err = groupHasChild(chList, "test_group_manage_file3")
			Expect(err).Should(BeNil())

			Expect(fsCore.RemoveEntry(ctx, namespace, "/test_group_manage_group1/test_group_manage_file1", types.DeleteEntry{})).Should(BeNil())
			Expect(fsCore.RemoveEntry(ctx, namespace, "/test_group_manage_group1/test_group_manage_file3", types.DeleteEntry{})).Should(BeNil())

			chList, err = fsCore.ListChildren(ctx, namespace, group1.ID)
			Expect(err).Should(BeNil())
			_, err = groupHasChild(chList, "test_group_manage_file1")
			Expect(err).Should(Equal(types.ErrNotFound))
			_, err = groupHasChild(chList, "test_group_manage_file3")
			Expect(err).Should(Equal(types.ErrNotFound))
		})
	})
})

var _ = Describe("TestDynamicGroupEntry", func() {
	var (
		ctx      = context.TODO()
		smtGrp   Group
		smtGrpEn *types.Entry
		err      error
	)
	Context("init dynamic group", func() {
		var srcGrpEn *types.Entry
		It("init source group should be succeed", func() {
			srcGrpEn, err = fsCore.CreateEntry(ctx, namespace, "/", types.EntryAttr{
				Name:   "test_dynamic_source_group",
				Kind:   types.GroupKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			Expect(srcGrpEn).ShouldNot(BeNil())
		})
		It("fill target file to dynamic group should be succeed", func() {
			_, err = fsCore.CreateEntry(ctx, namespace, "/test_dynamic_source_group", types.EntryAttr{
				Name:   "filter-target-file-1.txt",
				Kind:   types.RawKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			_, err = fsCore.CreateEntry(ctx, namespace, "/test_dynamic_source_group", types.EntryAttr{
				Name:       "filter-target-file-2.txt",
				Kind:       types.RawKind,
				Access:     accessPermissions,
				Properties: &types.Properties{Tags: []string{"tag2", "tag3"}},
			})
			Expect(err).Should(BeNil())
			_, err = fsCore.CreateEntry(ctx, namespace, "/test_dynamic_source_group", types.EntryAttr{
				Name:       "filter-target-file-3.txt",
				Kind:       types.RawKind,
				Access:     accessPermissions,
				Properties: &types.Properties{Tags: []string{"tag2", "tag4"}},
			})
			Expect(err).Should(BeNil())
			_, err = fsCore.CreateEntry(ctx, namespace, "/test_dynamic_source_group", types.EntryAttr{
				Name:       "filter-target-file-4.txt",
				Kind:       types.RawKind,
				Access:     accessPermissions,
				Properties: &types.Properties{Tags: []string{"tag1", "tag2"}},
			})
			Expect(err).Should(BeNil())
		})
	})
	Context("init dynamic group", func() {
		It("init smart group should be succeed", func() {
			var err error
			smtGrpEn, err = fsCore.CreateEntry(ctx, namespace, "/", types.EntryAttr{
				Name:   "test_dynamic_group",
				Kind:   types.SmartGroupKind,
				Access: accessPermissions,
				GroupProperties: &types.GroupProperties{
					Filter: &types.Filter{CELPattern: `"tag2" in tags`},
				},
			})
			Expect(err).Should(BeNil())
			Expect(smtGrpEn).ShouldNot(BeNil())
		})

		It("list smart group should be succeed", func() {
			smtGrp, err = fsCore.OpenGroup(ctx, namespace, smtGrpEn.ID)
			Expect(err).Should(BeNil())
			Expect(smtGrp).ShouldNot(BeNil())

			children, err := smtGrp.ListChildren(ctx)
			Expect(err).Should(BeNil())
			Expect(len(children)).Should(Equal(3))
		})
	})
})
