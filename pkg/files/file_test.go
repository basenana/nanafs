package files

import (
	"bytes"
	"context"
	"github.com/basenana/nanafs/pkg/storage"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io"
)

var _ = Describe("TestFileIO", func() {
	var (
		s   storage.Storage
		key = "test-file-key"
	)
	BeforeEach(func() {
		s = NewMockStorage()
		_ = s.Put(context.Background(), key, bytes.NewReader([]byte("testdata-1")), 0, 0)
		_ = s.Put(context.Background(), key, bytes.NewReader([]byte("          ")), 1, 0)
		_ = s.Put(context.Background(), key, bytes.NewReader([]byte("testdata-2")), 2, 0)
		_ = s.Put(context.Background(), key, bytes.NewReader([]byte("     ")), 3, 0)
	})

	Describe("test file open", func() {
		Context("open a file", func() {
			It("should be ok", func() {
				f, err := openFile(context.Background(), newMockObject(key), Attr{Read: true, Storage: s})
				Expect(err).Should(BeNil())
				Expect(f.Close(context.Background())).Should(BeNil())
			})
		})
		Context("open not found file", func() {
			It("should be ok", func() {
				_, err := openFile(context.Background(), newMockObject("not-found-file"), Attr{Read: true, Storage: s})
				Expect(err).ShouldNot(BeNil())
			})
		})
	})

	Describe("test file read", func() {
		var (
			f   *file
			err error
		)
		BeforeEach(func() {
			f, err = openFile(context.Background(), newMockObject(key), Attr{Read: true, Storage: s})
			Expect(err).Should(BeNil())
		})
		Context("read file succeed", func() {
			It("should be ok", func() {
				buf := make([]byte, 1024)
				n, err := f.Read(context.Background(), buf, 0)
				Expect(err).Should(Equal(io.EOF))
				Expect(buf[:n]).Should(Equal([]byte("testdata-1          testdata-2     ")))
			})
		})
		Context("read file failed", func() {
			It("should be no perm", func() {
				buf := make([]byte, 1024)
				f, err = openFile(context.Background(), newMockObject(key), Attr{Write: true, Storage: s})
				Expect(err).Should(BeNil())
				_, err = f.Read(context.Background(), buf, 0)
				Expect(err).ShouldNot(BeNil())
			})
		})
	})

	Describe("test file write", func() {
		var (
			data = []byte("testdata-3")
			f    *file
			err  error
		)
		BeforeEach(func() {
			f, err = openFile(context.Background(), newMockObject(key), Attr{Write: true, Storage: s})
			Expect(err).Should(BeNil())
		})
		AfterEach(func() {
			Expect(f.Close(context.Background())).Should(BeNil())
		})
		Context("write new content to file", func() {
			It("should be ok", func() {
				_, err = f.Write(context.Background(), data, 0)
				Expect(err).Should(BeNil())
				Expect(f.Close(context.Background())).Should(BeNil())

				Context("read file content", func() {
					f, err = openFile(context.Background(), newMockObject(key), Attr{Read: true, Storage: s})
					Expect(err).Should(BeNil())

					buf := make([]byte, 1024)
					n, err := f.Read(context.Background(), buf, 0)
					Expect(err).Should(Equal(io.EOF))
					Expect(buf[:n]).Should(Equal([]byte("testdata-3          testdata-2     ")))
				})
			})
		})
		Context("write a file without perm", func() {
			It("should be no perm", func() {
				f, err = openFile(context.Background(), newMockObject(key), Attr{Read: true, Storage: s})
				Expect(err).Should(BeNil())
				_, err = f.Write(context.Background(), data, 0)
				Expect(err).ShouldNot(BeNil())
			})
		})
	})

	Describe("test create new file", func() {
		var (
			data = []byte("testdata-2")
			f    *file
			err  error
		)

		Context("create and write a new file", func() {
			It("should be ok", func() {
				f, err = openFile(context.Background(), newMockObject("test-create-new-file"), Attr{Write: true, Create: true, Storage: s})
				Expect(err).Should(BeNil())
				_, err = f.Write(context.Background(), data, 0)
				Expect(err).Should(BeNil())
				Expect(f.Close(context.Background())).Should(BeNil())

				Context("read new file", func() {
					f, err = openFile(context.Background(), newMockObject("test-create-new-file"), Attr{Read: true, Storage: s})
					Expect(err).Should(BeNil())
					buf := make([]byte, 1024)
					n, err := f.Read(context.Background(), buf, 0)
					Expect(err).Should(Equal(io.EOF))
					Expect(buf[:n]).Should(Equal(data))
				})
			})
		})
		Context("write a new file without perm", func() {
			It("should be no perm", func() {
				f, err = openFile(context.Background(), newMockObject("test-create-new-file"), Attr{Write: true, Storage: s})
				Expect(err).ShouldNot(BeNil())
			})
		})
	})
})
