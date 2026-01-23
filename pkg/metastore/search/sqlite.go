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
	"fmt"
	"strings"
	"time"

	"github.com/basenana/nanafs/pkg/types"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SqliteDocument struct {
	ID        int64  `gorm:"column:id;primaryKey"`
	URI       string `gorm:"column:uri;index:sltdoc_namespace"`
	Namespace string `gorm:"column:namespace;index:sltdoc_namespace"`
	Title     string `gorm:"column:title"`
	Content   string `gorm:"column:content"`
	CreateAt  int64  `gorm:"column:created_at"`
	ChangedAt int64  `gorm:"column:changed_at"`
}

type sqliteHighlightResult struct {
	ID              int64  `gorm:"column:id"`
	URI             string `gorm:"column:uri"`
	Title           string `gorm:"column:title"`
	Content         string `gorm:"column:content"`
	CreateAt        int64  `gorm:"column:created_at"`
	ChangedAt       int64  `gorm:"column:changed_at"`
	HighlightTitle  string `gorm:"column:highlight_title"`
	HighlightContent string `gorm:"column:highlight_content"`
}

func (d *SqliteDocument) TableName() string {
	return "documents"
}

func (d *SqliteDocument) From(document *types.IndexDocument) {
	d.ID = document.ID
	d.URI = document.URI
	d.Title = document.Title
	d.Content = document.Content
	d.CreateAt = document.CreateAt
	if d.CreateAt == 0 {
		d.CreateAt = time.Now().UnixNano()
	}
	d.ChangedAt = time.Now().UnixNano()
}

func (d *SqliteDocument) To() *types.IndexDocument {
	return &types.IndexDocument{
		ID:        d.ID,
		URI:       d.URI,
		Title:     d.Title,
		Content:   d.Content,
		CreateAt:  d.CreateAt,
		ChangedAt: d.ChangedAt,
	}
}

func SqliteIndexDocument(ctx context.Context, db *gorm.DB, namespace string, document *types.IndexDocument) error {
	if document.ID == 0 || document.URI == "" {
		return fmt.Errorf("document id is empty")
	}

	model := &SqliteDocument{}
	model.From(document)
	model.Namespace = namespace

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Upsert main table using GORM ON CONFLICT
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"uri", "title", "content", "created_at", "changed_at"}),
		}).Create(model).Error; err != nil {
			return err
		}
		// Delete existing FTS5 entry first (contentless FTS5 requires manual management)
		if err := tx.Exec(`DELETE FROM documents_fts WHERE rowid = ?`, document.ID).Error; err != nil {
			return err
		}
		// Insert into FTS5 virtual table
		return tx.Exec(`INSERT INTO documents_fts(rowid, title, content) VALUES (?, ?, ?)`,
			document.ID, document.Title, document.Content).Error
	})
}

func SqliteQueryLanguage(ctx context.Context, db *gorm.DB, namespace, query string) ([]*types.IndexDocument, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []*types.IndexDocument{}, nil
	}

	var results []sqliteHighlightResult

	err := db.WithContext(ctx).
		Table("documents").
		Select(`documents.id, documents.uri, documents.title, documents.content,
			documents.created_at, documents.changed_at,
			highlight(documents_fts, 0, '<mark>', '</mark>') as highlight_title,
			highlight(documents_fts, 1, '<mark>', '</mark>') as highlight_content`).
		Joins("JOIN documents_fts ON documents.id = documents_fts.rowid").
		Where("documents.namespace = ?", namespace).
		Where("documents_fts MATCH ?", query).
		Scan(&results).Error

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
			CreateAt:         r.CreateAt,
			ChangedAt:        r.ChangedAt,
		})
	}
	return docs, nil
}

func SqliteDeleteDocument(ctx context.Context, db *gorm.DB, namespace string, id int64) error {
	// Delete from FTS5 virtual table first (outside transaction as FTS5 doesn't fully support transactions)
	if err := db.WithContext(ctx).Exec(`DELETE FROM documents_fts WHERE rowid = ?`, id).Error; err != nil {
		return err
	}
	// Delete from main table with namespace check using GORM
	result := db.WithContext(ctx).Where("id = ? AND namespace = ?", id, namespace).Delete(&SqliteDocument{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("document not found")
	}
	return nil
}

func SqliteUpdateDocumentURI(ctx context.Context, db *gorm.DB, namespace string, id int64, uri string) error {
	result := db.WithContext(ctx).Model(&SqliteDocument{}).
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
