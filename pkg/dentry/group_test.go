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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/basenana/nanafs/pkg/types"
)

var _ = Describe("TestManageGroupEntry", func() {
	var (
		group1 *types.Metadata
		file1  *types.Metadata
	)
	Context("init group1", func() {
		It("init group should be succeed", func() {
			var err error
			group1, err = entryManager.CreateEntry(context.TODO(), root.ID, types.EntryAttr{
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
			file1, err = entryManager.CreateEntry(context.TODO(), group1.ID, types.EntryAttr{
				Name:   "test_group_manage_file1",
				Kind:   types.GroupKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			Expect(file1).ShouldNot(BeNil())
		})
		It("create file2 should be succeed", func() {
			_, err := entryManager.CreateEntry(context.TODO(), group1.ID, types.EntryAttr{
				Name:   "test_group_manage_file2",
				Kind:   types.GroupKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
		})
		It("create file1 should be existed", func() {
			_, err := entryManager.CreateEntry(context.TODO(), group1.ID, types.EntryAttr{
				Name:   "test_group_manage_file1",
				Kind:   types.GroupKind,
				Access: accessPermissions,
			})
			Expect(err).Should(Equal(types.ErrIsExist))
		})
		It("rename file2 to file3 should be succeed", func() {
			grp, err := entryManager.OpenGroup(context.TODO(), group1.ID)
			Expect(err).Should(BeNil())
			file2, err := grp.FindEntry(context.TODO(), "test_group_manage_file2")
			Expect(err).Should(BeNil())

			file2.Name = "test_group_manage_file3"
			err = grp.UpdateEntry(context.TODO(), file2)
			Expect(err).Should(BeNil())

			_, err = grp.FindEntry(context.TODO(), "test_group_manage_file2")
			Expect(err).Should(Equal(types.ErrNotFound))
			_, err = grp.FindEntry(context.TODO(), "test_group_manage_file3")
			Expect(err).Should(BeNil())
		})
		It("list file1 & file3 should be succeed", func() {
			grp, err := entryManager.OpenGroup(context.TODO(), group1.ID)
			Expect(err).Should(BeNil())
			entries, err := grp.ListChildren(context.TODO(), nil, types.Filter{})
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
			grp, err := entryManager.OpenGroup(context.TODO(), group1.ID)
			Expect(err).Should(BeNil())
			file1, err := grp.FindEntry(context.TODO(), "test_group_manage_file1")
			Expect(err).Should(BeNil())
			file3, err := grp.FindEntry(context.TODO(), "test_group_manage_file3")
			Expect(err).Should(BeNil())

			Expect(grp.RemoveEntry(context.TODO(), file1.ID)).Should(BeNil())
			Expect(grp.RemoveEntry(context.TODO(), file3.ID)).Should(BeNil())

			file1, err = grp.FindEntry(context.TODO(), "test_group_manage_file1")
			Expect(err).Should(Equal(types.ErrNotFound))
			file3, err = grp.FindEntry(context.TODO(), "test_group_manage_file3")
			Expect(err).Should(Equal(types.ErrNotFound))
		})
	})
})

var _ = Describe("TestDynamicGroupEntry", func() {
	var (
		ctx      = context.TODO()
		smtGrp   Group
		smtGrpEn *types.Metadata
		err      error
	)
	Context("init source group", func() {
		var srcGrpEn *types.Metadata
		It("init source group should be succeed", func() {
			srcGrpEn, err = entryManager.CreateEntry(ctx, root.ID, types.EntryAttr{
				Name:   "test_dynamic_source_group",
				Kind:   types.GroupKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			Expect(srcGrpEn).ShouldNot(BeNil())
		})
		It("fill target file to source group should be succeed", func() {
			_, err = entryManager.CreateEntry(ctx, srcGrpEn.ID, types.EntryAttr{
				Name:   "filter-target-file-1.txt",
				Kind:   types.RawKind,
				Access: accessPermissions,
				ExtendData: types.ExtendData{
					Labels: types.Labels{
						Labels: []types.Label{
							{Key: "test.filter.key1", Value: "test.filter.val1"},
							{Key: "test.filter.key2", Value: "test.filter.val1"},
						},
					},
				},
			})
			Expect(err).Should(BeNil())
			_, err = entryManager.CreateEntry(ctx, srcGrpEn.ID, types.EntryAttr{
				Name:   "filter-target-file-2.txt",
				Kind:   types.RawKind,
				Access: accessPermissions,
				ExtendData: types.ExtendData{
					Labels: types.Labels{
						Labels: []types.Label{
							{Key: "test.filter.key1", Value: "test.filter.val2"},
							{Key: "test.filter.key2", Value: "test.filter.val2"},
						},
					},
				},
			})
			Expect(err).Should(BeNil())
			_, err = entryManager.CreateEntry(ctx, srcGrpEn.ID, types.EntryAttr{
				Name:   "filter-target-file-3.txt",
				Kind:   types.RawKind,
				Access: accessPermissions,
				ExtendData: types.ExtendData{
					Labels: types.Labels{
						Labels: []types.Label{
							{Key: "test.filter.key1", Value: "test.filter.val1"},
							{Key: "test.filter.key2", Value: "test.filter.val3"},
						},
					},
				},
			})
			Expect(err).Should(BeNil())
			_, err = entryManager.CreateEntry(ctx, srcGrpEn.ID, types.EntryAttr{
				Name:   "filter-target-file-4.txt",
				Kind:   types.RawKind,
				Access: accessPermissions,
				ExtendData: types.ExtendData{
					Labels: types.Labels{Labels: []types.Label{}},
				},
			})
			Expect(err).Should(BeNil())
		})
	})
	Context("init dynamic group", func() {
		It("init smart group should be succeed", func() {
			var err error
			smtGrpEn, err = entryManager.CreateEntry(ctx, root.ID, types.EntryAttr{
				Name:   "test_dynamic_group",
				Kind:   types.SmartGroupKind,
				Access: accessPermissions,
				ExtendData: types.ExtendData{
					GroupFilter: &types.Rule{
						Logic: types.RuleLogicAny,
						Rules: []types.Rule{
							{Labels: &types.LabelMatch{Include: []types.Label{{Key: "test.filter.key1", Value: "test.filter.val1"}}}},
							{Labels: &types.LabelMatch{Include: []types.Label{{Key: "test.filter.key2", Value: "test.filter.val2"}}}},
						},
					},
				},
			})
			Expect(err).Should(BeNil())
			Expect(smtGrpEn).ShouldNot(BeNil())
		})

		It("list smart group should be succeed", func() {
			smtGrp, err = entryManager.OpenGroup(ctx, smtGrpEn.ID)
			Expect(err).Should(BeNil())
			Expect(smtGrp).ShouldNot(BeNil())

			children, err := smtGrp.ListChildren(ctx, nil, types.Filter{})
			Expect(err).Should(BeNil())
			Expect(len(children)).ShouldNot(Equal(3))
		})
	})
})
