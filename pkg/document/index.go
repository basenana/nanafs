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
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis"
	"github.com/blevesearch/bleve/v2/analysis/char/html"
	"github.com/blevesearch/bleve/v2/analysis/lang/en"
	"github.com/blevesearch/bleve/v2/analysis/token/lowercase"
	"github.com/blevesearch/bleve/v2/analysis/tokenizer/unicode"
	"github.com/blevesearch/bleve/v2/index/upsidedown"
	"github.com/blevesearch/bleve/v2/index/upsidedown/store/boltdb"
	"github.com/blevesearch/bleve/v2/registry"
	"go.uber.org/zap"
	"time"
)

const (
	QueryDialectBleve = "bleve"
)

type Indexer struct {
	b      bleve.Index
	doc    metastore.DocumentRecorder
	cfg    config.Indexer
	logger *zap.SugaredLogger
}

func NewDocumentIndexer(doc metastore.DocumentRecorder, cfg config.Indexer) (*Indexer, error) {
	b, err := openBleveLocalIndexer(cfg)
	if err != nil {
		return nil, err
	}
	return &Indexer{
		b:      b,
		doc:    doc,
		cfg:    cfg,
		logger: logger.NewLogger("indexer"),
	}, nil
}

func (i *Indexer) Index(ctx context.Context, doc *types.Document) error {
	err := i.b.Index(doc.ID, doc)
	if err != nil {
		i.logger.Errorw("index document failed", "document", doc.ID, "err", err)
		return err
	}
	return err
}

func (i *Indexer) Delete(ctx context.Context, doc *types.Document) error {
	err := i.b.Delete(doc.ID)
	if err != nil {
		i.logger.Errorw("delete document index failed", "document", doc.ID, "err", err)
		return err
	}
	return err
}

func (i *Indexer) Query(ctx context.Context, query, dialect string) ([]*types.Document, error) {
	if dialect != QueryDialectBleve {
		return nil, types.ErrUnsupported
	}

	startAt := time.Now()
	i.logger.Infow("search document with query", "query", query)
	defer func() {
		i.logger.Infow("search document with query finish", "query", query, "cost", time.Since(startAt).String())
	}()

	q := bleve.NewQueryStringQuery(query)
	req := bleve.NewSearchRequest(q)
	result, err := i.b.SearchInContext(ctx, req)
	if err != nil {
		i.logger.Errorw("search document failed", "query", query, "err", err)
		return nil, err
	}

	i.logger.Infow(result.String())

	var (
		docList []*types.Document
		oneDoc  *types.Document
	)
	for _, match := range result.Hits {
		oneDoc, err = i.doc.GetDocument(ctx, match.ID)
		if err != nil {
			i.logger.Errorw("get document failed", "document", match.ID, "err", err)
			return nil, err
		}
		docList = append(docList, oneDoc)
	}
	return docList, nil
}

func openBleveLocalIndexer(cfg config.Indexer) (bleve.Index, error) {
	index, err := bleve.Open(cfg.LocalIndexerDir)
	if err != nil && err != bleve.ErrorIndexPathDoesNotExist {
		return nil, err
	}
	if err == nil {
		return index, nil
	}

	mapping := bleve.NewIndexMapping()
	mapping.DefaultType = "document"

	indexButNoStoreNumField := bleve.NewNumericFieldMapping()
	indexButNoStoreNumField.Index = true
	indexButNoStoreNumField.Store = false
	indexButNoStoreNumField.IncludeInAll = false

	indexButNoStoreDataField := bleve.NewDateTimeFieldMapping()
	indexButNoStoreDataField.Index = true
	indexButNoStoreDataField.Store = false
	indexButNoStoreDataField.IncludeInAll = false

	indexButNoStoreTextField := bleve.NewTextFieldMapping()
	indexButNoStoreTextField.Index = true
	indexButNoStoreTextField.Store = false
	indexButNoStoreTextField.IncludeInAll = false

	indexAndStoreTextField := bleve.NewTextFieldMapping()
	indexAndStoreTextField.Index = true
	indexAndStoreTextField.Store = true

	indexAndStoreHtmlField := bleve.NewTextFieldMapping()
	indexAndStoreHtmlField.Index = true
	indexAndStoreHtmlField.Store = true
	indexAndStoreHtmlField.Analyzer = HtmlAnalyzer

	documentMapping := bleve.NewDocumentMapping()
	documentMapping.AddFieldMappingsAt("id", indexButNoStoreNumField)
	documentMapping.AddFieldMappingsAt("oid", indexButNoStoreNumField)
	documentMapping.AddFieldMappingsAt("name", indexAndStoreTextField)
	documentMapping.AddFieldMappingsAt("parent_entry_id", indexButNoStoreNumField)

	documentMapping.AddFieldMappingsAt("source", indexButNoStoreTextField)
	documentMapping.AddFieldMappingsAt("content", indexAndStoreHtmlField)
	documentMapping.AddFieldMappingsAt("key_words", indexButNoStoreTextField)
	documentMapping.AddFieldMappingsAt("summary", indexButNoStoreTextField)
	documentMapping.AddFieldMappingsAt("created_at", indexButNoStoreDataField)
	documentMapping.AddFieldMappingsAt("changed_at", indexButNoStoreDataField)

	mapping.AddDocumentMapping("document", documentMapping)

	index, err = bleve.NewUsing(cfg.LocalIndexerDir, mapping, upsidedown.Name, boltdb.Name, map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	return index, nil
}

const HtmlAnalyzer = "html"

func AnalyzerConstructor(config map[string]interface{}, cache *registry.Cache) (analysis.Analyzer, error) {
	tokenizer, err := cache.TokenizerNamed(unicode.Name)
	if err != nil {
		return nil, err
	}
	toLowerFilter, err := cache.TokenFilterNamed(lowercase.Name)
	if err != nil {
		return nil, err
	}
	stopEnFilter, err := cache.TokenFilterNamed(en.StopName)
	if err != nil {
		return nil, err
	}

	htmlCharFilter, err := cache.CharFilterNamed(html.Name)
	if err != nil {
		return nil, err
	}

	rv := analysis.DefaultAnalyzer{
		Tokenizer: tokenizer,
		TokenFilters: []analysis.TokenFilter{
			toLowerFilter,
			stopEnFilter,
		},
		CharFilters: []analysis.CharFilter{
			htmlCharFilter,
		},
	}
	return &rv, nil
}

func init() {
	registry.RegisterAnalyzer(HtmlAnalyzer, AnalyzerConstructor)
}
