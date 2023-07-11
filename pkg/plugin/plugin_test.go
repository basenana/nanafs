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

package plugin

import (
	"context"
	"github.com/basenana/nanafs/pkg/plugin/stub"
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

	var req *stub.Request
	BeforeEach(func() {
		req = &stub.Request{
			CallType: stub.CallTrigger,
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

	var req *stub.Request
	BeforeEach(func() {
		req = &stub.Request{
			CallType: stub.CallTrigger,
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

	var req *stub.Request
	BeforeEach(func() {
		req = &stub.Request{
			WorkPath: "/",
		}
	})

	fileName := "testfile.txt"

	Context("with a normal call", func() {
		It("list should be succeed", func() {
			req.CallType = stub.CallListEntries
			resp, err := Call(context.TODO(), ps, req)
			Expect(err).Should(BeNil())
			Expect(resp.IsSucceed).Should(BeTrue())
		})
		It("add testfile.txt should be succeed", func() {
			var (
				resp *stub.Response
				err  error
			)
			req.CallType = stub.CallAddEntry
			req.Entry = stub.NewFileEntry(fileName, []byte{})
			resp, err = Call(context.TODO(), ps, req)
			Expect(err).Should(BeNil())
			Expect(resp.IsSucceed).Should(BeTrue())

			req.CallType = stub.CallListEntries
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
			req.CallType = stub.CallUpdateEntry
			req.Entry = stub.NewFileEntry(fileName, []byte("hello1"))
			resp, err := Call(context.TODO(), ps, req)
			Expect(err).Should(BeNil())
			Expect(resp.IsSucceed).Should(BeTrue())

			req.CallType = stub.CallListEntries
			resp, err = Call(context.TODO(), ps, req)
			Expect(err).Should(BeNil())
			Expect(resp.IsSucceed).Should(BeTrue())

			var newFile *stub.FileEntry
			for _, en := range resp.Entries {
				if en.Name() == fileName {
					newFile = en.(*stub.FileEntry)
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
				resp     *stub.Response
				err      error
				searched bool
			)
			req.CallType = stub.CallListEntries
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

			req.CallType = stub.CallDeleteEntry
			req.Entry = stub.NewFileEntry(fileName, nil)
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
