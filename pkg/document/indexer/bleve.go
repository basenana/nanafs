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

package indexer

import (
	"context"
	"errors"
	"fmt"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis"
	_ "github.com/blevesearch/bleve/v2/analysis/analyzer/custom"
	"github.com/blevesearch/bleve/v2/analysis/char/html"
	"github.com/blevesearch/bleve/v2/analysis/lang/en"
	"github.com/blevesearch/bleve/v2/analysis/token/lowercase"
	"github.com/blevesearch/bleve/v2/analysis/tokenizer/unicode"
	"github.com/blevesearch/bleve/v2/index/upsidedown"
	"github.com/blevesearch/bleve/v2/index/upsidedown/store/boltdb"
	"github.com/blevesearch/bleve/v2/registry"
	bquery "github.com/blevesearch/bleve/v2/search/query"
	jiebatk "github.com/hyponet/jiebago/tokenizers"
	"go.uber.org/zap"
	"strconv"
	"time"
)

type Bleve struct {
	b        bleve.Index
	recorder metastore.DEntry
	rebuild  bool
	logger   *zap.SugaredLogger
}

func NewBleveIndexer(recorder metastore.DEntry, indexArgs map[string]string) (*Bleve, error) {
	idx, rebuild, err := openBleveLocalIndexer(
		indexArgs["localIndexerDir"],
		indexArgs["jiebaDictFile"],
	)
	if err != nil {
		return nil, err
	}
	return &Bleve{
		b:        idx,
		recorder: recorder,
		rebuild:  rebuild,
		logger:   logger.NewLogger("bleveIndexer"),
	}, nil
}

func (b Bleve) Index(ctx context.Context, doc *types.Document) error {
	bd := newBleveDocument(doc)
	return b.b.Index(bd.ID, bd)
}

func (b Bleve) Delete(ctx context.Context, docID int64) error {
	return b.b.Delete(int64ToStr(docID))
}

func (b Bleve) Query(ctx context.Context, query, dialect string) ([]*types.Document, error) {
	if dialect != QueryDialectBleve {
		return nil, types.ErrUnsupported
	}

	startAt := time.Now()
	b.logger.Infow("search document with query", "query", query)
	defer func() {
		b.logger.Infow("search document with query finish", "query", query, "cost", time.Since(startAt).String())
	}()

	bleve.NewConjunctionQuery()

	var q bquery.Query
	q = bleve.NewQueryStringQuery(query)
	ns := types.GetNamespace(ctx)
	if ns.String() != types.DefaultNamespaceValue {
		nsQuery := bleve.NewPhraseQuery([]string{ns.String()}, "namespace")
		q = bleve.NewConjunctionQuery(q, nsQuery)
	}

	req := bleve.NewSearchRequest(q)
	result, err := b.b.SearchInContext(ctx, req)
	if err != nil {
		b.logger.Errorw("search document failed", "query", query, "err", err)
		return nil, err
	}

	b.logger.Infof("%d matches took %s", result.Total, result.Took)

	var (
		docList []*types.Document
		oneDoc  *types.Document
	)
	for _, match := range result.Hits {
		matchId, err := strToInt64(match.ID)
		if err != nil {
			b.logger.Errorw("turn doc id to int64 failed", "document", match.ID, "err", err)
			return nil, err
		}
		oneDoc, err = b.recorder.GetDocument(ctx, matchId)
		if err != nil {
			b.logger.Errorw("get document failed", "document", match.ID, "err", err)
			return nil, err
		}
		docList = append(docList, oneDoc)
	}
	return docList, nil
}

func (b Bleve) NeedRebuild(ctx context.Context) (bool, error) {
	return b.rebuild, nil
}

type BleveDocument struct {
	ID            string    `json:"id"`
	OID           int64     `json:"oid"`
	Name          string    `json:"name"`
	Namespace     string    `json:"namespace"`
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
		Namespace:     doc.Namespace,
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

func openBleveLocalIndexer(localIndexerDir, dictFile string) (bleve.Index, bool, error) {
	index, err := bleve.Open(localIndexerDir)
	if err != nil && !errors.Is(err, bleve.ErrorIndexPathDoesNotExist) {
		return nil, false, err
	}
	if err == nil {
		return index, false, nil
	}

	mapping := bleve.NewIndexMapping()

	if dictFile != "" {
		err = mapping.AddCustomTokenizer("jieba", map[string]interface{}{
			"file": dictFile,
			"hmm":  true,
			"type": jiebatk.Name,
		})
		if err != nil {
			return nil, false, err
		}

		err = mapping.AddCustomAnalyzer("jieba",
			map[string]interface{}{
				"type":      "custom",
				"tokenizer": "jieba",
				"token_filters": []string{
					"possessive_en",
					"to_lower",
					"stop_en",
				},
			})
		if err != nil {
			return nil, false, err
		}
		mapping.DefaultAnalyzer = "jieba"
	}

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
	documentMapping.AddFieldMappingsAt("namespace", indexAndStoreTextField)
	documentMapping.AddFieldMappingsAt("parent_entry_id", indexButNoStoreNumField)

	documentMapping.AddFieldMappingsAt("source", indexButNoStoreTextField)
	documentMapping.AddFieldMappingsAt("content", indexAndStoreHtmlField)
	documentMapping.AddFieldMappingsAt("keywords", indexButNoStoreTextField)
	documentMapping.AddFieldMappingsAt("summary", indexButNoStoreTextField)
	documentMapping.AddFieldMappingsAt("created_at", indexButNoStoreDataField)
	documentMapping.AddFieldMappingsAt("changed_at", indexButNoStoreDataField)

	mapping.AddDocumentMapping("document", documentMapping)

	index, err = bleve.NewUsing(localIndexerDir, mapping, upsidedown.Name, boltdb.Name, map[string]interface{}{})
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
