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

package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/basenana/nanafs/cmd/apps/apis/restfs/frame"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"mime/multipart"
	"net/http"
)

var _ = Describe("TestRestFsGet", func() {
	var (
		root dentry.Entry
		err  error
	)
	It("load root object", func() {
		root, err = ctrl.LoadRootEntry(context.Background())
		Expect(err).Should(BeNil())
	})

	Describe("test action read", func() {
		Context("normal", func() {
			It("create new file", func() {
				newFile, err := ctrl.CreateEntry(context.Background(), root, types.ObjectAttr{Name: "get-read-file1.txt", Kind: types.RawKind, Access: defaultAccessForTest()})
				Expect(err).Should(BeNil())

				f, err := ctrl.OpenFile(context.Background(), newFile, dentry.Attr{Read: true, Write: true})
				Expect(err).Should(BeNil())

				_, err = f.WriteAt(context.Background(), []byte("test"), 0)
				Expect(err).Should(BeNil())
				Expect(f.Close(context.Background())).Should(BeNil())
			})
			It("read file by default action read", func() {
				r, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8001/v1/fs/get-read-file1.txt", nil)
				Expect(err).Should(BeNil())
				resp, err := http.DefaultClient.Do(r)
				Expect(err).Should(BeNil())
				defer resp.Body.Close()
				Expect(resp.StatusCode).Should(Equal(http.StatusOK))

				buf := make([]byte, 1024)
				n, err := resp.Body.Read(buf)
				Expect(err).Should(BeNil())
				Expect(buf[:n]).Should(Equal([]byte("test")))
			})
			It("read file by action read", func() {
				r, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8001/v1/fs/get-read-file1.txt", nil)
				Expect(err).Should(BeNil())
				q := r.URL.Query()
				q.Add("action", "read")
				r.URL.RawQuery = q.Encode()
				resp, err := http.DefaultClient.Do(r)
				Expect(err).Should(BeNil())
				defer resp.Body.Close()
				Expect(resp.StatusCode).Should(Equal(http.StatusOK))

				buf := make([]byte, 1024)
				n, err := resp.Body.Read(buf)
				Expect(err).Should(BeNil())
				Expect(buf[:n]).Should(Equal([]byte("test")))
			})
		})
	})

	Describe("test action alias", func() {
		var oid int64
		Context("normal", func() {
			It("create new file", func() {
				newFile, err := ctrl.CreateEntry(context.Background(), root, types.ObjectAttr{Name: "get-alias-file1.txt", Kind: types.RawKind, Access: defaultAccessForTest()})
				Expect(err).Should(BeNil())
				oid = newFile.Metadata().ID
			})
			It("read file by action alias", func() {
				r, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8001/v1/fs/get-alias-file1.txt", nil)
				Expect(err).Should(BeNil())
				q := r.URL.Query()
				q.Add("action", "alias")
				r.URL.RawQuery = q.Encode()
				resp, err := http.DefaultClient.Do(r)
				Expect(err).Should(BeNil())
				defer resp.Body.Close()
				Expect(resp.StatusCode).Should(Equal(http.StatusOK))

				expect := struct {
					Data struct {
						ID int64 `json:"id"`
					} `json:"data"`
				}{}
				Expect(json.NewDecoder(resp.Body).Decode(&expect)).Should(BeNil())
				Expect(expect.Data.ID).Should(Equal(oid))
			})
		})
	})

	Describe("test action search", func() {
		Context("normal", func() {
			It("create new file", func() {
			})
			It("read file by action search", func() {
				// do nothing
			})
		})
	})

	Describe("test action download", func() {
		Context("normal", func() {
			It("create new file", func() {
			})
			It("read file by action download", func() {
				// do nothing
			})
		})
	})
})

var _ = Describe("TestRestFsPost", func() {
	var (
		root dentry.Entry
		err  error
	)
	It("load root object", func() {
		root, err = ctrl.LoadRootEntry(context.Background())
		Expect(err).Should(BeNil())
	})

	Describe("test action create", func() {
		var oid int64
		Context("normal", func() {
			It("create new file by action create", func() {
				req := frame.RequestV1{
					Parameters: frame.Parameters{
						Name:    "post-create-file1.txt",
						Content: []byte("test"),
					},
				}
				raw, _ := json.Marshal(req)
				r, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:8001/v1/fs/post-create-file1.txt", bytes.NewReader(raw))
				Expect(err).Should(BeNil())
				resp, err := http.DefaultClient.Do(r)
				Expect(err).Should(BeNil())
				defer resp.Body.Close()
				Expect(resp.StatusCode).Should(Equal(http.StatusOK))

				newObj := struct {
					Data struct {
						ID int64 `json:"id"`
					}
				}{}
				Expect(json.NewDecoder(resp.Body).Decode(&newObj)).Should(BeNil())
				oid = newObj.Data.ID
			})
			It("create succeed", func() {
				newObj, err := ctrl.FindEntry(context.Background(), root, "post-create-file1.txt")
				Expect(err).Should(BeNil())
				Expect(newObj.Metadata().ID).Should(Equal(oid))

				f, err := ctrl.OpenFile(context.Background(), newObj, dentry.Attr{Read: true})
				Expect(err).Should(BeNil())
				buf := make([]byte, 1024)
				n, err := f.ReadAt(context.Background(), buf, 0)
				Expect(err).Should(BeNil())
				Expect(buf[:n]).Should(Equal([]byte("test")))
			})
		})
	})

	Describe("test action bulk", func() {
		Context("normal", func() {
			It("create multi file by action bulk", func() {
				body := &bytes.Buffer{}

				writer := multipart.NewWriter(body)
				part1, _ := writer.CreateFormFile("files", "post-bulk-file1.txt")
				_, _ = part1.Write([]byte("content1"))
				part2, _ := writer.CreateFormFile("files", "post-bulk-file2.txt")
				_, _ = part2.Write([]byte("content2"))
				_ = writer.Close()

				r, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:8001/v1/fs/", body)
				Expect(err).Should(BeNil())
				q := r.URL.Query()
				q.Add("action", "bulk")
				r.URL.RawQuery = q.Encode()
				r.Header.Add("Content-Type", writer.FormDataContentType())

				resp, err := http.DefaultClient.Do(r)
				Expect(err).Should(BeNil())
				defer resp.Body.Close()
				Expect(resp.StatusCode).Should(Equal(http.StatusOK))
			})
			It("create succeed", func() {
				obj1, err := ctrl.FindEntry(context.Background(), root, "post-bulk-file1.txt")
				Expect(err).Should(BeNil())
				obj2, err := ctrl.FindEntry(context.Background(), root, "post-bulk-file2.txt")
				Expect(err).Should(BeNil())

				var (
					buf = make([]byte, 1024)
					n   int64
				)

				f1, err := ctrl.OpenFile(context.Background(), obj1, dentry.Attr{Read: true})
				Expect(err).Should(BeNil())
				defer ctrl.CloseFile(context.Background(), f1)

				n, err = ctrl.ReadFile(context.Background(), f1, buf, 0)
				Expect(err).Should(BeNil())
				Expect(buf[:n]).Should(Equal([]byte("content1")))

				f2, err := ctrl.OpenFile(context.Background(), obj2, dentry.Attr{Read: true})
				Expect(err).Should(BeNil())
				defer ctrl.CloseFile(context.Background(), f2)

				n, err = ctrl.ReadFile(context.Background(), f2, buf, 0)
				Expect(err).Should(BeNil())
				Expect(buf[:n]).Should(Equal([]byte("content2")))
			})
		})
	})

	Describe("test action copy", func() {
		Context("normal", func() {
			It("create new file", func() {
				newFile, err := ctrl.CreateEntry(context.Background(), root, types.ObjectAttr{Name: "post-copy-file1.txt", Kind: types.RawKind, Access: defaultAccessForTest()})
				Expect(err).Should(BeNil())

				f, err := ctrl.OpenFile(context.Background(), newFile, dentry.Attr{Read: true, Write: true})
				Expect(err).Should(BeNil())

				_, err = f.WriteAt(context.Background(), []byte("test"), 0)
				Expect(err).Should(BeNil())
				Expect(f.Close(context.Background())).Should(BeNil())
			})
			It("create file by action copy", func() {
				req := frame.RequestV1{
					Parameters: frame.Parameters{
						Destination: "/post-copy-file2.txt",
					},
				}
				raw, _ := json.Marshal(req)
				r, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:8001/v1/fs/post-copy-file1.txt", bytes.NewReader(raw))
				Expect(err).Should(BeNil())
				q := r.URL.Query()
				q.Add("action", "copy")
				r.URL.RawQuery = q.Encode()

				resp, err := http.DefaultClient.Do(r)
				Expect(err).Should(BeNil())
				defer resp.Body.Close()
				Expect(resp.StatusCode).Should(Equal(http.StatusOK))
			})
			It("copy succeed", func() {
				copyObj, err := ctrl.FindEntry(context.TODO(), root, "post-copy-file2.txt")
				Expect(err).Should(BeNil())

				f, err := ctrl.OpenFile(context.Background(), copyObj, dentry.Attr{Read: true})
				Expect(err).Should(BeNil())

				buf := make([]byte, 1024)
				n, err := f.ReadAt(context.TODO(), buf, 0)
				Expect(err).Should(BeNil())
				Expect(buf[:n]).Should(Equal([]byte("test")))
				Expect(ctrl.CloseFile(context.TODO(), f)).Should(BeNil())
			})
		})
	})
})

var _ = Describe("TestRestFsPut", func() {
	var (
		root dentry.Entry
		err  error
	)
	It("load root object", func() {
		root, err = ctrl.LoadRootEntry(context.Background())
		Expect(err).Should(BeNil())
	})

	Describe("test action update", func() {
		Context("normal", func() {
			It("create new file", func() {
				newFile, err := ctrl.CreateEntry(context.Background(), root, types.ObjectAttr{Name: "put-update-file1.txt", Kind: types.RawKind, Access: defaultAccessForTest()})
				Expect(err).Should(BeNil())

				f, err := ctrl.OpenFile(context.Background(), newFile, dentry.Attr{Read: true, Write: true})
				Expect(err).Should(BeNil())

				_, err = f.WriteAt(context.Background(), []byte("test"), 0)
				Expect(err).Should(BeNil())
				Expect(f.Close(context.Background())).Should(BeNil())
			})
			It("do update file", func() {
				req := frame.RequestV1{
					Parameters: frame.Parameters{
						Content: []byte("hello"),
					},
				}
				raw, _ := json.Marshal(req)
				r, err := http.NewRequest(http.MethodPut, "http://127.0.0.1:8001/v1/fs/put-update-file1.txt", bytes.NewReader(raw))
				Expect(err).Should(BeNil())
				q := r.URL.Query()
				q.Add("action", "update")
				r.URL.RawQuery = q.Encode()
				resp, err := http.DefaultClient.Do(r)
				Expect(err).Should(BeNil())
				defer resp.Body.Close()
				Expect(resp.StatusCode).Should(Equal(http.StatusOK))
			})
			It("update succeed", func() {
				obj, err := ctrl.FindEntry(context.Background(), root, "put-update-file1.txt")
				Expect(err).Should(BeNil())
				f, err := ctrl.OpenFile(context.Background(), obj, dentry.Attr{Read: true})
				Expect(err).Should(BeNil())
				buf := make([]byte, 1024)
				n, err := f.ReadAt(context.Background(), buf, 0)
				Expect(err).Should(BeNil())
				Expect(buf[:n]).Should(Equal([]byte("hello")))
				Expect(ctrl.CloseFile(context.TODO(), f)).Should(BeNil())
			})
		})
	})

	Describe("test action move", func() {
		var (
			srcDir, dstDir dentry.Entry
		)
		Context("normal", func() {
			It("create new file", func() {
				srcDir, err = ctrl.CreateEntry(context.Background(), root, types.ObjectAttr{Name: "put-move-src-dir", Kind: types.GroupKind, Access: defaultAccessForTest()})
				Expect(err).Should(BeNil())
				dstDir, err = ctrl.CreateEntry(context.Background(), root, types.ObjectAttr{Name: "put-move-dst-dir", Kind: types.GroupKind, Access: defaultAccessForTest()})
				Expect(err).Should(BeNil())

				newFile, err := ctrl.CreateEntry(context.Background(), srcDir, types.ObjectAttr{Name: "put-move-file1.txt", Kind: types.RawKind, Access: defaultAccessForTest()})
				Expect(err).Should(BeNil())

				f, err := ctrl.OpenFile(context.Background(), newFile, dentry.Attr{Read: true, Write: true})
				Expect(err).Should(BeNil())

				_, err = f.WriteAt(context.Background(), []byte("test move file"), 0)
				Expect(err).Should(BeNil())
				Expect(f.Close(context.Background())).Should(BeNil())
			})
			It("do move action", func() {
				req := frame.RequestV1{
					Parameters: frame.Parameters{
						Destination: "put-move-dst-dir",
					},
				}
				raw, _ := json.Marshal(req)
				r, err := http.NewRequest(http.MethodPut, "http://127.0.0.1:8001/v1/fs/put-move-src-dir/put-move-file1.txt", bytes.NewReader(raw))
				Expect(err).Should(BeNil())
				q := r.URL.Query()
				q.Add("action", "move")
				r.URL.RawQuery = q.Encode()
				resp, err := http.DefaultClient.Do(r)
				Expect(err).Should(BeNil())
				defer resp.Body.Close()
				Expect(resp.StatusCode).Should(Equal(http.StatusOK))
			})
			It("move succeed", func() {
				_, err = ctrl.FindEntry(context.Background(), srcDir, "put-move-file1.txt")
				Expect(err).Should(Equal(types.ErrNotFound))

				moved, err := ctrl.FindEntry(context.Background(), dstDir, "put-move-file1.txt")
				Expect(err).Should(BeNil())

				f, err := ctrl.OpenFile(context.Background(), moved, dentry.Attr{Read: true})
				Expect(err).Should(BeNil())
				buf := make([]byte, 1024)
				n, err := f.ReadAt(context.Background(), buf, 0)
				Expect(err).Should(BeNil())
				Expect(buf[:n]).Should(Equal([]byte("test move file")))
				Expect(ctrl.CloseFile(context.TODO(), f)).Should(BeNil())
			})
		})
	})

	Describe("test action rename", func() {
		Context("normal", func() {
			It("create new file", func() {
				newFile, err := ctrl.CreateEntry(context.Background(), root, types.ObjectAttr{Name: "put-rename-old-file1.txt", Kind: types.RawKind, Access: defaultAccessForTest()})
				Expect(err).Should(BeNil())

				f, err := ctrl.OpenFile(context.Background(), newFile, dentry.Attr{Read: true, Write: true})
				Expect(err).Should(BeNil())

				_, err = f.WriteAt(context.Background(), []byte("test"), 0)
				Expect(err).Should(BeNil())
				Expect(f.Close(context.Background())).Should(BeNil())
			})
			It("do rename", func() {
				req := frame.RequestV1{
					Parameters: frame.Parameters{
						Name: "put-rename-new-file1.txt",
					},
				}
				raw, _ := json.Marshal(req)
				r, err := http.NewRequest(http.MethodPut, "http://127.0.0.1:8001/v1/fs/put-rename-old-file1.txt", bytes.NewReader(raw))
				Expect(err).Should(BeNil())
				q := r.URL.Query()
				q.Add("action", "rename")
				r.URL.RawQuery = q.Encode()

				resp, err := http.DefaultClient.Do(r)
				Expect(err).Should(BeNil())
				defer resp.Body.Close()
				Expect(resp.StatusCode).Should(Equal(http.StatusOK))
			})
			It("rename succeed", func() {
				_, err = ctrl.FindEntry(context.Background(), root, "put-rename-old-file1.txt")
				Expect(err).Should(Equal(types.ErrNotFound))
				_, err = ctrl.FindEntry(context.Background(), root, "put-rename-new-file1.txt")
				Expect(err).Should(BeNil())
			})
		})
	})
})

var _ = Describe("TestRestFsDelete", func() {
	var (
		root dentry.Entry
		err  error
	)
	It("load root object", func() {
		root, err = ctrl.LoadRootEntry(context.Background())
		Expect(err).Should(BeNil())
	})

	Describe("test action delete", func() {
		Context("normal", func() {
			It("create new file", func() {
				newFile, err := ctrl.CreateEntry(context.Background(), root, types.ObjectAttr{Name: "delete-delete-file1.txt", Kind: types.RawKind, Access: defaultAccessForTest()})
				Expect(err).Should(BeNil())

				f, err := ctrl.OpenFile(context.Background(), newFile, dentry.Attr{Read: true, Write: true})
				Expect(err).Should(BeNil())

				_, err = f.WriteAt(context.Background(), []byte("test"), 0)
				Expect(err).Should(BeNil())
				Expect(f.Close(context.Background())).Should(BeNil())
			})
			It("do delete", func() {
				r, err := http.NewRequest(http.MethodDelete, "http://127.0.0.1:8001/v1/fs/delete-delete-file1.txt", nil)
				Expect(err).Should(BeNil())
				resp, err := http.DefaultClient.Do(r)
				Expect(err).Should(BeNil())
				defer resp.Body.Close()
				Expect(resp.StatusCode).Should(Equal(http.StatusOK))
			})
			It("file deleted", func() {
				_, err := ctrl.FindEntry(context.Background(), root, "delete-delete-file1.txt")
				Expect(err).Should(Equal(types.ErrNotFound))
			})
		})
	})
})
