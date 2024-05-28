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

	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
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
				Labels: types.Labels{
					Labels: []types.Label{
						{Key: "test.filter.key1", Value: "test.filter.val1"},
						{Key: "test.filter.key2", Value: "test.filter.val1"},
					},
				},
			})
			Expect(err).Should(BeNil())
			_, err = entryManager.CreateEntry(ctx, srcGrpEn.ID, types.EntryAttr{
				Name:   "filter-target-file-2.txt",
				Kind:   types.RawKind,
				Access: accessPermissions,
				Labels: types.Labels{
					Labels: []types.Label{
						{Key: "test.filter.key1", Value: "test.filter.val2"},
						{Key: "test.filter.key2", Value: "test.filter.val2"},
					},
				},
			})
			Expect(err).Should(BeNil())
			_, err = entryManager.CreateEntry(ctx, srcGrpEn.ID, types.EntryAttr{
				Name:   "filter-target-file-3.txt",
				Kind:   types.RawKind,
				Access: accessPermissions,
				Labels: types.Labels{
					Labels: []types.Label{
						{Key: "test.filter.key1", Value: "test.filter.val1"},
						{Key: "test.filter.key2", Value: "test.filter.val3"},
					},
				},
			})
			Expect(err).Should(BeNil())
			_, err = entryManager.CreateEntry(ctx, srcGrpEn.ID, types.EntryAttr{
				Name:   "filter-target-file-4.txt",
				Kind:   types.RawKind,
				Access: accessPermissions,
				Labels: types.Labels{Labels: []types.Label{}},
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
				GroupFilter: &types.Rule{
					Logic: types.RuleLogicAny,
					Rules: []types.Rule{
						{Labels: &types.LabelMatch{Include: []types.Label{{Key: "test.filter.key1", Value: "test.filter.val1"}}}},
						{Labels: &types.LabelMatch{Include: []types.Label{{Key: "test.filter.key2", Value: "test.filter.val2"}}}},
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

var _ = Describe("TestExtGroupEntry", func() {
	var (
		extGrp       Group
		extGrpEn     *types.Metadata
		file1, file2 *types.Metadata
		err          error
	)
	Context("init ext group", func() {
		It("init group should be succeed", func() {
			var err error
			extGrpEn, err = entryManager.CreateEntry(context.TODO(), root.ID, types.EntryAttr{
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

			extGrp, err = entryManager.OpenGroup(context.TODO(), extGrpEn.ID)
			Expect(err).Should(BeNil())
			Expect(extGrp).ShouldNot(BeNil())
		})
	})

	Context("create file", func() {
		It("create file1.yaml should be succeed", func() {
			file1, err = entryManager.CreateEntry(context.TODO(), extGrpEn.ID, types.EntryAttr{
				Name:   "file1.yaml",
				Kind:   types.RawKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			Expect(file1).ShouldNot(BeNil())
		})
		It("write and read file1.yaml should be succeed", func() {
			var f File
			f, err = entryManager.Open(context.TODO(), file1.ID, types.OpenAttr{Read: true, Write: true})
			Expect(err).Should(BeNil())

			_, err = f.WriteAt(context.TODO(), []byte("file1: hello world!"), 0)
			Expect(err).Should(BeNil())
			err = f.Close(context.TODO())
			Expect(err).Should(BeNil())

			f, err = entryManager.Open(context.TODO(), file1.ID, types.OpenAttr{Read: true, Write: true})
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
			file2, err = entryManager.CreateEntry(context.TODO(), extGrpEn.ID, types.EntryAttr{
				Name:   "file2.yaml",
				Kind:   types.RawKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			Expect(file1).ShouldNot(BeNil())
		})
		It("write and read file2.yaml should be succeed", func() {
			var f File
			f, err = entryManager.Open(context.TODO(), file2.ID, types.OpenAttr{Read: true, Write: true})
			Expect(err).Should(BeNil())

			_, err = f.WriteAt(context.TODO(), []byte("file2: hello world!"), 0)
			Expect(err).Should(BeNil())
			err = f.Close(context.TODO())
			Expect(err).Should(BeNil())

			f, err = entryManager.Open(context.TODO(), file2.ID, types.OpenAttr{Read: true, Write: true})
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
			_, err = entryManager.CreateEntry(context.TODO(), extGrpEn.ID, types.EntryAttr{
				Name:   "file1.yaml",
				Kind:   types.RawKind,
				Access: accessPermissions,
			})
			Expect(err).Should(Equal(types.ErrIsExist))
		})
	})

	Context("delete file", func() {
		It("list files should be succeed", func() {
			extGrp, err = entryManager.OpenGroup(context.TODO(), extGrpEn.ID)
			Expect(err).Should(BeNil())
			Expect(extGrp).ShouldNot(BeNil())

			child, err := extGrp.ListChildren(context.TODO(), nil, types.Filter{})
			Expect(err).Should(BeNil())
			Expect(len(child)).Should(Equal(2))

			need := map[string]struct{}{"file1.yaml": {}, "file2.yaml": {}}
			for _, ch := range child {
				delete(need, ch.Name)
			}
			Expect(len(need)).Should(Equal(0))
		})
		It("delete file2.yaml should be succeed", func() {
			err = entryManager.RemoveEntry(context.TODO(), extGrpEn.ID, file2.ID)
			Expect(err).Should(BeNil())
		})
		It("list files and file2.yaml should gone", func() {
			extGrp, err = entryManager.OpenGroup(context.TODO(), extGrpEn.ID)
			Expect(err).Should(BeNil())
			Expect(extGrp).ShouldNot(BeNil())

			child, err := extGrp.ListChildren(context.TODO(), nil, types.Filter{})
			Expect(err).Should(BeNil())
			Expect(len(child)).Should(Equal(1))

			need := map[string]struct{}{"file1.yaml": {}}
			for _, ch := range child {
				delete(need, ch.Name)
			}
			Expect(len(need)).Should(Equal(0))
		})
	})

	Context("ext group dir", func() {
		var (
			dir1       *types.Metadata
			innerFile1 *types.Metadata
		)
		It("create dir1 should be succeed", func() {
			dir1, err = entryManager.CreateEntry(context.TODO(), extGrpEn.ID, types.EntryAttr{
				Name:   "dir1",
				Kind:   types.ExternalGroupKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			Expect(dir1).ShouldNot(BeNil())
		})
		It("create dir1/file1.yaml should be succeed", func() {
			innerFile1, err = entryManager.CreateEntry(context.TODO(), dir1.ID, types.EntryAttr{
				Name:   "file1.yaml",
				Kind:   types.RawKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			Expect(innerFile1).ShouldNot(BeNil())
		})
		It("delete dir1 should be failed(dir not empty)", func() {
			err = entryManager.RemoveEntry(context.TODO(), extGrpEn.ID, dir1.ID)
			Expect(err).Should(Equal(types.ErrNotEmpty))
		})
		It("delete dir1/file1.yaml should be succeed", func() {
			err = entryManager.RemoveEntry(context.TODO(), dir1.ID, innerFile1.ID)
			Expect(err).Should(BeNil())
		})
		It("delete dir1 should be succeed", func() {
			err = entryManager.RemoveEntry(context.TODO(), extGrpEn.ID, dir1.ID)
			Expect(err).Should(BeNil())
		})
	})

	Context("sync file", func() {
		var memFS plugin.MirrorPlugin

		It("load memfs should succeed", func() {
			var ed types.ExtendData
			ed, err = entryManager.GetEntryExtendData(context.TODO(), extGrpEn.ID)
			memFS, err = plugin.NewMirrorPlugin(context.TODO(), *ed.PlugScope)
			Expect(err).Should(BeNil())
		})
		It("only file1.yaml in ext group", func() {
			extGrp, err = entryManager.OpenGroup(context.TODO(), extGrpEn.ID)
			Expect(err).Should(BeNil())
			Expect(extGrp).ShouldNot(BeNil())

			child, err := extGrp.ListChildren(context.TODO(), nil, types.Filter{})
			Expect(err).Should(BeNil())

			need := map[string]struct{}{"file1.yaml": {}}
			for _, ch := range child {
				delete(need, ch.Name)
			}
			Expect(len(need)).Should(Equal(0))
		})
		It("insert sync_file1.yaml to memfs should be succeed", func() {
			_, err = memFS.CreateEntry(context.TODO(), "/", pluginapi.EntryAttr{
				Name: "sync_file1.yaml",
				Kind: types.RawKind,
			})
			Expect(err).Should(BeNil())
		})
		It("list files should contain sync_file1.yaml", func() {
			extGrp, err = entryManager.OpenGroup(context.TODO(), extGrpEn.ID)
			Expect(err).Should(BeNil())
			Expect(extGrp).ShouldNot(BeNil())

			child, err := extGrp.ListChildren(context.TODO(), nil, types.Filter{})
			Expect(err).Should(BeNil())

			need := map[string]struct{}{"file1.yaml": {}, "sync_file1.yaml": {}}
			for _, ch := range child {
				delete(need, ch.Name)
			}
			Expect(len(need)).Should(Equal(0))
		})
		It("insert sync_file2.yaml to memfs should be succeed", func() {
			_, err = memFS.CreateEntry(context.TODO(), "/", pluginapi.EntryAttr{
				Name: "sync_file2.yaml",
				Kind: types.RawKind,
			})
			Expect(err).Should(BeNil())

			extGrp, err = entryManager.OpenGroup(context.TODO(), extGrpEn.ID)
			Expect(err).Should(BeNil())
			Expect(extGrp).ShouldNot(BeNil())

			child, err := extGrp.ListChildren(context.TODO(), nil, types.Filter{})
			Expect(err).Should(BeNil())

			need := map[string]struct{}{"file1.yaml": {}, "sync_file1.yaml": {}, "sync_file2.yaml": {}}
			for _, ch := range child {
				delete(need, ch.Name)
			}
			Expect(len(need)).Should(Equal(0))
		})
		It("delete sync_file2.yaml should be succeed", func() {
			err = memFS.RemoveEntry(context.TODO(), "/sync_file2.yaml")
			Expect(err).Should(BeNil())

			extGrp, err = entryManager.OpenGroup(context.TODO(), extGrpEn.ID)
			Expect(err).Should(BeNil())
			Expect(extGrp).ShouldNot(BeNil())

			child, err := extGrp.ListChildren(context.TODO(), nil, types.Filter{})
			Expect(err).Should(BeNil())

			need := map[string]struct{}{"file1.yaml": {}, "sync_file1.yaml": {}}
			for _, ch := range child {
				delete(need, ch.Name)
			}
			Expect(len(need)).Should(Equal(0))
		})
	})

	Context("mv file", func() {
		var (
			outDir    *types.Metadata
			outFile   *types.Metadata
			innerDir1 *types.Metadata
			movedEn   *types.Metadata
		)
		It("init dir1", func() {
			innerDir1, err = entryManager.CreateEntry(context.TODO(), extGrpEn.ID, types.EntryAttr{
				Name:   "dir1",
				Kind:   types.ExternalGroupKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			Expect(innerDir1).ShouldNot(BeNil())
		})
		It("move file1.yaml to dir1/moved_file1.yaml in same ext group should be succeed", func() {
			err = entryManager.ChangeEntryParent(context.TODO(), file1.ID, nil, extGrpEn.ID, innerDir1.ID, "moved_file1.yaml", types.ChangeParentAttr{})
			Expect(err).Should(BeNil())

			var (
				file1F File
			)
			grp, err := entryManager.OpenGroup(context.TODO(), innerDir1.ID)
			Expect(err).Should(BeNil())
			movedEn, err = grp.FindEntry(context.TODO(), "moved_file1.yaml")
			Expect(err).Should(BeNil())

			Expect(movedEn.ID >> entryIDPrefixMask).Should(Equal(externalIDPrefix))

			file1F, err = entryManager.Open(context.TODO(), movedEn.ID, types.OpenAttr{Write: true})
			Expect(err).Should(BeNil())

			var (
				buf = make([]byte, 64)
				n   int64
			)
			n, err = file1F.ReadAt(context.TODO(), buf, 0)
			Expect(err).Should(BeNil())
			err = file1F.Close(context.TODO())
			Expect(err).Should(BeNil())

			Expect(string(buf[:n])).Should(Equal("file1: hello world!"))
		})
		It("create test_ext_group_file_1.yaml in /out_dir should be succeed", func() {
			outDir, err = entryManager.CreateEntry(context.TODO(), root.ID, types.EntryAttr{
				Name:   "out_dir",
				Kind:   types.GroupKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			Expect(innerDir1).ShouldNot(BeNil())

			outFile, err = entryManager.CreateEntry(context.TODO(), outDir.ID, types.EntryAttr{
				Name:   "test_ext_group_file_1.yaml",
				Kind:   types.RawKind,
				Access: accessPermissions,
			})
			Expect(err).Should(BeNil())
			Expect(outFile).ShouldNot(BeNil())

			var outFile1F File
			outFile1F, err = entryManager.Open(context.TODO(), outFile.ID, types.OpenAttr{Write: true})
			Expect(err).Should(BeNil())
			_, err = outFile1F.WriteAt(context.TODO(), []byte("test_ext_group_file_1: hello world!"), 0)
			Expect(err).Should(BeNil())
			err = outFile1F.Close(context.TODO())
			Expect(err).Should(BeNil())
		})
		It("overwrite /out_dir/test_ext_group_file_1.yaml to ext group should be succeed", func() {
			grp, err := entryManager.OpenGroup(context.TODO(), innerDir1.ID)
			Expect(err).Should(BeNil())

			movedEn, err = grp.FindEntry(context.TODO(), "moved_file1.yaml")
			Expect(err).Should(BeNil())

			err = entryManager.ChangeEntryParent(context.TODO(), outFile.ID, &movedEn.ID, outDir.ID, innerDir1.ID, "moved_file1.yaml", types.ChangeParentAttr{Replace: true})
			Expect(err).Should(BeNil())

			var (
				overwritedEn   *types.Metadata
				overwritedFile File
			)
			overwritedEn, err = grp.FindEntry(context.TODO(), "moved_file1.yaml")
			Expect(err).Should(BeNil())

			overwritedFile, err = entryManager.Open(context.TODO(), overwritedEn.ID, types.OpenAttr{Write: true})
			Expect(err).Should(BeNil())

			var (
				buf = make([]byte, 64)
				n   int64
			)
			n, err = overwritedFile.ReadAt(context.TODO(), buf, 0)
			Expect(err).Should(BeNil())
			err = overwritedFile.Close(context.TODO())
			Expect(err).Should(BeNil())

			Expect(string(buf[:n])).Should(Equal("test_ext_group_file_1: hello world!"))
		})
	})

})
