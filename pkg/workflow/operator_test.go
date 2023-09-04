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
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/pkg/workflow/jobrun"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"strconv"
)

var _ = Describe("TestEntryInitOperator", func() {
	Context("operator do", func() {
		var (
			op  jobrun.Operator
			err error
			en  *types.Metadata
		)
		It("init file entry should be succeed", func() {
			root, err := entryMgr.Root(context.Background())
			Expect(err).Should(BeNil())

			en, err = entryMgr.CreateEntry(context.Background(), root.ID, dentry.EntryAttr{
				Name:   "test-file.txt",
				Kind:   types.RawKind,
				Access: root.Access,
			})
			Expect(err).Should(BeNil())

			f, err := entryMgr.Open(context.Background(), en.ID, dentry.Attr{Read: true, Write: true})
			Expect(err).Should(BeNil())
			defer f.Close(context.Background())

			_, err = f.WriteAt(context.Background(), []byte("hello"), 0)
			Expect(err).Should(BeNil())

			err = f.Fsync(context.Background())
			Expect(err).Should(BeNil())
		})
		It("create be succeed", func() {
			b := operatorBuilder{entryMgr: entryMgr}
			op, err = b.buildEntryInitOperator(
				jobrun.Task{},
				jobrun.Spec{
					Type: opEntryInit,
					Parameters: map[string]string{
						paramEntryIdKey:   strconv.FormatInt(en.ID, 10),
						paramEntryPathKey: en.Name,
					},
				})
			Expect(err).Should(BeNil())
		})
		It("do be succeed", func() {
			err = op.Do(context.Background(), &jobrun.Parameter{
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
			op  jobrun.Operator
			err error
		)
		It("create be succeed", func() {
			b := operatorBuilder{entryMgr: entryMgr}
			op, err = b.buildEntryCollectOperator(
				jobrun.Task{},
				jobrun.Spec{
					Type:       opEntryCollect,
					Parameters: map[string]string{},
				})
			Expect(err).Should(BeNil())
		})
		It("do be succeed", func() {
			err = op.Do(context.Background(), &jobrun.Parameter{
				FlowID:  "mocked-flow-1",
				Workdir: tempDir,
			})
			Expect(err).Should(BeNil())
		})
	})
})

var _ = Describe("TestPluginCallOperator", func() {
	ps := &types.PlugScope{
		PluginName: "delay",
		Version:    "1.0",
		Action:     "delay",
		PluginType: types.TypeProcess,
	}

	Context("operator do", func() {
		var (
			op  jobrun.Operator
			err error
		)
		It("create be succeed", func() {
			b := operatorBuilder{entryMgr: entryMgr}
			op, err = b.buildPluginCallOperator(
				jobrun.Task{},
				jobrun.Spec{
					Type: opPluginCall,
					Parameters: map[string]string{
						paramPluginName:    ps.PluginName,
						paramPluginVersion: ps.Version,
						paramPluginType:    string(types.TypeProcess),
						paramPluginAction:  ps.Action,
						paramEntryIdKey:    "0",
						"delay":            "1s",
					},
				})
			Expect(err).Should(BeNil())
		})

		It("do be succeed", func() {
			err = op.Do(context.Background(), &jobrun.Parameter{
				FlowID:  "mocked-flow-1",
				Workdir: tempDir,
			})
			Expect(err).Should(BeNil())
		})
	})
})
