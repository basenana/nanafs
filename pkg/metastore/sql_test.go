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
	rootObj := InitRootObject()
	Expect(sqlite.SaveObjects(context.TODO(), rootObj)).Should(BeNil())

	Context("create a new file object", func() {
		It("should be succeed", func() {
			obj, err := types.InitNewObject(&rootObj.Metadata, types.ObjectAttr{
				Name: "test-new-obj-1",
				Kind: types.RawKind,
			})
			Expect(err).Should(BeNil())

			err = sqlite.SaveObjects(context.TODO(), rootObj, obj)
			Expect(err).Should(BeNil())

			fetchObj, err := sqlite.GetObject(context.TODO(), obj.ID)
			Expect(err).Should(BeNil())
			Expect(fetchObj.Name).Should(Equal("test-new-obj-1"))
		})
	})

	Context("update a exist file object", func() {
		It("should be succeed", func() {
			obj, err := types.InitNewObject(&rootObj.Metadata, types.ObjectAttr{
				Name: "test-update-obj-1",
				Kind: types.RawKind,
			})
			Expect(err).Should(BeNil())

			err = sqlite.SaveObjects(context.TODO(), rootObj, obj)
			Expect(err).Should(BeNil())

			obj.Name = "test-update-obj-2"
			err = sqlite.SaveObjects(context.TODO(), obj)
			Expect(err).Should(BeNil())

			newObj, err := sqlite.GetObject(context.TODO(), obj.ID)
			Expect(err).Should(BeNil())
			Expect(newObj.Name).Should(Equal("test-update-obj-2"))
		})
	})

	Context("delete a exist file object", func() {
		It("should be succeed", func() {
			obj, err := types.InitNewObject(&rootObj.Metadata, types.ObjectAttr{
				Name: "test-delete-obj-1",
				Kind: types.RawKind,
			})
			Expect(err).Should(BeNil())

			err = sqlite.SaveObjects(context.TODO(), rootObj, obj)
			Expect(err).Should(BeNil())

			_, err = sqlite.GetObject(context.TODO(), obj.ID)
			Expect(err).Should(BeNil())

			Expect(sqlite.DestroyObject(context.TODO(), nil, obj)).Should(BeNil())

			_, err = sqlite.GetObject(context.TODO(), obj.ID)
			Expect(err).Should(Equal(types.ErrNotFound))
		})
	})

	Context("create a new group object", func() {
		It("should be succeed", func() {
			obj, err := types.InitNewObject(&rootObj.Metadata, types.ObjectAttr{
				Name: "test-new-group-1",
				Kind: types.GroupKind,
			})
			Expect(err).Should(BeNil())

			err = sqlite.SaveObjects(context.TODO(), rootObj, obj)
			Expect(err).Should(BeNil())

			fetchObj, err := sqlite.GetObject(context.TODO(), obj.ID)
			Expect(err).Should(BeNil())
			Expect(fetchObj.Name).Should(Equal("test-new-group-1"))
			Expect(string(fetchObj.Kind)).Should(Equal(string(types.GroupKind)))
		})
	})
})

var _ = Describe("TestSqliteGroupOperation", func() {
	var sqlite = buildNewSqliteMetaStore("test_group.db")
	// init root
	rootObj := InitRootObject()
	Expect(sqlite.SaveObjects(context.TODO(), rootObj)).Should(BeNil())

	group1, err := types.InitNewObject(&rootObj.Metadata, types.ObjectAttr{
		Name: "test-new-group-1",
		Kind: types.GroupKind,
	})
	Expect(err).Should(BeNil())
	Expect(sqlite.SaveObjects(context.TODO(), rootObj, group1)).Should(BeNil())

	group2, err := types.InitNewObject(&rootObj.Metadata, types.ObjectAttr{
		Name: "test-new-group-2",
		Kind: types.GroupKind,
	})
	Expect(err).Should(BeNil())
	Expect(sqlite.SaveObjects(context.TODO(), rootObj, group2)).Should(BeNil())

	Context("list a group object all children", func() {
		It("create group file should be succeed", func() {
			for i := 0; i < 4; i++ {
				obj, err := types.InitNewObject(&group1.Metadata, types.ObjectAttr{Name: "test-file-obj-1", Kind: types.RawKind})
				Expect(err).Should(BeNil())
				Expect(sqlite.SaveObjects(context.TODO(), group1, obj)).Should(BeNil())
			}

			obj, err := types.InitNewObject(&group1.Metadata, types.ObjectAttr{Name: "test-dev-obj-1", Kind: types.BlkDevKind})
			Expect(err).Should(BeNil())
			Expect(sqlite.SaveObjects(context.TODO(), group1, obj)).Should(BeNil())
		})

		It("list new file object should be succeed", func() {
			chIt, err := sqlite.ListChildren(context.TODO(), group1)
			Expect(err).Should(BeNil())

			chList := make([]*types.Object, 0)
			for chIt.HasNext() {
				chList = append(chList, chIt.Next())
			}

			Expect(len(chList)).Should(Equal(5))
		})
	})

	Context("change a exist file object parent group", func() {
		var targetObj *types.Object
		It("get target file object should be succeed", func() {
			chList, err := sqlite.ListObjects(context.TODO(), types.Filter{ParentID: group1.ID, Kind: types.BlkDevKind})
			Expect(err).Should(BeNil())
			Expect(len(chList)).Should(Equal(1))
			targetObj = chList[0]
		})

		It("should be succeed", func() {
			err = sqlite.ChangeParent(context.TODO(), group1, group2, targetObj, types.ChangeParentOption{})
			Expect(err).Should(BeNil())

			chIt, err := sqlite.ListChildren(context.TODO(), group2)
			Expect(err).Should(BeNil())

			chList := make([]*types.Object, 0)
			for chIt.HasNext() {
				chList = append(chList, chIt.Next())
			}

			Expect(len(chList)).Should(Equal(1))
		})

		It("query target file object in old group should not found", func() {
			chList, err := sqlite.ListObjects(context.TODO(), types.Filter{ParentID: group1.ID, Kind: types.BlkDevKind})
			Expect(err).Should(BeNil())
			Expect(len(chList)).Should(Equal(0))

			chIt, err := sqlite.ListChildren(context.TODO(), group1)
			Expect(err).Should(BeNil())
			chList = make([]*types.Object, 0)
			for chIt.HasNext() {
				chList = append(chList, chIt.Next())
			}
			Expect(len(chList)).Should(Equal(4))
		})
	})

	Context("mirror a exist file object to other group", func() {
		group3, err := types.InitNewObject(&rootObj.Metadata, types.ObjectAttr{
			Name: "test-mirror-group-1",
			Kind: types.GroupKind,
		})
		Expect(err).Should(BeNil())
		Expect(sqlite.SaveObjects(context.TODO(), group2, group3)).Should(BeNil())

		group4, err := types.InitNewObject(&rootObj.Metadata, types.ObjectAttr{
			Name: "test-mirror-group-2",
			Kind: types.GroupKind,
		})
		Expect(err).Should(BeNil())
		Expect(sqlite.SaveObjects(context.TODO(), group2, group4)).Should(BeNil())

		var srcObj *types.Object
		It("create new file be succeed", func() {
			srcObj, err = types.InitNewObject(&group1.Metadata, types.ObjectAttr{Name: "test-src-raw-obj-1", Kind: types.RawKind})
			Expect(err).Should(BeNil())
			Expect(sqlite.SaveObjects(context.TODO(), group3, srcObj)).Should(BeNil())
		})

		It("create mirror object should be succeed", func() {
			newObj, err := types.InitNewObject(&rootObj.Metadata, types.ObjectAttr{Name: "test-mirror-dst-file-2", Kind: types.RawKind})
			Expect(err).Should(BeNil())
			newObj.RefID = srcObj.ID
			Expect(sqlite.MirrorObject(context.TODO(), srcObj, group4, newObj)).Should(BeNil())
		})

		It("filter mirror object should be succeed", func() {
			chList, err := sqlite.ListObjects(context.TODO(), types.Filter{RefID: srcObj.ID})
			Expect(err).Should(BeNil())
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

func InitRootObject() *types.Object {
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
	root, _ := types.InitNewObject(nil, types.ObjectAttr{Name: "root", Kind: types.GroupKind, Access: acc})
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
