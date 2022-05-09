package restfs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/basenana/nanafs/pkg/files"
	"github.com/basenana/nanafs/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
)

var _ = Describe("TestRestFsGet", func() {
	var (
		root *types.Object
		err  error
	)
	It("load root object", func() {
		root, err = ctrl.LoadRootObject(context.Background())
		Expect(err).Should(BeNil())
	})

	Describe("test action read", func() {
		Context("normal", func() {
			It("create new file", func() {
				newFile, err := ctrl.CreateObject(context.Background(), root, types.ObjectAttr{Name: "get-read-file1.txt", Kind: types.RawKind, Access: defaultAccessForTest()})
				Expect(err).Should(BeNil())

				f, err := ctrl.OpenFile(context.Background(), newFile, files.Attr{Read: true, Write: true})
				Expect(err).Should(BeNil())

				_, err = f.Write(context.Background(), []byte("test"), 0)
				Expect(err).Should(BeNil())
				Expect(f.Close(context.Background())).Should(BeNil())
			})
			It("read file by default action read", func() {
				r, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8001/fs/get-read-file1.txt", nil)
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
				r, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8001/fs/get-read-file1.txt", nil)
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
		})
	})

	Describe("test action alias", func() {
		var oid string
		Context("normal", func() {
			It("create new file", func() {
				newFile, err := ctrl.CreateObject(context.Background(), root, types.ObjectAttr{Name: "get-alias-file1.txt", Kind: types.RawKind, Access: defaultAccessForTest()})
				Expect(err).Should(BeNil())
				oid = newFile.ID
			})
			It("read file by action alias", func() {
				act := Action{Action: ActionAlias}
				raw, _ := json.Marshal(FsRequest{Data: act})
				r, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8001/fs/get-alias-file1.txt", bytes.NewReader(raw))
				Expect(err).Should(BeNil())
				resp, err := http.DefaultClient.Do(r)
				Expect(err).Should(BeNil())
				defer resp.Body.Close()
				Expect(resp.StatusCode).Should(Equal(http.StatusOK))

				expect := struct {
					Data struct {
						ID string `json:"id"`
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
		root *types.Object
		err  error
	)
	It("load root object", func() {
		root, err = ctrl.LoadRootObject(context.Background())
		Expect(err).Should(BeNil())
	})

	Describe("test action create", func() {
		var oid string
		Context("normal", func() {
			It("create new file by action create", func() {
				act := Action{
					Action: ActionCreate,
					Parameters: struct {
						Name    string   `json:"name"`
						Content []byte   `json:"content"`
						Flags   []string `json:"flags"`
						Fields  []string `json:"fields"`
					}{
						Name:    "post-create-file1.txt",
						Content: []byte("test"),
					},
				}
				raw, _ := json.Marshal(FsRequest{Data: act})
				r, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:8001/fs/post-create-file1.txt", bytes.NewReader(raw))
				Expect(err).Should(BeNil())
				resp, err := http.DefaultClient.Do(r)
				Expect(err).Should(BeNil())
				defer resp.Body.Close()
				Expect(resp.StatusCode).Should(Equal(http.StatusOK))

				newObj := struct {
					Data struct {
						ID string `json:"id"`
					}
				}{}
				Expect(json.NewDecoder(resp.Body).Decode(&newObj)).Should(BeNil())
				oid = newObj.Data.ID
			})
			It("create succeed", func() {
				newObj, err := ctrl.FindObject(context.Background(), root, "post-create-file1.txt")
				Expect(err).Should(BeNil())
				Expect(newObj.ID).Should(Equal(oid))

				f, err := ctrl.OpenFile(context.Background(), newObj, files.Attr{Read: true})
				Expect(err).Should(BeNil())
				buf := make([]byte, 1024)
				n, err := f.Read(context.Background(), buf, 0)
				Expect(err).Should(BeNil())
				Expect(buf[:n]).Should(Equal([]byte("test")))
			})
		})
	})

	Describe("test action bulk", func() {
		Context("normal", func() {
			It("create multi file by action bulk", func() {
				body := &bytes.Buffer{}

				act := Action{Action: ActionBulk}
				raw, _ := json.Marshal(FsRequest{Data: act})

				writer := multipart.NewWriter(body)
				header := textproto.MIMEHeader{}
				header.Add("Content-Type", mime.FormatMediaType("application/json", map[string]string{"charset": "UTF-8"}))
				header.Add("Content-Disposition", fmt.Sprintf("form-data; name=\"%s\"", bulkActionKey))
				actWriter, _ := writer.CreatePart(header)
				_, _ = actWriter.Write(raw)

				part1, _ := writer.CreateFormFile(bulkFileKey, "post-bulk-file1.txt")
				_, _ = part1.Write([]byte("content1"))
				part2, _ := writer.CreateFormFile(bulkFileKey, "post-bulk-file2.txt")
				_, _ = part2.Write([]byte("content2"))
				_ = writer.Close()

				r, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:8001/fs/", body)
				Expect(err).Should(BeNil())
				r.Header.Add("Content-Type", writer.FormDataContentType())

				resp, err := http.DefaultClient.Do(r)
				Expect(err).Should(BeNil())
				defer resp.Body.Close()
				Expect(resp.StatusCode).Should(Equal(http.StatusOK))
			})
			It("create succeed", func() {
				obj1, err := ctrl.FindObject(context.Background(), root, "post-bulk-file1.txt")
				Expect(err).Should(BeNil())
				obj2, err := ctrl.FindObject(context.Background(), root, "post-bulk-file2.txt")
				Expect(err).Should(BeNil())

				var (
					buf = make([]byte, 1024)
					n   int
				)

				f1, err := ctrl.OpenFile(context.Background(), obj1, files.Attr{Read: true})
				Expect(err).Should(BeNil())
				defer ctrl.CloseFile(context.Background(), f1)

				n, err = ctrl.ReadFile(context.Background(), f1, buf, 0)
				Expect(err).Should(BeNil())
				Expect(buf[:n]).Should(Equal([]byte("content1")))

				f2, err := ctrl.OpenFile(context.Background(), obj2, files.Attr{Read: true})
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
			It("create file by action copy", func() {
			})
			It("create succeed", func() {
				// do nothing
			})
		})
	})
})

var _ = Describe("TestRestFsPut", func() {
	var (
		root *types.Object
		err  error
	)
	It("load root object", func() {
		root, err = ctrl.LoadRootObject(context.Background())
		Expect(err).Should(BeNil())
	})

	Describe("test action update", func() {
		Context("normal", func() {
			It("create new file", func() {
				newFile, err := ctrl.CreateObject(context.Background(), root, types.ObjectAttr{Name: "put-update-file1.txt", Kind: types.RawKind, Access: defaultAccessForTest()})
				Expect(err).Should(BeNil())

				f, err := ctrl.OpenFile(context.Background(), newFile, files.Attr{Read: true, Write: true})
				Expect(err).Should(BeNil())

				_, err = f.Write(context.Background(), []byte("test"), 0)
				Expect(err).Should(BeNil())
				Expect(f.Close(context.Background())).Should(BeNil())
			})
			It("do update file", func() {
				act := Action{
					Action: ActionUpdate,
					Parameters: struct {
						Name    string   `json:"name"`
						Content []byte   `json:"content"`
						Flags   []string `json:"flags"`
						Fields  []string `json:"fields"`
					}{
						Content: []byte("hello"),
					},
				}
				raw, _ := json.Marshal(FsRequest{Data: act})
				r, err := http.NewRequest(http.MethodPut, "http://127.0.0.1:8001/fs/put-update-file1.txt", bytes.NewReader(raw))
				Expect(err).Should(BeNil())
				resp, err := http.DefaultClient.Do(r)
				Expect(err).Should(BeNil())
				defer resp.Body.Close()
				Expect(resp.StatusCode).Should(Equal(http.StatusOK))
			})
			It("update succeed", func() {
				obj, err := ctrl.FindObject(context.Background(), root, "put-update-file1.txt")
				Expect(err).Should(BeNil())
				f, err := ctrl.OpenFile(context.Background(), obj, files.Attr{Read: true})
				Expect(err).Should(BeNil())
				buf := make([]byte, 1024)
				n, err := f.Read(context.Background(), buf, 0)
				Expect(err).Should(BeNil())
				Expect(buf[:n]).Should(Equal([]byte("hello")))
			})
		})
	})

	Describe("test action move", func() {
		Context("normal", func() {
			It("create new file", func() {
			})
			It("do move action", func() {
				// do nothing
			})
			It("move succeed", func() {
			})
		})
	})

	Describe("test action rename", func() {
		Context("normal", func() {
			It("create new file", func() {
				newFile, err := ctrl.CreateObject(context.Background(), root, types.ObjectAttr{Name: "put-rename-old-file1.txt", Kind: types.RawKind, Access: defaultAccessForTest()})
				Expect(err).Should(BeNil())

				f, err := ctrl.OpenFile(context.Background(), newFile, files.Attr{Read: true, Write: true})
				Expect(err).Should(BeNil())

				_, err = f.Write(context.Background(), []byte("test"), 0)
				Expect(err).Should(BeNil())
				Expect(f.Close(context.Background())).Should(BeNil())
			})
			It("do rename", func() {
				act := Action{
					Action: ActionRename,
					Parameters: struct {
						Name    string   `json:"name"`
						Content []byte   `json:"content"`
						Flags   []string `json:"flags"`
						Fields  []string `json:"fields"`
					}{
						Name: "put-rename-new-file1.txt",
					},
				}
				raw, _ := json.Marshal(FsRequest{Data: act})
				r, err := http.NewRequest(http.MethodPut, "http://127.0.0.1:8001/fs/put-rename-old-file1.txt", bytes.NewReader(raw))
				Expect(err).Should(BeNil())
				resp, err := http.DefaultClient.Do(r)
				Expect(err).Should(BeNil())
				defer resp.Body.Close()
				Expect(resp.StatusCode).Should(Equal(http.StatusOK))
			})
			It("rename succeed", func() {
				_, err = ctrl.FindObject(context.Background(), root, "put-rename-old-file1.txt")
				Expect(err).Should(Equal(types.ErrNotFound))
				_, err = ctrl.FindObject(context.Background(), root, "put-rename-new-file1.txt")
				Expect(err).Should(BeNil())
			})
		})
	})
})

var _ = Describe("TestRestFsDelete", func() {
	var (
		root *types.Object
		err  error
	)
	It("load root object", func() {
		root, err = ctrl.LoadRootObject(context.Background())
		Expect(err).Should(BeNil())
	})

	Describe("test action delete", func() {
		Context("normal", func() {
			It("create new file", func() {
				newFile, err := ctrl.CreateObject(context.Background(), root, types.ObjectAttr{Name: "delete-delete-file1.txt", Kind: types.RawKind, Access: defaultAccessForTest()})
				Expect(err).Should(BeNil())

				f, err := ctrl.OpenFile(context.Background(), newFile, files.Attr{Read: true, Write: true})
				Expect(err).Should(BeNil())

				_, err = f.Write(context.Background(), []byte("test"), 0)
				Expect(err).Should(BeNil())
				Expect(f.Close(context.Background())).Should(BeNil())
			})
			It("do delete", func() {
				r, err := http.NewRequest(http.MethodDelete, "http://127.0.0.1:8001/fs/delete-delete-file1.txt", nil)
				Expect(err).Should(BeNil())
				resp, err := http.DefaultClient.Do(r)
				Expect(err).Should(BeNil())
				defer resp.Body.Close()
				Expect(resp.StatusCode).Should(Equal(http.StatusOK))
			})
			It("file deleted", func() {
				_, err := ctrl.FindObject(context.Background(), root, "delete-delete-file1.txt")
				Expect(err).Should(Equal(types.ErrNotFound))
			})
		})
	})
})
