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

var (
	fileChunk1 []byte
	fileChunk2 []byte
	fileChunk3 []byte
	fileChunk4 []byte
)

const fileChunkSize = 1 << 26

func resetFileChunk() {
	fileChunk1 = make([]byte, fileChunkSize)
	fileChunk2 = make([]byte, fileChunkSize)
	fileChunk3 = make([]byte, fileChunkSize)
	fileChunk4 = make([]byte, fileChunkSize)
	copy(fileChunk1, []byte("testdata-1"))
	copy(fileChunk2, []byte("          "))
	copy(fileChunk3, []byte("testdata-3"))
	copy(fileChunk4, []byte(""))
}

func newMockFileStorage() storage.Storage {
	s, _ := storage.NewStorage(storage.MemoryStorage, storage.MemoryStorage, config.Storage{})
	return s
}

func newMockFileEntry(name string, inode int64) Entry {
	meta := types.NewMetadata(name, types.RawKind)
	meta.Size = fileChunkSize * 4
	meta.ID = inode
	meta.Storage = storage.MemoryStorage
	return BuildEntry(&types.Object{Metadata: meta}, objStore)
}

var _ = Describe("TestFileIO", func() {
	var (
		name        = "test-file-key"
		inode int64 = 1024
	)
	BeforeEach(func() {
		f, err := entryManager.Open(context.Background(), newMockFileEntry(name, inode), Attr{Read: true, Write: true})
		Expect(err).Should(BeNil())
		resetFileChunk()
		var off int64

		_, err = f.WriteAt(context.TODO(), fileChunk1, off)
		Expect(err).Should(BeNil())
		off += int64(len(fileChunk1))
		_, err = f.WriteAt(context.TODO(), fileChunk2, off)
		Expect(err).Should(BeNil())
		off += int64(len(fileChunk2))
		_, err = f.WriteAt(context.TODO(), fileChunk3, off)
		Expect(err).Should(BeNil())
		off += int64(len(fileChunk3))
		_, err = f.WriteAt(context.TODO(), fileChunk4, off)
		Expect(err).Should(BeNil())
	})

	Describe("test file open", func() {
		Context("open a file", func() {
			It("should be ok", func() {
				f, err := entryManager.Open(context.Background(), newMockFileEntry(name, inode), Attr{Read: true})
				Expect(err).Should(BeNil())
				Expect(f.Close(context.Background())).Should(BeNil())
			})
		})
	})

	Describe("test file read", func() {
		var (
			f   File
			err error
		)
		BeforeEach(func() {
			f, err = entryManager.Open(context.Background(), newMockFileEntry(name, inode), Attr{Read: true})
			Expect(err).Should(BeNil())
		})
		Context("read file succeed", func() {
			It("should be ok", func() {
				buf := make([]byte, fileChunkSize)
				var off int64
				for i := 0; i < 4; i++ {
					n, err := f.ReadAt(context.Background(), buf, off)
					Expect(n).Should(Equal(int64(fileChunkSize)))
					Expect(err).Should(BeNil())
					switch i {
					case 0:
						Expect(buf[:10]).Should(Equal(fileChunk1[:10]))
					case 1:
						Expect(buf[:10]).Should(Equal(fileChunk2[:10]))
					case 2:
						Expect(buf[:10]).Should(Equal(fileChunk3[:10]))
					case 3:
						Expect(buf[:10]).Should(Equal(fileChunk4[:10]))
					}
					off += int64(n)
				}
			})
		})
		Context("read file failed", func() {
			It("should be no perm", func() {
				buf := make([]byte, 1024)
				f, err = entryManager.Open(context.Background(), newMockFileEntry(name, inode), Attr{Write: true})
				Expect(err).Should(BeNil())
				_, err = f.ReadAt(context.Background(), buf, 0)
				Expect(err).ShouldNot(BeNil())
			})
		})
	})

	Describe("test file write", func() {
		var (
			data = []byte("testdata-3")
			f    File
			err  error
		)
		BeforeEach(func() {
			f, err = entryManager.Open(context.Background(), newMockFileEntry(name, inode), Attr{Write: true})
			Expect(err).Should(BeNil())
		})
		AfterEach(func() {
			Expect(f.Close(context.Background())).Should(BeNil())
		})
		Context("write new content to file", func() {
			It("should be ok", func() {
				_, err = f.WriteAt(context.Background(), data, 0)
				Expect(err).Should(BeNil())
				Expect(f.Fsync(context.TODO())).Should(BeNil())
				Expect(f.Close(context.Background())).Should(BeNil())
				time.Sleep(time.Second)

				Context("read file content", func() {
					f, err = entryManager.Open(context.Background(), newMockFileEntry(name, inode), Attr{Read: true})
					Expect(err).Should(BeNil())

					buf := make([]byte, 10)
					n, err := f.ReadAt(context.Background(), buf, 0)
					Expect(err).Should(BeNil())
					Expect(buf[:n]).Should(Equal([]byte("testdata-3")))
				})
			})
		})
	})

	Describe("test create new file", func() {
		var (
			data = []byte("testdata-2")
			f    File
			err  error
		)

		Context("create and write a new file", func() {
			It("should be ok", func() {
				f, err = entryManager.Open(context.Background(), newMockFileEntry("test-create-new-file", inode+1), Attr{Write: true, Create: true})
				Expect(err).Should(BeNil())
				_, err = f.WriteAt(context.Background(), data, 0)
				Expect(err).Should(BeNil())
				Expect(f.Close(context.Background())).Should(BeNil())
				time.Sleep(time.Second)
				Context("read new file", func() {
					f, err = entryManager.Open(context.Background(), newMockFileEntry("test-create-new-file", inode+1), Attr{Read: true})
					Expect(err).Should(BeNil())
					buf := make([]byte, 10)
					n, err := f.ReadAt(context.Background(), buf, 0)
					Expect(err).Should(BeNil())
					Expect(buf[:n]).Should(Equal(data[:10]))
				})
			})
		})
	})
})