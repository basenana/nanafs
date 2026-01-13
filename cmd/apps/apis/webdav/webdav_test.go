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

package webdav

import (
	"context"
	"io/fs"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("FsOperator", func() {
	var operator FsOperator
	var ctx context.Context
	var uniqueID string

	BeforeEach(func() {
		operator = FsOperator{
			fs:     testFs,
			root:   nil,
			cfg:    config.Webdav{},
			logger: testLogger,
		}
		ctx = withUserContext(context.Background(), 0, 0)
		uniqueID = strconv.FormatInt(time.Now().UnixNano(), 36)

		root, err := testFs.Root(ctx)
		Expect(err).Should(BeNil())
		operator.root = root
	})

	Describe("Mkdir", func() {
		It("should create a single directory", func() {
			err := operator.Mkdir(ctx, "/testdir-"+uniqueID, os.FileMode(syscall.S_IFDIR)|0755)
			Expect(err).Should(BeNil())

			_, entry, err := testFs.GetEntryByPath(ctx, "/testdir-"+uniqueID)
			Expect(err).Should(BeNil())
			Expect(string(entry.Kind)).To(Equal(string(types.GroupKind)))
			Expect(entry.Name).To(Equal("testdir-" + uniqueID))
		})

		It("should create nested directories", func() {
			err := operator.Mkdir(ctx, "/nested-"+uniqueID+"/a/b/c", os.FileMode(syscall.S_IFDIR)|0755)
			Expect(err).Should(BeNil())

			_, entry, err := testFs.GetEntryByPath(ctx, "/nested-"+uniqueID+"/a/b/c")
			Expect(err).Should(BeNil())
			Expect(string(entry.Kind)).To(Equal(string(types.GroupKind)))
		})
	})

	Describe("OpenFile", func() {
		BeforeEach(func() {
			_, err := testFs.CreateEntry(ctx, "/", types.EntryAttr{
				Name:   "testdir-" + uniqueID,
				Kind:   types.GroupKind,
				Access: &types.Access{},
			})
			Expect(err).Should(BeNil())
		})

		It("should open existing file", func() {
			_, err := testFs.CreateEntry(ctx, "/testdir-"+uniqueID, types.EntryAttr{
				Name:   "existing.txt",
				Kind:   types.RawKind,
				Access: &types.Access{},
			})
			Expect(err).Should(BeNil())

			f, err := operator.OpenFile(ctx, "/testdir-"+uniqueID+"/existing.txt", os.O_RDONLY, 0644)
			Expect(err).Should(BeNil())
			defer f.Close()

			Expect(f).ToNot(BeNil())
		})

		It("should create new file with O_CREATE flag", func() {
			f, err := operator.OpenFile(ctx, "/testdir-"+uniqueID+"/newfile.txt", os.O_CREATE|os.O_WRONLY, 0644)
			Expect(err).Should(BeNil())
			defer f.Close()

			_, entry, err := testFs.GetEntryByPath(ctx, "/testdir-"+uniqueID+"/newfile.txt")
			Expect(err).Should(BeNil())
			Expect(entry).ToNot(BeNil())
			Expect(entry.Name).To(Equal("newfile.txt"))
		})

		It("should error when file does not exist without O_CREATE", func() {
			_, err := operator.OpenFile(ctx, "/testdir-"+uniqueID+"/nonexistent.txt", os.O_RDONLY, 0644)
			Expect(err).To(Equal(fs.ErrNotExist))
		})

		It("should error when user info is missing", func() {
			ctxNoUser := context.Background()
			_, err := operator.OpenFile(ctxNoUser, "/testdir-"+uniqueID+"/file.txt", os.O_RDONLY, 0644)
			Expect(err).To(Equal(fs.ErrPermission))
		})
	})

	Describe("RemoveAll", func() {
		BeforeEach(func() {
			_, err := testFs.CreateEntry(ctx, "/", types.EntryAttr{
				Name:   "testdir-" + uniqueID,
				Kind:   types.GroupKind,
				Access: &types.Access{},
			})
			Expect(err).Should(BeNil())
		})

		It("should remove a file", func() {
			_, err := testFs.CreateEntry(ctx, "/testdir-"+uniqueID, types.EntryAttr{
				Name:   "deleteme.txt",
				Kind:   types.RawKind,
				Access: &types.Access{},
			})
			Expect(err).Should(BeNil())

			err = operator.RemoveAll(ctx, "/testdir-"+uniqueID+"/deleteme.txt")
			Expect(err).Should(BeNil())

			_, _, err = testFs.GetEntryByPath(ctx, "/testdir-"+uniqueID+"/deleteme.txt")
			Expect(err).To(Equal(types.ErrNotFound))
		})

		It("should remove empty directory", func() {
			_, err := testFs.CreateEntry(ctx, "/testdir-"+uniqueID, types.EntryAttr{
				Name:   "subdir",
				Kind:   types.GroupKind,
				Access: &types.Access{},
			})
			Expect(err).Should(BeNil())

			err = operator.RemoveAll(ctx, "/testdir-"+uniqueID+"/subdir")
			Expect(err).Should(BeNil())
		})

		It("should error when removing root entry", func() {
			err := operator.RemoveAll(ctx, "/")
			Expect(err).To(Equal(fs.ErrPermission))
		})

		It("should error when path does not exist", func() {
			err := operator.RemoveAll(ctx, "/nonexistent-"+uniqueID)
			Expect(err).To(Equal(fs.ErrNotExist))
		})
	})

	Describe("Rename", func() {
		BeforeEach(func() {
			_, err := testFs.CreateEntry(ctx, "/", types.EntryAttr{
				Name:   "testdir-" + uniqueID,
				Kind:   types.GroupKind,
				Access: &types.Access{},
			})
			Expect(err).Should(BeNil())

			_, err = testFs.CreateEntry(ctx, "/testdir-"+uniqueID, types.EntryAttr{
				Name:   "oldname.txt",
				Kind:   types.RawKind,
				Access: &types.Access{},
			})
			Expect(err).Should(BeNil())
		})

		It("should rename a file", func() {
			err := operator.Rename(ctx, "/testdir-"+uniqueID+"/oldname.txt", "/testdir-"+uniqueID+"/newname.txt")
			Expect(err).Should(BeNil())

			_, entry, err := testFs.GetEntryByPath(ctx, "/testdir-"+uniqueID+"/newname.txt")
			Expect(err).Should(BeNil())
			Expect(entry.Name).To(Equal("newname.txt"))
		})

		It("should error when source does not exist", func() {
			err := operator.Rename(ctx, "/testdir-"+uniqueID+"/nonexistent.txt", "/testdir-"+uniqueID+"/newname.txt")
			Expect(err).To(Equal(fs.ErrNotExist))
		})
	})

	Describe("Stat", func() {
		BeforeEach(func() {
			_, err := testFs.CreateEntry(ctx, "/", types.EntryAttr{
				Name:   "testdir-" + uniqueID,
				Kind:   types.GroupKind,
				Access: &types.Access{},
			})
			Expect(err).Should(BeNil())

			_, err = testFs.CreateEntry(ctx, "/testdir-"+uniqueID, types.EntryAttr{
				Name:   "testfile.txt",
				Kind:   types.RawKind,
				Access: &types.Access{},
			})
			Expect(err).Should(BeNil())
		})

		It("should return FileInfo for a file", func() {
			info, err := operator.Stat(ctx, "/testdir-"+uniqueID+"/testfile.txt")
			Expect(err).Should(BeNil())
			Expect(info.Name()).To(Equal("testfile.txt"))
			Expect(info.IsDir()).To(BeFalse())
		})

		It("should return FileInfo for a directory", func() {
			info, err := operator.Stat(ctx, "/testdir-"+uniqueID)
			Expect(err).Should(BeNil())
			Expect(info.Name()).To(Equal("testdir-" + uniqueID))
			Expect(info.IsDir()).To(BeTrue())
		})

		It("should error when path does not exist", func() {
			_, err := operator.Stat(ctx, "/nonexistent-"+uniqueID)
			Expect(err).To(Equal(fs.ErrNotExist))
		})
	})
})
