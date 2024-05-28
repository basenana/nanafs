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

package document

import (
	"context"
	"errors"

	_ "github.com/blevesearch/bleve/v2/analysis/analyzer/custom"
	"go.uber.org/zap"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/document/indexer"

	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
)

const (
	QueryDialectBleve = indexer.QueryDialectBleve
)

type Indexer struct {
	indexer.Indexer
	recorder metastore.DEntry
	logger   *zap.SugaredLogger
}

func NewDocumentIndexer(recorder metastore.DEntry, indexArgs map[string]string) (*Indexer, error) {
	idx, err := indexer.NewBleveIndexer(recorder, indexArgs)
	if err != nil {
		return nil, err
	}
	index := &Indexer{
		Indexer:  idx,
		recorder: recorder,
		logger:   logger.NewLogger("indexer"),
	}
	rebuild, err := idx.NeedRebuild(context.TODO())
	if err != nil {
		return nil, err
	}
	if rebuild {
		go index.rebuild()
	}
	return index, nil
}

func (i *Indexer) rebuild() {
	i.logger.Infow("rebuild index")
	ctx := context.Background()
	allDoc, err := i.recorder.ListDocument(ctx, types.DocFilter{ParentID: 0}, nil) // list all
	if err != nil {
		i.logger.Errorw("rebuild index failed: list all document failed", "err", err)
		return
	}
	for j := range allDoc {
		doc := allDoc[j]
		if err = i.Index(ctx, doc); err != nil {
			i.logger.Errorw("rebuild index failed: index document failed", "doc", doc.ID, "err", err)
		}
	}
	i.logger.Infow("rebuild index finish")
}

var (
	indexConfigKeyMapping = map[string]string{
		"localIndexerDir": "index.local_indexer_dir",
		"jiebaDictFile":   "index.jieba_dict_file",
	}
)

func buildIndexConfigArgs(cfg config.Loader) (bool, map[string]string, error) {
	indexConfig := make(map[string]string)

	enable, err := cfg.GetSystemConfig(context.Background(), config.DocConfigGroup, "index.enable").Bool()
	if err != nil {
		if errors.Is(err, config.ErrNotConfigured) {
			return false, indexConfig, nil
		}
		return false, indexConfig, err
	}

	if !enable {
		return false, indexConfig, nil
	}

	for configKey, configName := range indexConfigKeyMapping {
		indexConfig[configKey], err = cfg.GetSystemConfig(context.Background(), config.DocConfigGroup, configName).String()
		if err != nil && !errors.Is(err, config.ErrNotConfigured) {
			return false, indexConfig, err
		}
	}

	return enable, indexConfig, nil
}
