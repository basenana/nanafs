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

package core

import (
	"context"
	"fmt"
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
	copy(fileChunk1, []byte("testdata-A"))
	copy(fileChunk2, []byte("          "))
	copy(fileChunk3, []byte("testdata-C"))
	copy(fileChunk4, []byte(""))
}

func newMockFileEntry(name string) *types.Entry {
	ctx := context.TODO()
	grp, err := fsCore.OpenGroup(ctx, namespace, root.ID)
	Expect(err).Should(BeNil())
	en, err := grp.FindEntry(context.TODO(), name)
	if err == nil {
		return en
	}
	en, err = fsCore.CreateEntry(ctx, namespace, root.ID, types.EntryAttr{Name: name, Kind: types.RawKind})
	if err != nil {
		panic(fmt.Errorf("init file entry failed: %s", err))
	}
	en.Size = fileChunkSize * 4
	en.Storage = storage.MemoryStorage
	err = grp.UpdateEntry(context.TODO(), en)
	if err != nil {
		panic(fmt.Errorf("update file entry failed: %s", err))
	}
	return en
}

var _ = Describe("TestFileIO", func() {
	var (
		ctx  = context.TODO()
		name = "test-file-key"
	)
	BeforeEach(func() {
		f, err := fsCore.Open(ctx, namespace, newMockFileEntry(name).ID, types.OpenAttr{Read: true, Write: true})
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
		Expect(f.Fsync(context.Background())).Should(BeNil())
	})

	Describe("test file open", func() {
		Context("open a file", func() {
			It("should be ok", func() {
				f, err := fsCore.Open(ctx, namespace, newMockFileEntry(name).ID, types.OpenAttr{Read: true})
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
			f, err = fsCore.Open(ctx, namespace, newMockFileEntry(name).ID, types.OpenAttr{Read: true})
			Expect(err).Should(BeNil())
		})
		Context("read file succeed", func() {
			It("should be ok", func() {
				buf := make([]byte, fileChunkSize)
				var off int64
				for i := 0; i < 4; i++ {
					n, err := f.ReadAt(ctx, buf, off)
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
				f, err = fsCore.Open(ctx, namespace, newMockFileEntry(name).ID, types.OpenAttr{Write: true})
				Expect(err).Should(BeNil())
				_, err = f.ReadAt(ctx, buf, 0)
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
		Context("write new content to file", func() {
			It("should be ok", func() {
				f, err = fsCore.Open(ctx, namespace, newMockFileEntry(name).ID, types.OpenAttr{Write: true})
				Expect(err).Should(BeNil())

				_, err = f.WriteAt(ctx, data, 0)
				Expect(err).Should(BeNil())
				Expect(f.Fsync(context.TODO())).Should(BeNil())
				Expect(f.Close(context.Background())).Should(BeNil())
				time.Sleep(time.Second)

				f, err = fsCore.Open(ctx, namespace, newMockFileEntry(name).ID, types.OpenAttr{Read: true})
				Expect(err).Should(BeNil())

				buf := make([]byte, 10)
				n, err := f.ReadAt(ctx, buf, 0)
				Expect(err).Should(BeNil())
				Expect(buf[:n]).Should(Equal(data))

				Expect(f.Close(context.Background())).Should(BeNil())
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
				f, err = fsCore.Open(ctx, namespace, newMockFileEntry("test-create-new-file").ID, types.OpenAttr{Write: true, Create: true})
				Expect(err).Should(BeNil())
				_, err = f.WriteAt(ctx, data, 0)
				Expect(err).Should(BeNil())
				Expect(f.Fsync(context.Background())).Should(BeNil())
				Expect(f.Close(context.Background())).Should(BeNil())
				time.Sleep(time.Second)

				f, err = fsCore.Open(ctx, namespace, newMockFileEntry("test-create-new-file").ID, types.OpenAttr{Read: true})
				Expect(err).Should(BeNil())
				buf := make([]byte, 10)
				n, err := f.ReadAt(ctx, buf, 0)
				Expect(err).Should(BeNil())
				Expect(buf[:n]).Should(Equal(data[:10]))
			})
		})
	})
})
