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
	"gorm.io/gorm/clause"
)

type PostgresDocumentModel struct {
	ID        int64    `gorm:"column:id;primaryKey"`
	URI       string   `gorm:"column:uri;index:pgdoc_uri"`
	Namespace string   `gorm:"column:namespace;index:pgdoc_namespace"`
	Title     string   `gorm:"column:title"`
	Content   string   `gorm:"column:content"`
	Token     TsVector `gorm:"column:token;type:tsvector"`
	CreatedAt int64    `gorm:"column:created_at"`
	ChangedAt int64    `gorm:"column:changed_at"`
}

type postgresHighlightResult struct {
	PostgresDocumentModel
	HighlightTitle   string  `gorm:"column:highlight_title"`
	HighlightContent string  `gorm:"column:highlight_content"`
	Rank             float64 `gorm:"column:rank"`
}

func (d *PostgresDocumentModel) TableName() string {
	return "documents"
}

func (d *PostgresDocumentModel) From(document *types.IndexDocument) {
	d.ID = document.ID
	d.URI = document.URI
	d.Title = document.Title
	d.Content = document.Content

	d.CreatedAt = document.CreateAt
	if d.CreatedAt == 0 {
		d.CreatedAt = time.Now().UnixNano()
	}

	d.ChangedAt = time.Now().UnixNano()
}

func (d *PostgresDocumentModel) To() *types.IndexDocument {
	return &types.IndexDocument{
		ID:        d.ID,
		URI:       d.URI,
		Title:     d.Title,
		Content:   d.Content,
		CreateAt:  d.CreatedAt,
		ChangedAt: d.ChangedAt,
	}
}

func PostgresIndexDocument(ctx context.Context, db *gorm.DB, namespace string, document *types.IndexDocument, tokenizer func(string) []string) error {
	if document.ID == 0 || document.URI == "" {
		// The document ID must be specified at the outside.
		return fmt.Errorf("document id is empty")
	}

	model := &PostgresDocumentModel{}
	model.From(document)
	model.Namespace = namespace

	tokenExpr := toTsVectorExpr(document, tokenizer)
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Upsert using GORM ON CONFLICT
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"uri", "title", "content", "created_at", "changed_at"}),
		}).Create(model).Error; err != nil {
			return err
		}
		// Update token using GORM Expr
		return tx.Model(&PostgresDocumentModel{}).Where("id = ?", model.ID).Update("token", gorm.Expr(tokenExpr)).Error
	})
}

func PostgresQueryLanguage(ctx context.Context, db *gorm.DB, namespace, query string) ([]*types.IndexDocument, error) {
	keywords := strings.Fields(query)
	if len(keywords) == 0 {
		return []*types.IndexDocument{}, nil
	}
	tsQuery := strings.Join(keywords, " & ")

	var results []postgresHighlightResult

	headlineTitleExpr := fmt.Sprintf(
		"ts_headline('simple', title, to_tsquery('simple', $1), " +
			"'StartSel=<mark>, StopSel=</mark>, MaxWords=50, MinWords=20')",
	)
	headlineContentExpr := fmt.Sprintf(
		"ts_headline('simple', content, to_tsquery('simple', $1), " +
			"'StartSel=<mark>, StopSel=</mark>, MaxWords=100, MinWords=50, " +
			"FragmentDelimiter= ... ')",
	)

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&PostgresDocumentModel{}).
			Where("namespace = ?", namespace).
			Where("token @@ to_tsquery('simple', ?)", tsQuery).
			Select(
				fmt.Sprintf("*, %s as highlight_title, %s as highlight_content, ts_rank(token, to_tsquery('simple', ?)) as rank",
					headlineTitleExpr, headlineContentExpr),
				tsQuery,
			)

		res = res.Order("rank DESC")

		if page := types.GetPagination(ctx); page != nil {
			res = res.Offset(page.Offset()).Limit(page.Limit())
		}

		res = res.Find(&results)
		return res.Error
	})

	if err != nil {
		return nil, err
	}

	var docs []*types.IndexDocument
	for _, r := range results {
		docs = append(docs, &types.IndexDocument{
			ID:               r.ID,
			URI:              r.URI,
			Title:            r.Title,
			HighlightTitle:   r.HighlightTitle,
			HighlightContent: r.HighlightContent,
			CreateAt:         r.CreatedAt,
			ChangedAt:        r.ChangedAt,
		})
	}
	return docs, nil
}

func PostgresDeleteDocument(ctx context.Context, db *gorm.DB, namespace string, id int64) error {
	result := db.WithContext(ctx).Where("id = ? AND namespace = ?", id, namespace).Delete(&PostgresDocumentModel{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("document not found")
	}
	return nil
}

func PostgresUpdateDocumentURI(ctx context.Context, db *gorm.DB, namespace string, id int64, uri string) error {
	result := db.WithContext(ctx).Model(&PostgresDocumentModel{}).
		Where("id = ? AND namespace = ?", id, namespace).
		Update("uri", uri)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("document not found")
	}
	return nil
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

func toTsVectorExpr(document *types.IndexDocument, tokenizer func(string) []string) string {
	titleTokens := tokenizer(document.Title)
	contentTokens := tokenizer(document.Content)

	return fmt.Sprintf("setweight(to_tsvector('simple', '%s'), 'A') || setweight(to_tsvector('simple', '%s'), 'B')",
		strings.Join(titleTokens, " "),
		strings.Join(contentTokens, " "),
	)
}
