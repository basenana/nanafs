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
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("TestRootEntryInit", func() {
	emptyObjectStore, err := storage.NewMetaStorage(storage.MemoryStorage, config.Meta{})
	Expect(err).Should(BeNil())

	Context("query root entry first time", func() {
		var mgr Manage
		It("init manager should be succeed", func() {
			mgr = NewManager(emptyObjectStore, config.Config{Owner: &config.FsOwner{}})
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

		var mgr Manage
		It("init manager should be succeed", func() {
			mgr = NewManager(emptyObjectStore, config.Config{Owner: &config.FsOwner{}})
		})
		It("feat root should be succeed", func() {
			r, err := mgr.Root(context.TODO())
			Expect(err).Should(BeNil())
			Expect(time.Since(r.Object().CreatedAt) > time.Second).Should(BeTrue())

		})
	})
})

var _ = Describe("TestEntryManage", func() {
	var grp1 Entry
	Context("create group entry", func() {
		It("create should be succeed", func() {
			var err error
			grp1, err = entryManager.CreateEntry(context.TODO(), root, EntryAttr{
				Name: "test_create_grp1",
				Kind: types.GroupKind,
				Access: types.Access{
					Permissions: []types.Permission{
						types.PermOwnerRead,
						types.PermOwnerWrite,
						types.PermOwnerExec,
						types.PermGroupRead,
						types.PermGroupWrite,
						types.PermOthersRead,
					},
				},
			})
			Expect(err).Should(BeNil())
		})
		It("query should be succeed", func() {
			_, err := root.Group().FindEntry(context.TODO(), "test_create_grp1")
			Expect(err).Should(BeNil())
		})
	})
	Context("create file entry", func() {
		It("create should be succeed", func() {
			_, err := entryManager.CreateEntry(context.TODO(), grp1, EntryAttr{
				Name: "test_create_file1",
				Kind: types.GroupKind,
				Access: types.Access{
					Permissions: []types.Permission{
						types.PermOwnerRead,
						types.PermOwnerWrite,
						types.PermOwnerExec,
						types.PermGroupRead,
						types.PermGroupWrite,
						types.PermOthersRead,
					},
				},
			})
			Expect(err).Should(BeNil())

			_, err = grp1.Group().FindEntry(context.TODO(), "test_create_file1")
			Expect(err).Should(BeNil())
		})
	})
	Context("delete file entry", func() {
		It("delete should be succeed", func() {
			file1, err := grp1.Group().FindEntry(context.TODO(), "test_create_file1")
			Expect(err).Should(BeNil())
			err = entryManager.DestroyEntry(context.TODO(), grp1, file1)
			Expect(err).Should(BeNil())

			_, err = grp1.Group().FindEntry(context.TODO(), "test_create_file1")
			Expect(err).Should(Equal(types.ErrNotFound))
		})
	})
	Context("delete group entry", func() {
		It("delete should be succeed", func() {
			var err error
			grp1, err = root.Group().FindEntry(context.TODO(), "test_create_grp1")
			Expect(err).Should(BeNil())
			err = entryManager.DestroyEntry(context.TODO(), root, grp1)
			Expect(err).Should(BeNil())
		})
		It("query should be failed", func() {
			_, err := root.Group().FindEntry(context.TODO(), "test_create_grp1")
			Expect(err).Should(Equal(types.ErrNotFound))
		})
	})
})

var _ = Describe("TestMirrorGroupEntryManage", func() {
	Context("create group entry test_mirror_grp1", func() {
		It("should be succeed", func() {
		})
	})
	Context("create mirror entry mirror_grp1", func() {
		It("should be succeed", func() {
		})
	})
	Context("create file entry in mirror_grp1", func() {
		It("should be succeed", func() {
		})
	})
	Context("delete mirror entry mirror_grp1", func() {
		It("should be succeed", func() {
		})
	})
	Context("delete group entry test_mirror_grp1", func() {
		It("should be succeed", func() {
		})
	})
	Context("create group entry test_mirror_grp2", func() {
		It("should be succeed", func() {
		})
	})
	Context("create mirror entry mirror_grp2", func() {
		It("should be succeed", func() {
		})
	})
	Context("create file entry in test_mirror_grp2", func() {
		It("should be succeed", func() {
		})
	})
	Context("delete group entry test_mirror_grp2", func() {
		It("should be succeed", func() {
		})
	})
	Context("delete mirror entry mirror_grp2", func() {
		It("should be succeed", func() {
		})
	})
})

var _ = Describe("TestMirrorFileEntryManage", func() {
	Context("create file entry test_mirror_file1", func() {
		It("should be succeed", func() {
		})
	})
	Context("create mirror file entry mirror_file1", func() {
		It("should be succeed", func() {
		})
	})
	Context("edit file content to mirror_file1", func() {
		It("should be succeed", func() {
			// TODO
		})
	})
	Context("delete mirror file entry mirror_file1", func() {
		It("should be succeed", func() {
		})
	})
	Context("delete file entry test_mirror_file1", func() {
		It("should be succeed", func() {
		})
	})

	Context("create file entry test_mirror_file2", func() {
		It("should be succeed", func() {
		})
	})
	Context("create mirror file entry mirror_file2", func() {
		It("should be succeed", func() {
		})
	})
	Context("edit file content to test_mirror_file2", func() {
		It("should be succeed", func() {
			// TODO
		})
	})
	Context("delete file entry test_mirror_file2", func() {
		It("should be succeed", func() {
		})
	})
	Context("delete mirror file entry mirror_file2", func() {
		It("should be succeed", func() {
		})
	})
})
