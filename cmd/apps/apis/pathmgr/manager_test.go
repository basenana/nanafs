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

package pathmgr

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
)

var _ = Describe("TestPathSplit", func() {
	mgr, _ := New(NewMockController())
	ctx := context.TODO()
	Context("test get path", func() {
		It("get path should be succeed", func() {
			var (
				path string
				err  error
			)
			path, err = mgr.getPath(ctx, "/")
			Expect(err).Should(BeNil())
			Expect(path).Should(Equal("/"))

			path, err = mgr.getPath(ctx, "/a/b/c")
			Expect(err).Should(BeNil())
			Expect(path).Should(Equal("/a/b/c"))

			path, err = mgr.getPath(ctx, "/a//b/c")
			Expect(err).Should(BeNil())
			Expect(path).Should(Equal("/a/b/c"))

			path, err = mgr.getPath(ctx, "/a//b/c/")
			Expect(err).Should(BeNil())
			Expect(path).Should(Equal("/a/b/c"))
		})
		It("get path should be succeed", func() {
			var err error
			_, err = mgr.getPath(ctx, "/../sys")
			Expect(err).ShouldNot(BeNil())

			_, err = mgr.getPath(ctx, "/./sys")
			Expect(err).ShouldNot(BeNil())

			_, err = mgr.getPath(ctx, "/./../sys")
			Expect(err).ShouldNot(BeNil())
		})
	})
	Context("test split path", func() {
		It("should be succeed", func() {
			var paths []string
			paths = mgr.splitPath("/")
			Expect(strings.Join(paths, ",")).Should(Equal(strings.Join([]string{"/"}, ",")))

			paths = mgr.splitPath("/a")
			Expect(strings.Join(paths, ",")).Should(Equal(strings.Join([]string{"/", "/a"}, ",")))

			paths = mgr.splitPath("/a//b")
			Expect(strings.Join(paths, ",")).Should(Equal(strings.Join([]string{"/", "/a", "/a/b"}, ",")))
		})
	})
})

var _ = Describe("TestPathMgr", func() {
	ctrl := NewMockController()
	var root *types.Entry
	Context("init test file", func() {
		It("should be succeed", func() {
			var err error
			root, err = ctrl.LoadRootEntry(context.Background())
			Expect(err).Should(BeNil())
			_, err = ctrl.CreateEntry(context.Background(), root.ID, types.EntryAttr{
				Name: "file1.txt",
				Kind: types.RawKind,
			})
			Expect(err).Should(BeNil())
		})
	})

	mgr, _ := New(ctrl)
	Context("test FindEntry", func() {
		It("should be succeed", func() {
			en, err := mgr.FindEntry(context.Background(), "/file1.txt")
			Expect(err).Should(BeNil())
			Expect(en.Name).Should(Equal("file1.txt"))
		})
	})
	Context("test FindParentEntry", func() {
		It("should be succeed", func() {
			en, err := mgr.FindParentEntry(context.Background(), "/file1.txt")
			Expect(err).Should(BeNil())
			Expect(en.ID).Should(Equal(int64(dentry.RootEntryID)))
		})
	})
	Context("test Open file", func() {
		It("open existed should be succeed", func() {
			_, err := mgr.CreateFile(context.Background(), "/", types.EntryAttr{Name: "file1.txt"})
			Expect(err).Should(BeNil())
		})
		It("write new should be succeed", func() {
			en, err := mgr.CreateFile(context.Background(), "/", types.EntryAttr{Name: "file2.txt"})
			Expect(err).Should(BeNil())
			enID := en.ID

			f, err := mgr.Open(context.Background(), enID, types.OpenAttr{Write: true})
			Expect(err).Should(BeNil())

			n, err := f.WriteAt(context.Background(), []byte("abc"), 10)
			Expect(n).Should(Equal(int64(3)))
			Expect(err).Should(BeNil())
			Expect(f.Close(context.Background())).Should(BeNil())

			en, err = mgr.FindEntry(context.Background(), "/file2.txt")
			Expect(err).Should(BeNil())
			Expect(en.ID).Should(Equal(enID))
		})
	})
	Context("test CreateAll", func() {
		It("create dir2 should be succeed", func() {
			dir2, err := mgr.CreateAll(context.Background(), "/dir1/dir2", types.EntryAttr{})
			Expect(err).Should(BeNil())
			Expect(dir2.Name).Should(Equal("dir2"))

			en1, err := mgr.FindEntry(context.Background(), "/dir1")
			Expect(err).Should(BeNil())
			Expect(en1.Name).Should(Equal("dir1"))
			en2, err := mgr.FindEntry(context.Background(), "/dir1/dir2")
			Expect(err).Should(BeNil())
			Expect(en2.Name).Should(Equal("dir2"))

			Expect(dir2.ID).Should(Equal(en2.ID))
			Expect(dir2.ParentID).Should(Equal(en1.ID))
		})
		It("create dir5 should be succeed", func() {
			_, err := mgr.CreateAll(context.Background(), "/dir1/dir2/dir3/dir4/dir5", types.EntryAttr{})
			Expect(err).Should(BeNil())
		})
		It("create dir3 should be succeed", func() {
			_, err := mgr.CreateAll(context.Background(), "/dir1/dir2/dir3", types.EntryAttr{})
			Expect(err).Should(BeNil())
		})
		It("create dir2.1 should be succeed", func() {
			_, err := mgr.CreateAll(context.Background(), "/dir1/dir2.1/dir3.1/dir4.1/dir5.1", types.EntryAttr{})
			Expect(err).Should(BeNil())
		})
	})
	Context("test RemoveAll", func() {
		It("recursion is false should be succeed", func() {
			err := mgr.RemoveAll(context.Background(), "/dir1/dir2/dir3/dir4/dir5", false)
			Expect(err).Should(BeNil())

			_, err = mgr.FindEntry(context.Background(), "/dir1/dir2/dir3/dir4")
			Expect(err).Should(BeNil())
		})
		It("recursion is true should be succeed", func() {
			err := mgr.RemoveAll(context.Background(), "/dir1/dir2.1/dir3.1/dir4.1/dir5.1", true)
			Expect(err).Should(BeNil())

			_, err = mgr.FindEntry(context.Background(), "/dir1/dir2.1/dir3.1")
			Expect(err).Should(Equal(types.ErrNotFound))
		})
	})
	Context("test Rename", func() {
		It("should be succeed", func() {
			err := mgr.Rename(context.Background(), "/file1.txt", "/new-file-1.txt")
			Expect(err).Should(BeNil())

			_, err = mgr.FindEntry(context.Background(), "/file1.txt")
			Expect(err).Should(Equal(types.ErrNotFound))
		})
	})
})
