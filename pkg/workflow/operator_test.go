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

package workflow

import (
	"context"
	"github.com/basenana/go-flow/flow"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/plugin/common"
	"github.com/basenana/nanafs/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"strconv"
	"time"
)

var _ = Describe("TestEntryInitOperator", func() {
	Context("operator do", func() {
		var (
			op  flow.Operator
			err error
			en  dentry.Entry
		)
		It("init file entry should be succeed", func() {
			root, err := entryMgr.Root(context.Background())
			Expect(err).Should(BeNil())

			en, err = entryMgr.CreateEntry(context.Background(), root, dentry.EntryAttr{
				Name:   "test-file.txt",
				Kind:   types.RawKind,
				Access: root.Metadata().Access,
			})
			Expect(err).Should(BeNil())

			f, err := entryMgr.Open(context.Background(), en, dentry.Attr{Read: true, Write: true})
			Expect(err).Should(BeNil())
			defer f.Close(context.Background())

			_, err = f.WriteAt(context.Background(), []byte("hello"), 0)
			Expect(err).Should(BeNil())

			err = f.Fsync(context.Background())
			Expect(err).Should(BeNil())
		})
		It("create be succeed", func() {
			b := operatorBuilder{entryMgr: entryMgr}
			op, err = b.buildEntryInitOperator(flow.Spec{
				Type: opEntryInit,
				Parameter: map[string]string{
					paramEntryIdKey:   strconv.FormatInt(en.Metadata().ID, 10),
					paramEntryPathKey: en.Metadata().Name,
				},
			})
			Expect(err).Should(BeNil())
		})
		It("do be succeed", func() {
			err = op.Do(context.Background(), flow.Parameter{
				FlowID:  "mocked-flow-1",
				Workdir: tempDir,
			})
			Expect(err).Should(BeNil())
		})
	})
})

var _ = Describe("TestEntryCollectOperator", func() {
	Context("operator do", func() {
		var (
			op  flow.Operator
			err error
		)
		It("create be succeed", func() {
			b := operatorBuilder{entryMgr: entryMgr}
			op, err = b.buildEntryCollectOperator(flow.Spec{
				Type:      opEntryCollect,
				Parameter: map[string]string{},
			})
			Expect(err).Should(BeNil())
		})
		It("do be succeed", func() {
			err = op.Do(context.Background(), flow.Parameter{
				FlowID:  "mocked-flow-1",
				Workdir: tempDir,
			})
			Expect(err).Should(BeNil())
		})
	})
})

var _ = Describe("TestPluginCallOperator", func() {
	ps := &types.PlugScope{
		PluginName: "test-sleep-plugin-succeed",
		Version:    "1.0",
		PluginType: types.TypeProcess,
		Parameters: map[string]string{},
	}

	caller.mockResponse(*ps, func() (*common.Response, error) {
		time.Sleep(time.Second)
		return &common.Response{IsSucceed: true}, nil
	})

	Context("operator do", func() {
		var (
			op  flow.Operator
			err error
		)
		It("create be succeed", func() {
			b := operatorBuilder{entryMgr: entryMgr}
			op, err = b.buildPluginCallOperator(flow.Spec{
				Type: opPluginCall,
				Parameter: map[string]string{
					paramPluginName:    ps.PluginName,
					paramPluginVersion: ps.Version,
					paramPluginType:    string(types.TypeProcess),
				},
			})
			Expect(err).Should(BeNil())
		})

		It("do be succeed", func() {
			err = op.Do(context.Background(), flow.Parameter{
				FlowID:  "mocked-flow-1",
				Workdir: tempDir,
			})
			Expect(err).Should(BeNil())
		})
	})
})
