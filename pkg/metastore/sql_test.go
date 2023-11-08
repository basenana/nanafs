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
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"path"
	"sort"
	"strings"
)

var _ = Describe("TestSqliteObjectOperation", func() {
	var sqlite = buildNewSqliteMetaStore("test_object.db")
	// init root
	rootEn := InitRootEntry()
	Expect(sqlite.CreateEntry(context.TODO(), 0, rootEn)).Should(BeNil())

	Context("create a new file object", func() {
		It("should be succeed", func() {
			en, err := types.InitNewEntry(rootEn, types.EntryAttr{
				Name: "test-new-en-1",
				Kind: types.RawKind,
			})
			Expect(err).Should(BeNil())

			err = sqlite.CreateEntry(context.TODO(), rootEn.ID, en)
			Expect(err).Should(BeNil())

			fetchObj, err := sqlite.GetEntry(context.TODO(), en.ID)
			Expect(err).Should(BeNil())
			Expect(fetchObj.Name).Should(Equal("test-new-en-1"))
		})
	})

	Context("update a exist file object", func() {
		It("should be succeed", func() {
			en, err := types.InitNewEntry(rootEn, types.EntryAttr{
				Name: "test-update-en-1",
				Kind: types.RawKind,
			})
			Expect(err).Should(BeNil())

			err = sqlite.CreateEntry(context.TODO(), rootEn.ID, en)
			Expect(err).Should(BeNil())

			en.Name = "test-update-en-2"
			err = sqlite.UpdateEntryMetadata(context.TODO(), en)
			Expect(err).Should(BeNil())

			newObj, err := sqlite.GetEntry(context.TODO(), en.ID)
			Expect(err).Should(BeNil())
			Expect(newObj.Name).Should(Equal("test-update-en-2"))
		})
	})

	Context("delete a exist file object", func() {
		It("should be succeed", func() {
			en, err := types.InitNewEntry(rootEn, types.EntryAttr{
				Name: "test-delete-en-1",
				Kind: types.RawKind,
			})
			Expect(err).Should(BeNil())

			err = sqlite.CreateEntry(context.TODO(), rootEn.ID, en)
			Expect(err).Should(BeNil())

			_, err = sqlite.FindEntry(context.TODO(), rootEn.ID, en.Name)
			Expect(err).Should(BeNil())

			Expect(sqlite.RemoveEntry(context.TODO(), en.ParentID, en.ID)).Should(BeNil())

			_, err = sqlite.FindEntry(context.TODO(), rootEn.ID, en.Name)
			Expect(err).Should(Equal(types.ErrNotFound))
		})
	})

	Context("create a new group object", func() {
		It("should be succeed", func() {
			en, err := types.InitNewEntry(rootEn, types.EntryAttr{
				Name: "test-new-group-1",
				Kind: types.GroupKind,
			})
			Expect(err).Should(BeNil())

			err = sqlite.CreateEntry(context.TODO(), rootEn.ID, en)
			Expect(err).Should(BeNil())

			fetchEn, err := sqlite.GetEntry(context.TODO(), en.ID)
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
	Expect(sqlite.CreateEntry(context.TODO(), 0, rootEn)).Should(BeNil())

	group1, err := types.InitNewEntry(rootEn, types.EntryAttr{
		Name: "test-new-group-1",
		Kind: types.GroupKind,
	})
	Expect(err).Should(BeNil())
	Expect(sqlite.CreateEntry(context.TODO(), rootEn.ID, group1)).Should(BeNil())

	group2, err := types.InitNewEntry(rootEn, types.EntryAttr{
		Name: "test-new-group-2",
		Kind: types.GroupKind,
	})
	Expect(err).Should(BeNil())
	Expect(sqlite.CreateEntry(context.TODO(), rootEn.ID, group2)).Should(BeNil())

	Context("list a group object all children", func() {
		It("create group file should be succeed", func() {
			for i := 0; i < 4; i++ {
				en, err := types.InitNewEntry(group1, types.EntryAttr{Name: "test-file-en-1", Kind: types.RawKind})
				Expect(err).Should(BeNil())
				Expect(sqlite.CreateEntry(context.TODO(), group1.ID, en)).Should(BeNil())
			}

			en, err := types.InitNewEntry(group1, types.EntryAttr{Name: "test-dev-en-1", Kind: types.BlkDevKind})
			Expect(err).Should(BeNil())
			Expect(sqlite.CreateEntry(context.TODO(), group1.ID, en)).Should(BeNil())
		})

		It("list new file object should be succeed", func() {
			chIt, err := sqlite.ListEntryChildren(context.TODO(), group1.ID)
			Expect(err).Should(BeNil())

			chList := make([]*types.Metadata, 0)
			for chIt.HasNext() {
				chList = append(chList, chIt.Next())
			}

			Expect(len(chList)).Should(Equal(5))
		})
	})

	Context("change a exist file object parent group", func() {
		var targetEn *types.Metadata
		It("get target file object should be succeed", func() {
			chIt, err := sqlite.FilterEntries(context.TODO(), types.Filter{ParentID: group1.ID, Kind: types.BlkDevKind})
			Expect(err).Should(BeNil())

			chList := make([]*types.Metadata, 0)
			for chIt.HasNext() {
				chList = append(chList, chIt.Next())
			}
			Expect(len(chList)).Should(Equal(1))
			targetEn = chList[0]
		})

		It("should be succeed", func() {
			err = sqlite.ChangeEntryParent(context.TODO(), targetEn.ID, group2.ID, targetEn.Name, types.ChangeParentAttr{})
			Expect(err).Should(BeNil())

			chIt, err := sqlite.ListEntryChildren(context.TODO(), group2.ID)
			Expect(err).Should(BeNil())

			chList := make([]*types.Metadata, 0)
			for chIt.HasNext() {
				chList = append(chList, chIt.Next())
			}

			Expect(len(chList)).Should(Equal(1))
		})

		It("query target file object in old group should not found", func() {
			chIt, err := sqlite.FilterEntries(context.TODO(), types.Filter{ParentID: group1.ID, Kind: types.BlkDevKind})
			Expect(err).Should(BeNil())

			chList := make([]*types.Metadata, 0)
			for chIt.HasNext() {
				chList = append(chList, chIt.Next())
			}
			Expect(len(chList)).Should(Equal(0))

			chIt, err = sqlite.ListEntryChildren(context.TODO(), group1.ID)
			Expect(err).Should(BeNil())

			chList = make([]*types.Metadata, 0)
			for chIt.HasNext() {
				chList = append(chList, chIt.Next())
			}
			Expect(len(chList)).Should(Equal(4))
		})
	})

	Context("mirror a exist file object to other group", func() {
		group3, err := types.InitNewEntry(rootEn, types.EntryAttr{
			Name: "test-mirror-group-1",
			Kind: types.GroupKind,
		})
		Expect(err).Should(BeNil())
		Expect(sqlite.CreateEntry(context.TODO(), rootEn.ID, group3)).Should(BeNil())

		group4, err := types.InitNewEntry(rootEn, types.EntryAttr{
			Name: "test-mirror-group-2",
			Kind: types.GroupKind,
		})
		Expect(err).Should(BeNil())
		Expect(sqlite.CreateEntry(context.TODO(), rootEn.ID, group4)).Should(BeNil())

		var srcEN *types.Metadata
		It("create new file be succeed", func() {
			srcEN, err = types.InitNewEntry(group1, types.EntryAttr{Name: "test-src-raw-obj-1", Kind: types.RawKind})
			Expect(err).Should(BeNil())
			Expect(sqlite.CreateEntry(context.TODO(), group1.ID, srcEN)).Should(BeNil())
		})

		It("create mirror object should be succeed", func() {
			newEn, err := types.InitNewEntry(rootEn, types.EntryAttr{Name: "test-mirror-dst-file-2", Kind: types.RawKind})
			Expect(err).Should(BeNil())
			newEn.RefID = srcEN.ID
			Expect(sqlite.MirrorEntry(context.TODO(), newEn)).Should(BeNil())
		})

		It("filter mirror object should be succeed", func() {
			chIt, err := sqlite.FilterEntries(context.TODO(), types.Filter{RefID: srcEN.ID})
			Expect(err).Should(BeNil())

			chList := make([]*types.Metadata, 0)
			for chIt.HasNext() {
				chList = append(chList, chIt.Next())
			}
			Expect(len(chList)).Should(Equal(1))
		})
	})
})

var _ = Describe("TestSqliteLabelOperation", func() {
	var (
		ctx    = context.TODO()
		sqlite = buildNewSqliteMetaStore("test_label_operation.db")
	)
	// init root
	rootEn := InitRootEntry()
	Expect(sqlite.CreateEntry(context.TODO(), 0, rootEn)).Should(BeNil())

	Context("save labels", func() {
		It("create object with/without labels should succeed", func() {
			entry1, err := types.InitNewEntry(rootEn, types.EntryAttr{Name: "test-label-obj-1", Kind: types.RawKind})
			Expect(err).Should(BeNil())
			Expect(sqlite.CreateEntry(context.TODO(), rootEn.ID, entry1)).Should(BeNil())

			entry2, err := types.InitNewEntry(rootEn, types.EntryAttr{Name: "test-label-obj-2", Kind: types.RawKind})
			Expect(err).Should(BeNil())
			Expect(sqlite.CreateEntry(context.TODO(), rootEn.ID, entry2)).Should(BeNil())

			Expect(sqlite.UpdateEntryLabels(context.TODO(), entry2.ID, types.Labels{Labels: []types.Label{
				{Key: "test.nanafs.label1", Value: "cus_value"},
				{Key: "test.nanafs.label2", Value: "cus_value"},
			}})).Should(BeNil())

			entry3, err := types.InitNewEntry(rootEn, types.EntryAttr{Name: "test-label-obj-3", Kind: types.RawKind})
			Expect(err).Should(BeNil())
			Expect(sqlite.CreateEntry(context.TODO(), rootEn.ID, entry3)).Should(BeNil())

			Expect(sqlite.UpdateEntryLabels(context.TODO(), entry3.ID, types.Labels{Labels: []types.Label{
				{Key: "test.nanafs.label2", Value: "cus_value"},
			}})).Should(BeNil())

			Expect(sqlite.UpdateEntryExtendData(context.TODO(), entry3.ID, types.ExtendData{
				Properties: types.Properties{Fields: map[string]string{"custom_field": "cus_value"}},
			})).Should(BeNil())
		})
		It("add object labels should succeed", func() {
			entry, err := sqlite.FindEntry(ctx, rootEn.ID, "test-label-obj-1")
			Expect(err).Should(BeNil())

			Expect(sqlite.UpdateEntryLabels(context.TODO(), entry.ID, types.Labels{Labels: []types.Label{
				{Key: "test.nanafs.label1", Value: "cus_value"},
				{Key: "test.nanafs.label2", Value: "cus_value2"},
			}})).Should(BeNil())
		})
		It("update object labels should succeed", func() {
			entry, err := sqlite.FindEntry(ctx, rootEn.ID, "test-label-obj-3")
			Expect(err).Should(BeNil())

			Expect(sqlite.UpdateEntryLabels(context.TODO(), entry.ID, types.Labels{Labels: []types.Label{
				{Key: "test.nanafs.label1", Value: "cus_value"},
				{Key: "test.nanafs.label2", Value: "cus_value2"},
				{Key: "test.nanafs.label3", Value: "cus_value3"},
			}})).Should(BeNil())
		})
	})

	/*
		test-label-obj-1
			- test.nanafs.label1=cus_value
			- test.nanafs.label2=cus_value2

		test-label-obj-2
			- test.nanafs.label1=cus_value
			- test.nanafs.label2=cus_value

		test-label-obj-3
			- test.nanafs.label1=cus_value
			- test.nanafs.label2=cus_value2
			- test.nanafs.label3=cus_value3
	*/

	Context("query object with labels", func() {
		It("list object with labels test.nanafs.label1 should succeed", func() {
			enIt, err := sqlite.FilterEntries(ctx, types.Filter{
				Label: types.LabelMatch{
					Include: []types.Label{{Key: "test.nanafs.label1", Value: "cus_value"}},
				},
			})
			Expect(err).Should(BeNil())

			chList := make([]*types.Metadata, 0)
			for enIt.HasNext() {
				chList = append(chList, enIt.Next())
			}
			Expect(len(chList)).Should(Equal(3))
		})
		It("list object with labels test.nanafs.label2 should succeed", func() {
			enIt, err := sqlite.FilterEntries(ctx, types.Filter{
				Label: types.LabelMatch{
					Include: []types.Label{{Key: "test.nanafs.label2", Value: "cus_value2"}},
				},
			})
			Expect(err).Should(BeNil())

			chList := make([]*types.Metadata, 0)
			for enIt.HasNext() {
				chList = append(chList, enIt.Next())
			}
			Expect(len(chList)).Should(Equal(2))
		})
		It("list object with label exclude should succeed", func() {
			enIt, err := sqlite.FilterEntries(ctx, types.Filter{
				Label: types.LabelMatch{
					Include: []types.Label{{Key: "test.nanafs.label2", Value: "cus_value2"}},
					Exclude: []string{"test.nanafs.label3"},
				},
			})
			Expect(err).Should(BeNil())

			chList := make([]*types.Metadata, 0)
			for enIt.HasNext() {
				chList = append(chList, enIt.Next())
			}
			Expect(len(chList)).Should(Equal(1))
		})
	})
})

var _ = Describe("TestSqlitePluginData", func() {
	var (
		sqlite      = buildNewSqliteMetaStore("test_plugin_data.db")
		pluginASpec = types.PlugScope{PluginName: "plugin-a", Version: "v1.0", PluginType: types.TypeMirror, Parameters: map[string]string{}}
		pluginBSpec = types.PlugScope{PluginName: "plugin-b", Version: "v1.0", PluginType: types.TypeSource, Parameters: map[string]string{}}
	)

	Context("fetch plugin data", func() {
		It("save plugins data should be succeed", func() {
			pluginARecorder := sqlite.PluginRecorder(pluginASpec)
			Expect(pluginARecorder.SaveRecord(context.TODO(), "group_1", "rid_1", map[string]string{"key": "plugin_a"})).Should(BeNil())

			pluginBRecorder := sqlite.PluginRecorder(pluginBSpec)
			Expect(pluginBRecorder.SaveRecord(context.TODO(), "group_1", "rid_1", map[string]string{"key": "plugin_b"})).Should(BeNil())
		})

		It("fetch plugin A should be succeed", func() {
			pluginARecorder := sqlite.PluginRecorder(pluginASpec)
			data := map[string]string{}
			Expect(pluginARecorder.GetRecord(context.TODO(), "rid_1", &data)).Should(BeNil())
			Expect(data["key"]).Should(Equal("plugin_a"))
		})

		It("fetch plugin B should be succeed", func() {
			pluginBRecorder := sqlite.PluginRecorder(pluginBSpec)
			data := map[string]string{}
			Expect(pluginBRecorder.GetRecord(context.TODO(), "rid_1", &data)).Should(BeNil())
			Expect(data["key"]).Should(Equal("plugin_b"))
		})

		It("update plugin B old data should be succeed", func() {
			pluginBRecorder := sqlite.PluginRecorder(pluginBSpec)
			Expect(pluginBRecorder.SaveRecord(context.TODO(), "group_1", "rid_1", map[string]string{"key2": "plugin_b"})).Should(BeNil())

			data := map[string]string{}
			Expect(pluginBRecorder.GetRecord(context.TODO(), "rid_1", &data)).Should(BeNil())
			Expect(data["key2"]).Should(Equal("plugin_b"))
		})
	})

	Context("list plugin group data", func() {
		It("should be succeed", func() {
			pluginARecorder := sqlite.PluginRecorder(pluginASpec)
			Expect(pluginARecorder.SaveRecord(context.TODO(), "group_1", "rid_2", map[string]string{"key": "plugin_a"})).Should(BeNil())
			Expect(pluginARecorder.SaveRecord(context.TODO(), "group_1", "rid_3", map[string]string{"key": "plugin_a"})).Should(BeNil())
			Expect(pluginARecorder.SaveRecord(context.TODO(), "group_1", "rid_4", map[string]string{"key": "plugin_a"})).Should(BeNil())

			recordList, err := pluginARecorder.ListRecords(context.TODO(), "group_1")
			Expect(err).Should(BeNil())
			sort.Strings(recordList)
			Expect(strings.Join(recordList, ",")).Should(Equal("rid_1,rid_2,rid_3,rid_4"))
		})
	})

	Context("delete plugin one data", func() {
		It("should be succeed", func() {
			pluginARecorder := sqlite.PluginRecorder(pluginASpec)
			Expect(pluginARecorder.SaveRecord(context.TODO(), "group_1", "rid_5", map[string]string{"key": "plugin_a"})).Should(BeNil())

			data := map[string]string{}
			Expect(pluginARecorder.GetRecord(context.TODO(), "rid_5", &data)).Should(BeNil())
			Expect(data["key"]).Should(Equal("plugin_a"))
			Expect(pluginARecorder.DeleteRecord(context.TODO(), "rid_5")).Should(BeNil())

			Expect(pluginARecorder.GetRecord(context.TODO(), "rid_5", &data)).Should(Equal(types.ErrNotFound))
		})
	})
})

func InitRootEntry() *types.Metadata {
	acc := types.Access{
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
	root.ID = -1
	root.ParentID = root.ID
	return root
}

func buildNewSqliteMetaStore(dbName string) *sqliteMetaStore {
	result, err := newSqliteMetaStore(config.Meta{
		Type: SqliteMeta,
		Path: path.Join(workdir, dbName),
	})
	Expect(err).Should(BeNil())
	return result
}
