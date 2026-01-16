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
	"strconv"
	"time"

	"github.com/basenana/nanafs/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("File Wrapper", func() {
	var ctx context.Context
	var testDir *types.Entry
	var uniqueID string

	BeforeEach(func() {
		ctx = withUserContext(context.Background(), 0, 0)
		uniqueID = strconv.FormatInt(time.Now().UnixNano(), 36)
		var err error
		testDir, err = testFs.CreateEntry(ctx, "/", types.EntryAttr{
			Name:   "testdir-" + uniqueID,
			Kind:   types.GroupKind,
			Access: &types.Access{},
		})
		Expect(err).Should(BeNil())
	})

	Describe("File.Stat", func() {
		It("should return FileInfo for a file", func() {
			file, err := testFs.CreateEntry(ctx, "/testdir-"+uniqueID, types.EntryAttr{
				Name:   "testfile.txt",
				Kind:   types.RawKind,
				Access: &types.Access{},
			})
			Expect(err).Should(BeNil())

			// Use Stat utility function which properly creates Info with name
			info := Stat(file)
			Expect(info.Name()).To(Equal("testfile.txt"))
			Expect(info.IsDir()).To(BeFalse())
		})

		It("should return FileInfo for a directory", func() {
			// Use Stat utility function which properly creates Info with name
			info := Stat(testDir)
			Expect(info.Name()).To(Equal("testdir-" + uniqueID))
			Expect(info.IsDir()).To(BeTrue())
		})
	})

	Describe("File.Readdir", func() {
		It("should read directory contents", func() {
			_, err := testFs.CreateEntry(ctx, "/testdir-"+uniqueID, types.EntryAttr{
				Name:   "file1.txt",
				Kind:   types.RawKind,
				Access: &types.Access{},
			})
			Expect(err).Should(BeNil())

			_, err = testFs.CreateEntry(ctx, "/testdir-"+uniqueID, types.EntryAttr{
				Name:   "file2.txt",
				Kind:   types.RawKind,
				Access: &types.Access{},
			})
			Expect(err).Should(BeNil())

			f, err := openFile(ctx, testDir, testFs, types.OpenAttr{Read: true})
			Expect(err).Should(BeNil())
			defer f.Close()

			infos, err := f.Readdir(-1)
			Expect(err).Should(BeNil())
			Expect(len(infos)).To(Equal(2))
		})

		It("should return empty slice for empty directory", func() {
			f, err := openFile(ctx, testDir, testFs, types.OpenAttr{Read: true})
			Expect(err).Should(BeNil())
			defer f.Close()

			infos, err := f.Readdir(-1)
			Expect(err).Should(BeNil())
			Expect(len(infos)).To(Equal(0))
		})
	})

	Describe("File.Close", func() {
		It("should close file without error", func() {
			file, err := testFs.CreateEntry(ctx, "/testdir-"+uniqueID, types.EntryAttr{
				Name:   "closetest.txt",
				Kind:   types.RawKind,
				Access: &types.Access{},
			})
			Expect(err).Should(BeNil())

			f, err := openFile(ctx, file, testFs, types.OpenAttr{Read: true})
			Expect(err).Should(BeNil())

			err = f.Close()
			Expect(err).Should(BeNil())
		})

		It("should close directory without error", func() {
			f, err := openFile(ctx, testDir, testFs, types.OpenAttr{Read: true})
			Expect(err).Should(BeNil())

			err = f.Close()
			Expect(err).Should(BeNil())
		})
	})
})

var _ = Describe("Info os.FileInfo implementation", func() {
	Describe("Mode()", func() {
		It("should return correct file mode", func() {
			info := &Info{
				name:  "test",
				size:  100,
				mode:  0644,
				mTime: time.Now(),
				isDir: false,
			}
			mode := info.Mode()
			Expect(mode).To(Equal(fs.FileMode(0644)))
		})
	})

	Describe("IsDir()", func() {
		It("should return true for directory", func() {
			info := &Info{
				name:  "test",
				isDir: true,
			}
			Expect(info.IsDir()).To(BeTrue())
		})

		It("should return false for file", func() {
			info := &Info{
				name:  "test",
				isDir: false,
			}
			Expect(info.IsDir()).To(BeFalse())
		})
	})

	Describe("Sys()", func() {
		It("should return nil", func() {
			info := &Info{}
			Expect(info.Sys()).To(BeNil())
		})
	})
})
