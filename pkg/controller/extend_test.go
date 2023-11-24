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
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/basenana/nanafs/pkg/types"
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
				Name: "test-rss-group",
				Kind: types.GroupKind,
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

var _ = Describe("testFeed", func() {
	Context("get documents by feed", func() {
		var (
			grp     *types.Metadata
			grpFile *types.Metadata

			siteUrl  string = "https://abc"
			siteName string = "test for feed"
			feedUrl  string = "https://abc/feed"

			metaId      string = "https://abc/123.html"
			metaTitle   string = "test blog"
			metaLink    string = "https://abc/123.html"
			metaUpdated string = "2023-03-19T14:56:59+08:00"
		)
		It("create extend data should succeed", func() {
			var err error
			root, err := ctrl.LoadRootEntry(context.TODO())
			Expect(err).Should(BeNil())
			grp, err = ctrl.CreateEntry(context.TODO(), root.ID, types.EntryAttr{
				Name: "test-feed-group",
				Kind: types.GroupKind,
			})
			Expect(err).Should(BeNil())
			grpFile, err = ctrl.CreateEntry(context.TODO(), root.ID, types.EntryAttr{
				Name: "test-feed-group-file",
				Kind: types.RawKind,
			})
			Expect(err).Should(BeNil())

			err = entryStore.UpdateEntryExtendData(context.TODO(), grp.ID, types.ExtendData{
				Properties: types.Properties{
					Fields: map[string]types.PropertyItem{
						attrSourcePluginPrefix + "site_url":  {Value: siteUrl},
						attrSourcePluginPrefix + "site_name": {Value: siteName},
						attrSourcePluginPrefix + "feed_url":  {Value: feedUrl},
					},
				},
			})
			Expect(err).Should(BeNil())
			err = entryStore.UpdateEntryExtendData(context.TODO(), grpFile.ID, types.ExtendData{
				Properties: types.Properties{
					Fields: map[string]types.PropertyItem{
						rssPostMetaID:        {Value: metaId},
						rssPostMetaLink:      {Value: metaLink},
						rssPostMetaTitle:     {Value: metaTitle},
						rssPostMetaUpdatedAt: {Value: metaUpdated},
					},
				},
			})
			Expect(err).Should(BeNil())
		})
		It("Enable should succeed", func() {
			err := ctrl.EnableGroupFeed(context.TODO(), grp.ID, "test-feed")
			Expect(err).Should(BeNil())
			_, err = entryStore.GetEntryLabels(context.TODO(), grp.ID)
			Expect(err).Should(BeNil())
		})
		It("create doc should succeed", func() {
			var err error
			err = entryStore.SaveDocument(context.TODO(), &types.Document{
				OID:           grpFile.ID,
				Name:          grpFile.Name,
				ParentEntryID: grp.ID,
				Content:       "this is content",
				Summary:       "this is summary",
			})
			Expect(err).Should(BeNil())
		})
		It("get docs by feed should succeed", func() {
			feed, err := ctrl.GetDocumentsByFeed(context.TODO(), "test-feed", 1)
			Expect(err).Should(BeNil())
			Expect(feed.FeedUrl).Should(Equal(feedUrl))
			Expect(feed.SiteUrl).Should(Equal(siteUrl))
			Expect(feed.SiteName).Should(Equal(siteName))
			Expect(feed.GroupName).Should(Equal(grp.Name))
			Expect(len(feed.Documents)).Should(Equal(1))
			Expect(feed.Documents[0].ID).Should(Equal(metaId))
			Expect(feed.Documents[0].Link).Should(Equal(metaLink))
			Expect(feed.Documents[0].Title).Should(Equal(metaTitle))
			Expect(feed.Documents[0].UpdatedAt).Should(Equal(metaUpdated))
			Expect(feed.Documents[0].Document.Content).Should(Equal("this is content"))
			Expect(feed.Documents[0].Document.Summary).Should(Equal("this is summary"))
		})
	})
})
