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
	"fmt"
	"strconv"
	"time"

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

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
)

const (
	QueryDialectBleve = "bleve"
)

type Indexer struct {
	b        bleve.Index
	recorder metastore.DEntry
	cfg      config.Indexer
	logger   *zap.SugaredLogger
}

func NewDocumentIndexer(recorder metastore.DEntry, cfg config.Indexer) (*Indexer, error) {
	b, rebuild, err := openBleveLocalIndexer(cfg)
	if err != nil {
		return nil, err
	}
	index := &Indexer{
		b:        b,
		recorder: recorder,
		cfg:      cfg,
		logger:   logger.NewLogger("indexer"),
	}
	if rebuild {
		go index.rebuild()
	}
	return index, nil
}

func (i *Indexer) Index(ctx context.Context, doc *types.Document) error {
	bd := newBleveDocument(doc)
	err := i.b.Index(bd.ID, bd)
	if err != nil {
		i.logger.Errorw("index document failed", "document", doc.ID, "err", err)
		return err
	}
	return err
}

func (i *Indexer) Delete(ctx context.Context, docID int64) error {
	err := i.b.Delete(int64ToStr(docID))
	if err != nil {
		i.logger.Errorw("delete document index failed", "document", docID, "err", err)
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

	i.logger.Infof("%d matches, showing %d through %d, took %s",
		result.Total, result.Request.From+1, result.Request.From+len(result.Hits), result.Took)

	var (
		docList []*types.Document
		oneDoc  *types.Document
	)
	for _, match := range result.Hits {
		matchId, err := strToInt64(match.ID)
		if err != nil {
			i.logger.Errorw("turn doc id to int64 failed", "document", match.ID, "err", err)
			return nil, err
		}
		oneDoc, err = i.recorder.GetDocument(ctx, matchId)
		if err != nil {
			i.logger.Errorw("get document failed", "document", match.ID, "err", err)
			return nil, err
		}
		docList = append(docList, oneDoc)
	}
	return docList, nil
}

func (i *Indexer) rebuild() {
	i.logger.Infow("rebuild index")
	ctx := context.Background()
	allDoc, err := i.recorder.ListDocument(ctx, 0)
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

type BleveDocument struct {
	ID            string    `json:"id"`
	OID           int64     `json:"oid"`
	Name          string    `json:"name"`
	ParentEntryID int64     `json:"parent_entry_id"`
	Source        string    `json:"source"`
	KeyWords      []string  `json:"keywords,omitempty"`
	Content       string    `json:"content,omitempty"`
	Summary       string    `json:"summary,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	ChangedAt     time.Time `json:"changed_at"`
}

func newBleveDocument(doc *types.Document) *BleveDocument {
	return &BleveDocument{
		ID:            int64ToStr(doc.ID),
		OID:           doc.OID,
		Name:          doc.Name,
		ParentEntryID: doc.ParentEntryID,
		Source:        doc.Source,
		KeyWords:      doc.KeyWords,
		Content:       doc.Content,
		Summary:       doc.Summary,
		CreatedAt:     doc.CreatedAt,
		ChangedAt:     doc.ChangedAt,
	}
}

func (d *BleveDocument) BleveType() string {
	return "document"
}

func openBleveLocalIndexer(cfg config.Indexer) (bleve.Index, bool, error) {
	index, err := bleve.Open(cfg.LocalIndexerDir)
	if err != nil && err != bleve.ErrorIndexPathDoesNotExist {
		return nil, false, err
	}
	if err == nil {
		return index, false, nil
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
	documentMapping.AddFieldMappingsAt("keywords", indexButNoStoreTextField)
	documentMapping.AddFieldMappingsAt("summary", indexButNoStoreTextField)
	documentMapping.AddFieldMappingsAt("created_at", indexButNoStoreDataField)
	documentMapping.AddFieldMappingsAt("changed_at", indexButNoStoreDataField)

	mapping.AddDocumentMapping("document", documentMapping)

	index, err = bleve.NewUsing(cfg.LocalIndexerDir, mapping, upsidedown.Name, boltdb.Name, map[string]interface{}{})
	if err != nil {
		return nil, false, err
	}
	return index, true, nil
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

func int64ToStr(s int64) string {
	return fmt.Sprintf("doc_%d", s)
}

func strToInt64(s string) (int64, error) {
	return strconv.ParseInt(s[4:], 10, 64)
}
