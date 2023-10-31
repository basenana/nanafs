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
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"time"
)

var defaultQuery Query

func InitQuery(objectStore metastore.ObjectStore) {
	defaultQuery = &query{
		objectStore: objectStore,
		logger:      logger.NewLogger("ruleQuery"),
	}
}

type Query interface {
	Reset() Query
	Rule(rule types.Rule) Query
	Label(label types.LabelMatch) Query
	Results(ctx context.Context) ([]*types.Metadata, error)
}

type query struct {
	rules       []types.Rule
	labels      []types.LabelMatch
	objectStore metastore.ObjectStore
	logger      *zap.SugaredLogger
}

func (q *query) Reset() Query {
	q.rules = nil
	q.labels = nil
	return &query{objectStore: q.objectStore, logger: q.logger}
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

func (q *query) Results(ctx context.Context) ([]*types.Metadata, error) {
	var (
		startAt = time.Now()
		entries []*types.Metadata
		objects []*types.Object
		err     error
	)
	defer q.logger.Infow("query entries with rules and labels",
		"ruleCount", len(q.rules), "labelCount", len(q.labels), "cost", time.Since(startAt).String())

	if len(q.labels) == 0 {
		q.logger.Warnf("scan all object without lable query")
	}

	objects, err = q.objectStore.ListObjects(ctx, types.Filter{Label: mergeLabelMatch(q.labels)})
	if err != nil {
		q.logger.Errorw("list objects from store with label match failed", "err", err)
		return nil, err
	}

	if len(q.rules) == 0 {
		// labels only
		entries = make([]*types.Metadata, len(objects))
		for i := range objects {
			entries[i] = &objects[i].Metadata
		}
		return entries, nil
	}

	// filter in memory
	entries = make([]*types.Metadata, 0, len(objects)/2)
	for i := range objects {
		obj := objects[i]
		err = q.objectStore.GetObjectExtendData(ctx, obj)
		if err != nil {
			q.logger.Errorw("get object extend data failed", "obj", obj.ID, "err", err)
			return nil, err
		}

		if Filter(mergeRules(q.rules), &obj.Metadata, obj.ExtendData, obj.Labels) {
			entries = append(entries, &obj.Metadata)
		}
	}

	if len(objects) > 100 && len(entries) < len(objects)/2 {
		q.logger.Warnf("slow memory filter performance")
	}
	return entries, nil
}

func Q() Query {
	return defaultQuery.Reset()
}
