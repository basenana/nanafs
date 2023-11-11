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
	"github.com/basenana/nanafs/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"strings"
)

func init() {
	parseRssUrl = func(url string) (string, string, error) {
		return "Hypo's Blog", "https://blog.ihypo.net", nil
	}
}

var _ = Describe("testConfigEntrySourcePlugin", func() {
	var (
		entryID int64
		ctx     = context.TODO()
	)
	Context("create group entry", func() {
		It("create should be succeed", func() {
			root, err := ctrl.LoadRootEntry(ctx)
			Expect(err).Should(BeNil())
			en, err := ctrl.CreateEntry(ctx, root.ID, types.EntryAttr{
				Name:   "test-rss-group",
				Kind:   types.GroupKind,
				Access: root.Access,
			})
			Expect(err).Should(BeNil())
			entryID = en.ID
		})
	})
	Context("config group entry source plugin", func() {
		var (
			patch types.ExtendData
			err   error
		)
		It("init rss ed should be succeed", func() {
			patch, err = BuildRssPluginScopeFromURL("https://blog.ihypo.net/atom.xml")
			Expect(err).Should(BeNil())
		})

		It("config should be succeed", func() {
			err = ctrl.ConfigEntrySourcePlugin(ctx, entryID, patch)
			Expect(err).Should(BeNil())
		})
		It("check should be succeed", func() {
			ed, err := entryStore.GetEntryExtendData(ctx, entryID)
			Expect(err).Should(BeNil())

			Expect(ed.PlugScope).ShouldNot(BeNil())
			Expect(ed.PlugScope.PluginName).Should(Equal("rss"))
			Expect(string(ed.PlugScope.PluginType)).Should(Equal(string(types.TypeSource)))

			labels, err := entryStore.GetEntryLabels(ctx, entryID)
			Expect(err).Should(BeNil())

			var needLabels []string
			for _, l := range labels.Labels {
				switch l.Key {
				case types.LabelKeyPluginName, types.LabelKeyPluginKind:
					needLabels = append(needLabels, l.Key)
				}
			}
			Expect(len(needLabels)).Should(Equal(2))
		})
	})
	Context("clean group entry source plugin", func() {
		It("clean should be succeed", func() {
			err := ctrl.CleanupEntrySourcePlugin(ctx, entryID)
			Expect(err).Should(BeNil())
		})
		It("check should be succeed", func() {
			ed, err := entryStore.GetEntryExtendData(ctx, entryID)
			Expect(err).Should(BeNil())
			Expect(ed.PlugScope).Should(BeNil())

			labels, err := entryStore.GetEntryLabels(ctx, entryID)
			Expect(err).Should(BeNil())

			var needLabels []string
			for _, l := range labels.Labels {
				if strings.HasPrefix(l.Key, types.LabelKeyPluginPrefix) {
					needLabels = append(needLabels, l.Key)
				}
			}
			Expect(len(needLabels)).Should(Equal(0))
		})
	})
})
