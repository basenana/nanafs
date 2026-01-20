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

package search

import (
	"context"
	"database/sql/driver"
	"fmt"
	"strings"
	"time"

	"github.com/basenana/nanafs/pkg/types"

	"gorm.io/gorm"
)

type PosgtresDocumentModel struct {
	ID        int64    `gorm:"column:id;primaryKey"`
	URI       string   `gorm:"column:uri;index:pgdoc_uri"`
	Namespace string   `gorm:"column:namespace;index:pgdoc_namespace"`
	Title     string   `gorm:"column:title"`
	Content   string   `gorm:"column:content"`
	Token     TsVector `gorm:"column:token;type:tsvector"`
	CreatedAt int64    `gorm:"column:created_at"`
	ChangedAt int64    `gorm:"column:changed_at"`
}

func (d *PosgtresDocumentModel) From(document *types.IndexDocument) {
	d.ID = document.ID
	d.URI = document.URI
	d.Title = document.Title
	d.Content = document.Content

	d.CreatedAt = document.CreateAt
	if d.CreatedAt == 0 {
		d.CreatedAt = time.Now().UnixNano()
	}

	d.CreatedAt = document.CreateAt
	if d.CreatedAt == 0 {
		d.CreatedAt = time.Now().UnixNano()
	}
}

func (d *PosgtresDocumentModel) To() *types.IndexDocument {
	return &types.IndexDocument{
		ID:        d.ID,
		URI:       d.URI,
		Title:     d.Title,
		Content:   d.Content,
		CreateAt:  d.CreatedAt,
		ChangedAt: d.ChangedAt,
	}
}

func PogstresIndexDocument(ctx context.Context, db *gorm.DB, namespace string, document *types.IndexDocument) error {
	if document.ID == 0 || document.URI == "" {
		// The document ID must be specified at the outside.
		return fmt.Errorf("document id is empty")
	}

	model := &PosgtresDocumentModel{}
	model.From(document)
	model.Namespace = namespace

	var err error
	tokenExpr := toTsVectorExpr(document)
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err = tx.Create(model).Error; err != nil {
			return err
		}
		res := tx.Model(&PosgtresDocumentModel{}).Where("id = ?", model.ID).Update("token", gorm.Expr(tokenExpr))
		if res.Error != nil {
			return res.Error
		}
		return nil
	})
}

func PostgresQueryLanguage(ctx context.Context, db *gorm.DB, namespace, query string) ([]*types.IndexDocument, error) {
	keywords := strings.Fields(query)
	if len(keywords) == 0 {
		return []*types.IndexDocument{}, nil
	}
	tsQuery := strings.Join(keywords, " & ")

	var (
		docModels []PosgtresDocumentModel
		result    []*types.IndexDocument
	)

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&PosgtresDocumentModel{}).
			Where("namespace = ?", namespace).
			Where("token @@ to_tsquery('simple', ?)", tsQuery).
			Select("*, ts_rank(token, to_tsquery('simple', ?)) as rank", tsQuery)

		res = res.Order("rank DESC").Find(&docModels)
		return res.Error
	})

	if err != nil {
		return nil, err
	}

	for _, dm := range docModels {
		result = append(result, dm.To())
	}
	return result, nil
}

type TsVector string

func (t *TsVector) Scan(value interface{}) error {
	if value == nil {
		*t = ""
		return nil
	}
	switch v := value.(type) {
	case []byte:
		*t = TsVector(v)
	case string:
		*t = TsVector(v)
	default:
		return fmt.Errorf("cannot scan type %T into TsVector", value)
	}
	return nil
}

func (t *TsVector) Value() (driver.Value, error) {
	return string(*t), nil
}

func toTsVectorExpr(document *types.IndexDocument) string {
	titleTokens := splitTokens(document.Title)
	contentTokens := splitTokens(document.Content)

	return fmt.Sprintf("setweight(to_tsvector('simple', '%s'), 'A') || setweight(to_tsvector('simple', '%s'), 'B')",
		strings.Join(titleTokens, " "),
		strings.Join(contentTokens, " "),
	)
}
