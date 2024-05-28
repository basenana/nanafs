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

package controller

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/basenana/nanafs/pkg/types"
)

var _ = Describe("testBuildGroupEntry", func() {
	var ctx = context.TODO()
	Context("create group entry", func() {
		It("create should be succeed", func() {
			root, err := ctrl.LoadRootEntry(ctx)
			Expect(err).Should(BeNil())

			group1, err := ctrl.CreateEntry(ctx, root.ID, types.EntryAttr{
				Name:   "group1",
				Kind:   types.GroupKind,
				Access: &root.Access,
			})
			Expect(err).Should(BeNil())

			group2, err := ctrl.CreateEntry(ctx, group1.ID, types.EntryAttr{
				Name:   "group2",
				Kind:   types.GroupKind,
				Access: &root.Access,
			})

			_, err = ctrl.CreateEntry(ctx, group2.ID, types.EntryAttr{
				Name:   "file1",
				Kind:   types.RawKind,
				Access: &root.Access,
			})
			Expect(err).Should(BeNil())
		})
	})
	Context("create group entry", func() {
		It("create should be succeed", func() {
			groupTree, err := ctrl.GetGroupTree(ctx)
			Expect(err).Should(BeNil())

			var (
				group1 *types.GroupEntry
				group2 *types.GroupEntry
			)

			for i, g := range groupTree.Children {
				if g.Entry.Name == "group1" {
					group1 = groupTree.Children[i]
				}
			}
			Expect(group1).ShouldNot(BeNil())

			for i, g := range group1.Children {
				if g.Entry.Name == "group2" {
					group2 = group1.Children[i]
				}
			}
			Expect(group2).ShouldNot(BeNil())

			Expect(len(group2.Children)).Should(Equal(0))
		})
	})
})
