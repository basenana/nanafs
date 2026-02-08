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
	"io/fs"
	"os"
	"syscall"
	"time"

	"github.com/basenana/nanafs/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Utils", func() {
	Describe("mode2EntryAttr", func() {
		It("should convert file mode to entry attr", func() {
			attr := mode2EntryAttr(os.FileMode(0644))
			Expect(string(attr.Kind)).To(Equal(string(types.RawKind)))
			Expect(attr.Access).ToNot(BeNil())
		})

		It("should convert directory mode to entry attr", func() {
			attr := mode2EntryAttr(os.FileMode(syscall.S_IFDIR) | 0755)
			Expect(string(attr.Kind)).To(Equal(string(types.GroupKind)))
		})
	})

	Describe("flag2EntryOpenAttr", func() {
		It("should set read flag by default", func() {
			attr := flag2EntryOpenAttr(os.O_RDONLY)
			Expect(attr.Read).To(BeTrue())
			Expect(attr.Write).To(BeFalse())
		})

		It("should set create flag when O_CREATE is set", func() {
			attr := flag2EntryOpenAttr(os.O_CREATE | os.O_RDWR)
			Expect(attr.Create).To(BeTrue())
			Expect(attr.Write).To(BeTrue())
		})

		It("should set truncate flag when O_TRUNC is set", func() {
			attr := flag2EntryOpenAttr(os.O_TRUNC | os.O_WRONLY)
			Expect(attr.Trunc).To(BeTrue())
			Expect(attr.Write).To(BeTrue())
		})
	})

	Describe("modeFromFileKind", func() {
		It("should return S_IFREG for RawKind", func() {
			Expect(modeFromFileKind(types.RawKind)).To(Equal(uint32(syscall.S_IFREG)))
		})

		It("should return S_IFDIR for GroupKind", func() {
			Expect(modeFromFileKind(types.GroupKind)).To(Equal(uint32(syscall.S_IFDIR)))
		})

		It("should return S_IFDIR for SmartGroupKind", func() {
			Expect(modeFromFileKind(types.SmartGroupKind)).To(Equal(uint32(syscall.S_IFDIR)))
		})

		It("should return S_IFLNK for SymLinkKind", func() {
			Expect(modeFromFileKind(types.SymLinkKind)).To(Equal(uint32(syscall.S_IFLNK)))
		})
	})

	Describe("fileKindFromMode", func() {
		It("should return RawKind for S_IFREG", func() {
			mode := fileKindFromMode(syscall.S_IFREG)
			Expect(string(mode)).To(Equal(string(types.RawKind)))
		})

		It("should return GroupKind for S_IFDIR", func() {
			mode := fileKindFromMode(syscall.S_IFDIR)
			Expect(string(mode)).To(Equal(string(types.GroupKind)))
		})

		It("should return SymLinkKind for S_IFLNK", func() {
			mode := fileKindFromMode(syscall.S_IFLNK)
			Expect(string(mode)).To(Equal(string(types.SymLinkKind)))
		})
	})

	Describe("error2FsError", func() {
		It("should convert ErrNotFound to fs.ErrNotExist", func() {
			Expect(error2FsError(types.ErrNotFound)).To(Equal(fs.ErrNotExist))
		})

		It("should convert ErrIsExist to fs.ErrExist", func() {
			Expect(error2FsError(types.ErrIsExist)).To(Equal(fs.ErrExist))
		})

		It("should convert ErrNotEmpty to fs.ErrInvalid", func() {
			Expect(error2FsError(types.ErrNotEmpty)).To(Equal(fs.ErrInvalid))
		})

		It("should convert ErrNoAccess to fs.ErrPermission", func() {
			Expect(error2FsError(types.ErrNoAccess)).To(Equal(fs.ErrPermission))
		})

		It("should convert ErrNoPerm to fs.ErrPermission", func() {
			Expect(error2FsError(types.ErrNoPerm)).To(Equal(fs.ErrPermission))
		})

		It("should return nil for nil error", func() {
			Expect(error2FsError(nil)).To(BeNil())
		})

		It("should return original error for unknown errors", func() {
			customErr := &os.SyscallError{}
			Expect(error2FsError(customErr)).To(Equal(customErr))
		})
	})

	Describe("splitPath", func() {
		It("should split path by separator", func() {
			parts := splitPath("/a/b/c")
			Expect(parts).To(Equal([]string{"", "a", "b", "c"}))
		})

		It("should handle single path", func() {
			parts := splitPath("/file")
			Expect(parts).To(Equal([]string{"", "file"}))
		})
	})

	Describe("Stat(entry)", func() {
		It("should create Info from Entry for file", func() {
			entry := &types.Entry{
				Name:       "test.txt",
				Size:       100,
				Kind:       types.RawKind,
				ModifiedAt: time.Now(),
				IsGroup:    false,
				Access:     types.Access{UID: 0, GID: 0},
			}
			info := Stat(entry)
			Expect(info.Name()).To(Equal("test.txt"))
			Expect(info.Size()).To(Equal(int64(100)))
			Expect(info.IsDir()).To(BeFalse())
		})

		It("should create Info from Entry for directory", func() {
			entry := &types.Entry{
				Name:       "mydir",
				Size:       0,
				Kind:       types.GroupKind,
				ModifiedAt: time.Now(),
				IsGroup:    true,
				Access:     types.Access{UID: 0, GID: 0},
			}
			info := Stat(entry)
			Expect(info.Name()).To(Equal("mydir"))
			Expect(info.IsDir()).To(BeTrue())
		})
	})
})
