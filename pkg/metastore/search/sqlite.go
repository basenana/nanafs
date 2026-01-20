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
	d.ChangedAt = document.ChangedAt
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
		if err := tx.Create(model).Error; err != nil {
			return err
		}
		// Insert into FTS5 virtual table (use rowid for content-linked FTS5)
		return tx.Exec(`INSERT INTO documents_fts(rowid, title, content) VALUES (?, ?, ?)`,
			document.ID, document.Title, document.Content).Error
	})
}

func SqliteQueryLanguage(ctx context.Context, db *gorm.DB, namespace, query string) ([]*types.IndexDocument, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []*types.IndexDocument{}, nil
	}

	// Use FTS5 MATCH for full-text search
	var results []SqliteDocument

	err := db.WithContext(ctx).
		Table("documents").
		Where("namespace = ?", namespace).
		Where("id IN (SELECT rowid FROM documents_fts WHERE documents_fts MATCH ?)", query).
		Find(&results).Error

	if err != nil {
		return nil, err
	}

	var docs []*types.IndexDocument
	for _, r := range results {
		docs = append(docs, r.To())
	}
	return docs, nil
}
