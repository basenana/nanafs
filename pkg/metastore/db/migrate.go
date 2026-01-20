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

package db

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func buildMigrations() []*gormigrate.Migration {
	return []*gormigrate.Migration{
		{
			ID: "2025062100",
			Migrate: func(db *gorm.DB) error {
				return db.AutoMigrate(
					&SystemInfo{},
					&SystemConfig{},
					&Entry{},
					&Children{},
					&EntryProperty{},
					&EntryChunk{},
					&Project{},
					&ScheduledTask{},
					&Workflow{},
					&WorkflowJob{},
					&WorkflowJobData{},
					&Notification{},
				)
			},
			Rollback: func(db *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "2026011800",
			Migrate: func(db *gorm.DB) error {
				err := db.AutoMigrate(&Children{})
				if err != nil {
					return err
				}
				sql := `
					UPDATE children
					SET is_group = (
						SELECT e.is_group
						FROM entry e
						WHERE e.id = children.child_id
						  AND e.namespace = children.namespace
					)
					WHERE is_group IS NULL
				`
				return db.Exec(sql).Error
			},
			Rollback: func(db *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "2026011801",
			Migrate: func(db *gorm.DB) error {
				switch db.Dialector.Name() {
				case "sqlite":
					// Create main table for storing documents
					mainTable := `CREATE TABLE IF NOT EXISTS documents (
						id INTEGER PRIMARY KEY,
						uri TEXT NOT NULL,
						namespace TEXT NOT NULL,
						title TEXT,
						content TEXT,
						created_at INTEGER,
						changed_at INTEGER
					)`
					if err := db.Exec(mainTable).Error; err != nil {
						return err
					}
					// Create composite index on (uri, namespace) for efficient lookups
					if err := db.Exec(`CREATE INDEX IF NOT EXISTS sltdoc_uri_ns_idx ON documents(uri, namespace)`).Error; err != nil {
						return err
					}
					// Create FTS5 virtual table for full-text search with Unicode tokenization
					ftsTable := `CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(
						title, content,
						content='documents',
						content_rowid='id',
						tokenize='unicode61'
					)`
					return db.Exec(ftsTable).Error
				case "postgres":
					sql := `CREATE TABLE IF NOT EXISTS documents (
						id BIGSERIAL PRIMARY KEY,
						uri TEXT NOT NULL,
						namespace TEXT NOT NULL,
						title TEXT,
						content TEXT,
						token TSVECTOR,
						created_at BIGINT,
						changed_at BIGINT
					)`
					if err := db.Exec(sql).Error; err != nil {
						return err
					}
					// Create indexes for uri and namespace
					if err := db.Exec(`CREATE INDEX IF NOT EXISTS pgdoc_uri ON documents(uri)`).Error; err != nil {
						return err
					}
					if err := db.Exec(`CREATE INDEX IF NOT EXISTS pgdoc_namespace ON documents(namespace)`).Error; err != nil {
						return err
					}
					return db.Exec(`CREATE INDEX IF NOT EXISTS documents_token_idx
							ON documents USING GIN (token)`).Error
				}
				return nil
			},
			Rollback: func(db *gorm.DB) error {
				return nil
			},
		},
	}
}

func Migrate(db *gorm.DB) error {
	m := gormigrate.New(db, gormigrate.DefaultOptions, buildMigrations())
	err := m.Migrate()
	return err
}
