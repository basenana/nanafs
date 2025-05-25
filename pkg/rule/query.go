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
	"time"

	"go.uber.org/zap"

	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
)

var defaultQuery Query

func InitQuery(entryStore metastore.EntryStore) {
	defaultQuery = &query{
		store:  entryStore,
		logger: logger.NewLogger("ruleQuery"),
	}
}

type Query interface {
	Reset(namesapce string) Query
	Rule(rule types.Rule) Query
	Label(label types.LabelMatch) Query
	Results(ctx context.Context) ([]*types.Entry, error)
}

type query struct {
	rules     []types.Rule
	labels    []types.LabelMatch
	namespace string
	store     metastore.EntryStore
	logger    *zap.SugaredLogger
}

func (q *query) Reset(namespace string) Query {
	q.rules = nil
	q.labels = nil
	return &query{store: q.store, namespace: namespace, logger: q.logger}
}

func (q *query) Rule(rule types.Rule) Query {
	q.rules = append(q.rules, rule)
	if lm := ruleLabelMatch(rule); lm != nil {
		q.labels = append(q.labels, *lm)
	}
	return q
}

func (q *query) Label(label types.LabelMatch) Query {
	q.labels = append(q.labels, label)
	return q
}

func (q *query) Results(ctx context.Context) ([]*types.Entry, error) {
	var (
		startAt = time.Now()
		entries []*types.Entry
		err     error
	)
	defer func() {
		cost := time.Since(startAt).String()
		q.logger.Infow("query entries with rules and labels",
			"ruleCount", len(q.rules), "labelCount", len(q.labels), "cost", cost)
	}()

	if len(q.labels) == 0 {
		q.logger.Warnf("scan all entries without lable query")
	}

	var entriesIt metastore.EntryIterator
	entriesIt, err = q.store.FilterEntries(ctx, q.namespace, types.Filter{Label: mergeLabelMatch(q.labels)})
	if err != nil {
		q.logger.Errorw("list entries from store with label match failed", "err", err)
		return nil, err
	}

	if len(q.rules) == 0 {
		// labels only
		for entriesIt.HasNext() {
			entries = append(entries, entriesIt.Next())
		}
		return entries, nil
	}

	// filter in memory
	for entriesIt.HasNext() {
		en := entriesIt.Next()
		properties, err := q.store.ListEntryProperties(ctx, en.Namespace, en.ID)
		if err != nil {
			q.logger.Errorw("get entry extend data failed", "entry", en.ID, "err", err)
			return nil, err
		}

		labels, err := q.store.GetEntryLabels(ctx, en.Namespace, en.ID)
		if err != nil {
			q.logger.Errorw("get entry labels failed", "entry", en.ID, "err", err)
			return nil, err
		}

		if Filter(mergeRules(q.rules), en, &properties, &labels) {
			entries = append(entries, en)
		}
	}
	return entries, nil
}

func Q(namespace string) Query {
	return defaultQuery.Reset(namespace)
}
