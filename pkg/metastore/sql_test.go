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
	"fmt"
	"path"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/metastore/db"
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

			err = sqlite.CreateEntry(context.TODO(), namespace, rootEn.ID, en)
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

			err = sqlite.CreateEntry(context.TODO(), namespace, rootEn.ID, en)
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
				en, err := types.InitNewEntry(group1, types.EntryAttr{Name: fmt.Sprintf("test-file-en-%d", i), Kind: types.RawKind})
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
		var (
			targetEn *types.Entry
			enName   = "test-mv-src-raw-obj-1"
		)
		It("create new file be succeed", func() {
			targetEn, err = types.InitNewEntry(group1, types.EntryAttr{Name: enName, Kind: types.RawKind})
			Expect(err).Should(BeNil())
			Expect(sqlite.CreateEntry(context.TODO(), namespace, group1.ID, targetEn)).Should(BeNil())
		})

		It("should be succeed", func() {
			err = sqlite.ChangeEntryParent(context.TODO(), namespace, targetEn.ID, group1.ID, group2.ID, enName, enName, types.ChangeParentAttr{})
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
			err := sqlite.MirrorEntry(context.TODO(), namespace, srcEN.ID, "test-mirror-dst-file-2", rootEn.ID, types.EntryAttr{})
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

var _ = Describe("TestListNamespaceGroups", func() {
	var sqlite = buildNewSqliteMetaStore("test_namespace_groups.db")
	rootEn := InitRootEntry()
	Expect(sqlite.CreateEntry(context.TODO(), namespace, 0, rootEn)).Should(BeNil())

	group1, err := types.InitNewEntry(rootEn, types.EntryAttr{
		Name: "ns-group-1",
		Kind: types.GroupKind,
	})
	Expect(err).Should(BeNil())
	Expect(sqlite.CreateEntry(context.TODO(), namespace, rootEn.ID, group1)).Should(BeNil())

	group2, err := types.InitNewEntry(rootEn, types.EntryAttr{
		Name: "ns-group-2",
		Kind: types.GroupKind,
	})
	Expect(err).Should(BeNil())
	Expect(sqlite.CreateEntry(context.TODO(), namespace, rootEn.ID, group2)).Should(BeNil())

	subGroup1, err := types.InitNewEntry(group1, types.EntryAttr{
		Name: "ns-subgroup-1",
		Kind: types.GroupKind,
	})
	Expect(err).Should(BeNil())
	Expect(sqlite.CreateEntry(context.TODO(), namespace, group1.ID, subGroup1)).Should(BeNil())

	file1, err := types.InitNewEntry(rootEn, types.EntryAttr{
		Name: "ns-file-1",
		Kind: types.RawKind,
	})
	Expect(err).Should(BeNil())
	Expect(sqlite.CreateEntry(context.TODO(), namespace, rootEn.ID, file1)).Should(BeNil())

	Context("list all namespace groups", func() {
		It("should return all group children in namespace", func() {
			groups, err := sqlite.ListNamespaceGroups(context.TODO(), namespace)
			Expect(err).Should(BeNil())
			Expect(groups).Should(HaveLen(3))

			groupIDs := []int64{group1.ID, group2.ID, subGroup1.ID}
			for _, g := range groups {
				Expect(g.IsGroup).Should(BeTrue())
				Expect(containsInt64(groupIDs, g.ChildID)).Should(BeTrue())
			}
		})

		It("should return empty when no groups", func() {
			// Query a namespace that has no groups
			groups, err := sqlite.ListNamespaceGroups(context.TODO(), "non-existent-namespace")
			Expect(err).Should(BeNil())
			Expect(groups).Should(BeEmpty())
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

func containsInt64(slice []int64, item int64) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

var _ = Describe("TestSysConfig", func() {
	var sqlite = buildNewSqliteMetaStore("test_config.db")
	// init root
	rootEn := InitRootEntry()
	Expect(sqlite.CreateEntry(context.TODO(), namespace, 0, rootEn)).Should(BeNil())

	Context("set and get config value", func() {
		It("should succeed", func() {
			err := sqlite.SetConfigValue(context.TODO(), namespace, "test-group", "key1", "value1")
			Expect(err).Should(BeNil())

			val, err := sqlite.GetConfigValue(context.TODO(), namespace, "test-group", "key1")
			Expect(err).Should(BeNil())
			Expect(val).Should(Equal("value1"))
		})

		It("should update existing config", func() {
			err := sqlite.SetConfigValue(context.TODO(), namespace, "test-group", "key1", "value2")
			Expect(err).Should(BeNil())

			val, err := sqlite.GetConfigValue(context.TODO(), namespace, "test-group", "key1")
			Expect(err).Should(BeNil())
			Expect(val).Should(Equal("value2"))
		})
	})

	Context("list config values by group", func() {
		It("should list all configs in the group", func() {
			// Set multiple configs in the same group
			_ = sqlite.SetConfigValue(context.TODO(), namespace, "list-group", "name1", "val1")
			_ = sqlite.SetConfigValue(context.TODO(), namespace, "list-group", "name2", "val2")
			_ = sqlite.SetConfigValue(context.TODO(), namespace, "list-group", "name3", "val3")

			configs, err := sqlite.ListConfigValues(context.TODO(), namespace, "list-group")
			Expect(err).Should(BeNil())
			Expect(len(configs)).Should(Equal(3))

			// Verify all configs are returned
			names := make(map[string]bool)
			for _, cfg := range configs {
				names[cfg.Name] = true
				Expect(cfg.Group).Should(Equal("list-group"))
			}
			Expect(names["name1"]).Should(BeTrue())
			Expect(names["name2"]).Should(BeTrue())
			Expect(names["name3"]).Should(BeTrue())
		})

		It("should return empty list for non-existent group", func() {
			configs, err := sqlite.ListConfigValues(context.TODO(), namespace, "non-existent")
			Expect(err).Should(BeNil())
			Expect(len(configs)).Should(Equal(0))
		})
	})

	Context("delete config value", func() {
		It("should delete existing config", func() {
			_ = sqlite.SetConfigValue(context.TODO(), namespace, "delete-group", "to-delete", "delete-value")

			err := sqlite.DeleteConfigValue(context.TODO(), namespace, "delete-group", "to-delete")
			Expect(err).Should(BeNil())

			_, err = sqlite.GetConfigValue(context.TODO(), namespace, "delete-group", "to-delete")
			Expect(err).ShouldNot(BeNil())
		})

		It("should not return error when deleting non-existent config (GORM behavior)", func() {
			err := sqlite.DeleteConfigValue(context.TODO(), namespace, "delete-group", "non-existent")
			// GORM doesn't return error for non-existent records on delete
			Expect(err).Should(BeNil())
		})
	})
})

var _ = Describe("TestScanOrphanEntries", func() {
	var sqlite *sqlMetaStore

	BeforeEach(func() {
		sqlite = buildNewSqliteMetaStore("test_orphan.db")
		// init root
		rootEn := InitRootEntry()
		Expect(sqlite.CreateEntry(context.TODO(), namespace, 0, rootEn)).Should(BeNil())
	})

	AfterEach(func() {
		// Clean up: delete all entries created during test
		sqlite.WithContext(context.TODO()).Where("namespace = ?", namespace).Delete(&db.Entry{})
	})

	Context("scan orphan entries with ref_count = 0", func() {
		It("should find entries with zero ref_count", func() {
			// Create an entry directly with ref_count = 0
			orphanEn, err := types.InitNewEntry(InitRootEntry(), types.EntryAttr{
				Name: "orphan-entry-1",
				Kind: types.RawKind,
			})
			Expect(err).Should(BeNil())

			// Manually set ref_count to 0 and create the entry
			orphanEn.RefCount = 0
			err = sqlite.CreateEntry(context.TODO(), namespace, 1, orphanEn)
			Expect(err).Should(BeNil())

			// Scan for orphan entries
			cutoffTime := time.Now()
			orphans, err := sqlite.ScanOrphanEntries(context.TODO(), cutoffTime)
			Expect(err).Should(BeNil())

			// Should find the orphaned entry
			found := false
			for _, o := range orphans {
				if o.Name == "orphan-entry-1" {
					found = true
					Expect(o.RefCount).Should(Equal(0))
				}
			}
			Expect(found).Should(BeTrue())
		})

		It("should not find entries with ref_count > 0", func() {
			// Create a normal entry with positive ref_count
			normalEn, err := types.InitNewEntry(InitRootEntry(), types.EntryAttr{
				Name: "normal-entry-1",
				Kind: types.RawKind,
			})
			Expect(err).Should(BeNil())
			// RefCount is automatically set to 1 when created as child
			err = sqlite.CreateEntry(context.TODO(), namespace, 1, normalEn)
			Expect(err).Should(BeNil())

			// Scan should not find entries with positive ref_count
			cutoffTime := time.Now()
			orphans, err := sqlite.ScanOrphanEntries(context.TODO(), cutoffTime)
			Expect(err).Should(BeNil())

			for _, o := range orphans {
				Expect(o.Name).ShouldNot(Equal("normal-entry-1"))
			}
		})

		It("should return empty list when no orphan entries exist", func() {
			cutoffTime := time.Now()
			orphans, err := sqlite.ScanOrphanEntries(context.TODO(), cutoffTime)
			Expect(err).Should(BeNil())
			Expect(len(orphans)).Should(Equal(0))
		})

		It("should filter entries by changed_at time", func() {
			// Create an orphan entry
			recentOrphan, err := types.InitNewEntry(InitRootEntry(), types.EntryAttr{
				Name: "recent-orphan",
				Kind: types.RawKind,
			})
			Expect(err).Should(BeNil())
			recentOrphan.RefCount = 0
			err = sqlite.CreateEntry(context.TODO(), namespace, 1, recentOrphan)
			Expect(err).Should(BeNil())

			// Scan with cutoff time in the past should not find the entry (changed_at is now)
			pastCutoff := time.Now().Add(-time.Hour)
			orphans, err := sqlite.ScanOrphanEntries(context.TODO(), pastCutoff)
			Expect(err).Should(BeNil())

			// Entry should not be found because its changed_at (now) is after past cutoff
			for _, o := range orphans {
				Expect(o.Name).ShouldNot(Equal("recent-orphan"))
			}
		})

		It("should find multiple orphan entries", func() {
			// Create multiple orphan entries
			for i := 1; i <= 3; i++ {
				orphanEn, err := types.InitNewEntry(InitRootEntry(), types.EntryAttr{
					Name: "orphan-multi-" + string(rune('a'+i-1)),
					Kind: types.RawKind,
				})
				Expect(err).Should(BeNil())
				orphanEn.RefCount = 0
				err = sqlite.CreateEntry(context.TODO(), namespace, 1, orphanEn)
				Expect(err).Should(BeNil())
			}

			cutoffTime := time.Now()
			orphans, err := sqlite.ScanOrphanEntries(context.TODO(), cutoffTime)
			Expect(err).Should(BeNil())
			Expect(len(orphans)).Should(Equal(3))
		})
	})
})

var _ = Describe("TestWorkflowQueue", func() {
	var sqlite *sqlMetaStore

	BeforeEach(func() {
		dbFile := fmt.Sprintf("test_queue_%d_%d.db", GinkgoParallelProcess(), time.Now().UnixNano())
		sqlite = buildNewSqliteMetaStore(dbFile)
		sqlite.WithContext(context.TODO()).Where("namespace = ?", namespace).Delete(&db.Entry{})
		rootEn := InitRootEntry()
		Expect(sqlite.CreateEntry(context.TODO(), namespace, 0, rootEn)).Should(BeNil())
	})

	AfterEach(func() {
		sqlite.WithContext(context.TODO()).Where("1=1").Delete(&db.Workflow{})
		sqlite.WithContext(context.TODO()).Where("1=1").Delete(&db.WorkflowJob{})
		sqlite.WithContext(context.TODO()).Where("1=1").Delete(&db.Entry{})
	})

	It("GetPendingNamespaces returns namespaces with initializing jobs", func() {
		wf := &types.Workflow{Id: "wf-1", QueueName: "default"}
		Expect(sqlite.SaveWorkflow(context.TODO(), namespace, wf)).Should(BeNil())

		job := &types.WorkflowJob{Id: "job-1", Namespace: namespace, Workflow: "wf-1", Status: "initializing", QueueName: "default"}
		Expect(sqlite.SaveWorkflowJob(context.TODO(), namespace, job)).Should(BeNil())

		ns, err := sqlite.GetPendingNamespaces(context.TODO(), "default")
		Expect(err).Should(BeNil())
		Expect(ns).Should(ContainElement(namespace))
	})

	It("ClaimNextJob claims job and updates status to running", func() {
		wf := &types.Workflow{Id: "wf-1", QueueName: "default"}
		Expect(sqlite.SaveWorkflow(context.TODO(), namespace, wf)).Should(BeNil())

		job := &types.WorkflowJob{Id: "job-1", Namespace: namespace, Workflow: "wf-1", Status: "initializing", QueueName: "default"}
		Expect(sqlite.SaveWorkflowJob(context.TODO(), namespace, job)).Should(BeNil())

		claimed, err := sqlite.ClaimNextJob(context.TODO(), "default", namespace)
		Expect(err).Should(BeNil())
		Expect(claimed).ShouldNot(BeNil())
		Expect(claimed.Id).Should(Equal("job-1"))
		Expect(claimed.Status).Should(Equal("running"))
	})
})
