package plugin

import (
	"context"
	"github.com/basenana/nanafs/pkg/plugin/common"
	"github.com/basenana/nanafs/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
)

var _ = Describe("TestSourcePluginCall", func() {
	ps := types.PlugScope{
		PluginName: "dummy-source-plugin",
		Version:    "1.0",
		PluginType: types.TypeSource,
		Parameters: map[string]string{},
	}

	var req *common.Request
	BeforeEach(func() {
		req = &common.Request{
			CallType: common.CallTrigger,
			WorkPath: "/",
		}
	})

	Context("with a normal call", func() {
		It("should be succeed", func() {
			resp, err := Call(context.TODO(), ps, req)
			Expect(err).Should(BeNil())
			Expect(resp.IsSucceed).Should(BeTrue())
		})
	})
})

var _ = Describe("TestProcessPluginCall", func() {
	ps := types.PlugScope{
		PluginName: "dummy-process-plugin",
		Version:    "1.0",
		PluginType: types.TypeProcess,
		Parameters: map[string]string{},
	}

	var req *common.Request
	BeforeEach(func() {
		req = &common.Request{
			CallType: common.CallTrigger,
			WorkPath: "/",
		}
	})

	Context("with a normal call", func() {
		It("should be succeed", func() {
			resp, err := Call(context.TODO(), ps, req)
			Expect(err).Should(BeNil())
			Expect(resp.IsSucceed).Should(BeTrue())
		})
	})
})

var _ = Describe("TestMirrorPluginCall", func() {
	ps := types.PlugScope{
		PluginName: "dummy-mirror-plugin",
		Version:    "1.0",
		PluginType: types.TypeMirror,
		Parameters: map[string]string{},
	}

	var req *common.Request
	BeforeEach(func() {
		req = &common.Request{
			WorkPath: "/",
		}
	})

	fileName := "testfile.txt"

	Context("with a normal call", func() {
		It("list should be succeed", func() {
			req.CallType = common.CallListEntries
			resp, err := Call(context.TODO(), ps, req)
			Expect(err).Should(BeNil())
			Expect(resp.IsSucceed).Should(BeTrue())
		})
		It("add testfile.txt should be succeed", func() {
			var (
				resp *common.Response
				err  error
			)
			req.CallType = common.CallAddEntry
			req.Entry = common.NewFileEntry(fileName, []byte{})
			resp, err = Call(context.TODO(), ps, req)
			Expect(err).Should(BeNil())
			Expect(resp.IsSucceed).Should(BeTrue())

			req.CallType = common.CallListEntries
			resp, err = Call(context.TODO(), ps, req)
			Expect(err).Should(BeNil())
			Expect(resp.IsSucceed).Should(BeTrue())

			searched := false
			for _, en := range resp.Entries {
				if en.Name() == fileName {
					searched = true
					break
				}
			}
			Expect(searched).Should(BeTrue())
		})
		It("update should be succeed", func() {
			req.CallType = common.CallUpdateEntry
			req.Entry = common.NewFileEntry(fileName, []byte("hello1"))
			resp, err := Call(context.TODO(), ps, req)
			Expect(err).Should(BeNil())
			Expect(resp.IsSucceed).Should(BeTrue())

			req.CallType = common.CallListEntries
			resp, err = Call(context.TODO(), ps, req)
			Expect(err).Should(BeNil())
			Expect(resp.IsSucceed).Should(BeTrue())

			var newFile *common.FileEntry
			for _, en := range resp.Entries {
				if en.Name() == fileName {
					newFile = en.(*common.FileEntry)
					break
				}
			}
			Expect(newFile).ShouldNot(BeNil())
			r, err := newFile.OpenReader()
			Expect(err).Should(BeNil())
			content, err := ioutil.ReadAll(r)
			Expect(err).Should(BeNil())
			Expect(string(content)).Should(Equal("hello1"))
		})
		It("delete should be succeed", func() {
			var (
				resp     *common.Response
				err      error
				searched bool
			)
			req.CallType = common.CallListEntries
			resp, err = Call(context.TODO(), ps, req)
			Expect(err).Should(BeNil())
			Expect(resp.IsSucceed).Should(BeTrue())

			for _, en := range resp.Entries {
				if en.Name() == fileName {
					searched = true
					break
				}
			}
			Expect(searched).Should(BeTrue())

			req.CallType = common.CallDeleteEntry
			req.Entry = common.NewFileEntry(fileName, nil)
			resp, err = Call(context.TODO(), ps, req)
			Expect(err).Should(BeNil())
			Expect(resp.IsSucceed).Should(BeTrue())

			searched = false
			for _, en := range resp.Entries {
				if en.Name() == fileName {
					searched = true
					break
				}
			}
			Expect(searched).Should(BeFalse())
		})
	})
})
