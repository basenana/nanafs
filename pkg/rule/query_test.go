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
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"strings"
)

var _ = Describe("TestQuery", func() {

	ctx := context.Background()

	memMeta, err := metastore.NewMetaStorage(storage.MemoryStorage, config.Meta{})
	gomega.Expect(err).Should(gomega.BeNil())

	InitQuery(memMeta)
	err = mockedObjectForQuery(ctx, memMeta)
	gomega.Expect(err).Should(gomega.BeNil())

	tests := []struct {
		name         string
		rules        []types.Rule
		labelMatches []types.LabelMatch
		wantEntries  []int64
	}{
		{
			name:         "test-filter-by-label-1",
			labelMatches: []types.LabelMatch{{Include: []types.Label{{Key: "test.nanafs.labels1", Value: "label_value"}}}},
			wantEntries:  []int64{10001, 10002},
		},
		{
			name:         "test-filter-by-label-2.1",
			labelMatches: []types.LabelMatch{{Include: []types.Label{{Key: "test.nanafs.labels1", Value: "label_value"}, {Key: "test.nanafs.labels2", Value: "label_value"}}}},
			wantEntries:  []int64{10001},
		},
		{
			name: "test-filter-by-label-2.2",
			labelMatches: []types.LabelMatch{
				{Include: []types.Label{{Key: "test.nanafs.labels1", Value: "label_value"}}},
				{Include: []types.Label{{Key: "test.nanafs.labels2", Value: "label_value"}}},
			},
			wantEntries: []int64{10001},
		},
		{
			name:         "test-filter-by-label-exclude-3.1",
			labelMatches: []types.LabelMatch{{Include: []types.Label{{Key: "test.nanafs.labels1", Value: "label_value"}}, Exclude: []string{"test.nanafs.labels2"}}},
			wantEntries:  []int64{10002},
		},
		{
			name: "test-filter-by-label-exclude-3.2",
			labelMatches: []types.LabelMatch{
				{Exclude: []string{"test.nanafs.labels2"}},
				{Include: []types.Label{{Key: "test.nanafs.labels1", Value: "label_value"}}},
			},
			wantEntries: []int64{10002},
		},
		{
			name:         "test-filter-by-rule-and-label-1",
			rules:        []types.Rule{{Operation: types.RuleOpIn, Column: "properties.fields.customKey1", Value: "customValue, customValue2"}},
			labelMatches: []types.LabelMatch{{Include: []types.Label{{Key: "test.nanafs.labels1", Value: "label_value"}}}},
			wantEntries:  []int64{10001, 10002},
		},
		{
			name: "test-filter-by-rule-2",
			rules: []types.Rule{
				{Labels: &types.LabelMatch{Include: []types.Label{{Key: "test.nanafs.labels1", Value: "label_value"}}}},
				{Operation: types.RuleOpIn, Column: "properties.fields.customKey1", Value: "customValue, customValue2"},
			},
			wantEntries: []int64{10001, 10002},
		},
		{
			name: "test-filter-by-rule-3",
			rules: []types.Rule{
				{Logic: types.RuleLogicAny, Rules: []types.Rule{
					{Labels: &types.LabelMatch{Include: []types.Label{{Key: "test.nanafs.labels1", Value: "label_value"}}}},
					{Operation: types.RuleOpIn, Column: "properties.fields.customKey1", Value: "customValue,customValue2"},
					{Operation: types.RuleOpEqual, Column: "properties.fields.customKey3", Value: "customValue3"},
				}},
			},
			wantEntries: []int64{10001, 10002, 10003},
		},
		{
			name: "test-filter-by-rule-4",
			rules: []types.Rule{
				{Logic: types.RuleLogicAll, Rules: []types.Rule{
					{Labels: &types.LabelMatch{Include: []types.Label{{Key: "test.nanafs.labels1", Value: "label_value2"}}}},
					{Operation: types.RuleOpEqual, Column: "properties.fields.customKey3", Value: "customValue3"},
				}},
			},
			wantEntries: []int64{10003},
		},
	}
	for _, tt := range tests {
		Context(tt.name, func() {
			q := Q()

			for i := range tt.rules {
				q = q.Rule(tt.rules[i])
			}

			for i := range tt.labelMatches {
				q = q.Label(tt.labelMatches[i])
			}

			entries, err := q.Results(ctx)
			gomega.Expect(err).Should(gomega.BeNil())

			err = isMatchAllWanted(tt.wantEntries, entries)
			gomega.Expect(err).Should(gomega.BeNil())
		})
	}
})

func isMatchAllWanted(wants []int64, got []*types.Metadata) error {
	wantMap := make(map[int64]struct{})
	for _, wId := range wants {
		wantMap[wId] = struct{}{}
	}

	var errMsg []string
	for _, en := range got {
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
		return fmt.Errorf(strings.Join(errMsg, ", "))
	}
	return nil
}

func mockedObjectForQuery(ctx context.Context, objectStore metastore.ObjectStore) error {
	var objectList = []*types.Object{
		{
			Metadata: createMetadata(10001),
			ExtendData: &types.ExtendData{
				Properties: types.Properties{Fields: map[string]string{
					"customKey1": "customValue",
					"customKey2": "customValue",
					"customKey3": "customValue2",
				}},
			},
			Labels: &types.Labels{Labels: []types.Label{
				{Key: "test.nanafs.labels1", Value: "label_value"},
				{Key: "test.nanafs.labels2", Value: "label_value"},
				{Key: "test.nanafs.labels3", Value: "label_value2"},
			}},
			ExtendDataChanged: true,
			LabelsChanged:     true,
		},
		{
			Metadata: createMetadata(10002),
			ExtendData: &types.ExtendData{
				Properties: types.Properties{Fields: map[string]string{
					"customKey1": "customValue2",
				}},
			},
			Labels: &types.Labels{Labels: []types.Label{
				{Key: "test.nanafs.labels1", Value: "label_value"},
			}},
			ExtendDataChanged: true,
			LabelsChanged:     true,
		},
		{
			Metadata: createMetadata(10003),
			ExtendData: &types.ExtendData{
				Properties: types.Properties{Fields: map[string]string{
					"customKey1": "customValue2",
					"customKey3": "customValue3",
				}},
			},
			Labels: &types.Labels{Labels: []types.Label{
				{Key: "test.nanafs.labels3", Value: "label_value3"},
			}},
			ExtendDataChanged: true,
			LabelsChanged:     true,
		},
		{
			Metadata:          createMetadata(10004),
			ExtendData:        &types.ExtendData{Properties: types.Properties{Fields: map[string]string{}}},
			Labels:            &types.Labels{Labels: []types.Label{}},
			ExtendDataChanged: true,
			LabelsChanged:     true,
		},
	}

	return objectStore.SaveObjects(ctx, objectList...)
}

func createMetadata(enID int64) types.Metadata {
	enName := utils.RandStringRunes(50)
	md := types.NewMetadata(enName, types.RawKind)
	md.ID = enID
	md.ParentID = 1
	return md
}
