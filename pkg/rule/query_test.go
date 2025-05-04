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

package rule

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"strings"
)

var _ = Describe("TestQuery", func() {

	ctx := context.Background()

	memMeta, err := metastore.NewMetaStorage(metastore.MemoryMeta, config.Meta{})
	gomega.Expect(err).Should(gomega.BeNil())

	InitQuery(memMeta.(metastore.EntryStore))
	err = mockedObjectForQuery(ctx, memMeta)
	gomega.Expect(err).Should(gomega.BeNil())

	doQuery := func(rules []types.Rule, labelMatches []types.LabelMatch) []*types.Entry {
		q := Q()
		for i := range rules {
			q = q.Rule(rules[i])
		}

		for i := range labelMatches {
			q = q.Label(labelMatches[i])
		}

		entries, err := q.Results(ctx)
		if err != nil {
			return nil
		}
		return entries
	}

	Context("test filter by labels", func() {
		It("test-filter-by-label-1", func() {
			got := doQuery(nil, []types.LabelMatch{{Include: []types.Label{{Key: "test.nanafs.labels1", Value: "label_value"}}}})
			gomega.Expect(isMatchAllWanted([]int64{10001, 10002}, got)).Should(gomega.BeNil())
		})
		It("test-filter-by-label-2.1", func() {
			got := doQuery(nil, []types.LabelMatch{{Include: []types.Label{
				{Key: "test.nanafs.labels1", Value: "label_value"},
				{Key: "test.nanafs.labels2", Value: "label_value"}}}})
			gomega.Expect(isMatchAllWanted([]int64{10001}, got)).Should(gomega.BeNil())
		})
		It("test-filter-by-label-2.2", func() {
			got := doQuery(nil, []types.LabelMatch{
				{Include: []types.Label{{Key: "test.nanafs.labels1", Value: "label_value"}}},
				{Include: []types.Label{{Key: "test.nanafs.labels2", Value: "label_value"}}},
			})
			gomega.Expect(isMatchAllWanted([]int64{10001}, got)).Should(gomega.BeNil())
		})
	})

	Context("test filter by labels exclude", func() {
		It("test-filter-by-label-exclude-3.1", func() {
			got := doQuery(nil, []types.LabelMatch{{Include: []types.Label{{Key: "test.nanafs.labels1", Value: "label_value"}}, Exclude: []string{"test.nanafs.labels2"}}})
			gomega.Expect(isMatchAllWanted([]int64{10002}, got)).Should(gomega.BeNil())
		})
		It("test-filter-by-label-exclude-3.2", func() {
			got := doQuery(nil, []types.LabelMatch{
				{Exclude: []string{"test.nanafs.labels2"}},
				{Include: []types.Label{{Key: "test.nanafs.labels1", Value: "label_value"}}},
			})
			gomega.Expect(isMatchAllWanted([]int64{10002}, got)).Should(gomega.BeNil())
		})
	})

	Context("test filter by labels", func() {
		It("test-filter-by-rule-1", func() {
			got := doQuery([]types.Rule{
				{Labels: &types.LabelMatch{Include: []types.Label{{Key: "test.nanafs.labels1", Value: "label_value"}}}},
				{Operation: types.RuleOpIn, Column: "properties.customKey1", Value: "customValue,customValue2"},
			}, nil)
			gomega.Expect(isMatchAllWanted([]int64{10001, 10002}, got)).Should(gomega.BeNil())
		})
		// FIXME: label match in memory
		It("test-filter-by-rule-2", func() {
			got := doQuery([]types.Rule{
				{Logic: types.RuleLogicAny, Rules: []types.Rule{
					{Labels: &types.LabelMatch{Include: []types.Label{{Key: "test.nanafs.labels1", Value: "label_value"}}}},
					{Operation: types.RuleOpIn, Column: "properties.customKey1", Value: "customValue,customValue2"},
					{Operation: types.RuleOpEqual, Column: "properties.customKey3", Value: "customValue3"},
				}},
			}, nil)
			gomega.Expect(isMatchAllWanted([]int64{10001, 10002, 10003}, got)).Should(gomega.BeNil())
		})
		It("test-filter-by-rule-3", func() {
			got := doQuery([]types.Rule{
				{Logic: types.RuleLogicAll, Rules: []types.Rule{
					{Labels: &types.LabelMatch{Include: []types.Label{{Key: "test.nanafs.labels3", Value: "label_value3"}}}},
					{Operation: types.RuleOpEqual, Column: "properties.customKey3", Value: "customValue3"},
				}},
			}, nil)
			gomega.Expect(isMatchAllWanted([]int64{10003}, got)).Should(gomega.BeNil())
		})
	})

	Context("test filter by rules and labels", func() {
		It("test-filter-by-rule-and-label-1", func() {
			got := doQuery([]types.Rule{{Operation: types.RuleOpIn, Column: "properties.customKey1", Value: "customValue,customValue2"}},
				[]types.LabelMatch{{Include: []types.Label{{Key: "test.nanafs.labels1", Value: "label_value"}}}})
			gomega.Expect(isMatchAllWanted([]int64{10001, 10002}, got)).Should(gomega.BeNil())
		})
	})
})

func isMatchAllWanted(wants []int64, got []*types.Entry) error {
	wantMap := make(map[int64]struct{})
	for _, wId := range wants {
		wantMap[wId] = struct{}{}
	}

	var (
		errMsg    []string
		gotIdList []int64
	)
	for _, en := range got {
		gotIdList = append(gotIdList, en.ID)
		_, has := wantMap[en.ID]
		if !has {
			errMsg = append(errMsg, fmt.Sprintf("entry %d not wanted", en.ID))
			continue
		}
		delete(wantMap, en.ID)
	}

	if len(wantMap) > 0 {
		enIdList := make([]int64, 0, len(wantMap))
		for wId := range wantMap {
			enIdList = append(enIdList, wId)
		}
		errMsg = append(errMsg, fmt.Sprintf("wanted entry not filterd: %+v", enIdList))
	}

	if len(errMsg) > 0 {
		return fmt.Errorf("want: %v\n got: %v\nmessage: %s", wants, gotIdList, strings.Join(errMsg, ", "))
	}
	return nil
}

func mockedObjectForQuery(ctx context.Context, entryStore metastore.EntryStore) error {
	var objectList = []object{
		{
			Entry: createMetadata(10001),
			Properties: &types.Properties{Fields: map[string]types.PropertyItem{
				"customKey1": {Value: "customValue"},
				"customKey2": {Value: "customValue"},
				"customKey3": {Value: "customValue2"},
			},
			},
			Labels: &types.Labels{Labels: []types.Label{
				{Key: "test.nanafs.labels1", Value: "label_value"},
				{Key: "test.nanafs.labels2", Value: "label_value"},
				{Key: "test.nanafs.labels3", Value: "label_value2"},
			}},
		},
		{
			Entry: createMetadata(10002),
			Properties: &types.Properties{Fields: map[string]types.PropertyItem{
				"customKey1": {Value: "customValue2"},
			},
			},
			Labels: &types.Labels{Labels: []types.Label{
				{Key: "test.nanafs.labels1", Value: "label_value"},
			}},
		},
		{
			Entry: createMetadata(10003),
			Properties: &types.Properties{Fields: map[string]types.PropertyItem{
				"customKey1": {Value: "customValue2"},
				"customKey3": {Value: "customValue3"},
			},
			},
			Labels: &types.Labels{Labels: []types.Label{
				{Key: "test.nanafs.labels3", Value: "label_value3"},
			}},
		},
		{
			Entry:      createMetadata(10004),
			Properties: &types.Properties{Fields: map[string]types.PropertyItem{}},
			Labels:     &types.Labels{Labels: []types.Label{}},
		},
	}

	for _, o := range objectList {
		err := entryStore.CreateEntry(ctx, 0, o.Entry, nil)
		if err != nil {
			return err
		}
		err = entryStore.UpdateEntryProperties(ctx, o.ID, *o.Properties)
		if err != nil {
			return err
		}
		err = entryStore.UpdateEntryLabels(ctx, o.ID, *o.Labels)
		if err != nil {
			return err
		}
	}

	return nil
}

func createMetadata(enID int64) *types.Entry {
	enName := utils.RandStringRunes(50)
	md := types.NewEntry(enName, types.RawKind)
	md.ID = enID
	md.ParentID = 1
	return &md
}
