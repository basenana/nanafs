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
	}
}

func Migrate(db *gorm.DB) error {
	m := gormigrate.New(db, gormigrate.DefaultOptions, buildMigrations())
	err := m.Migrate()
	return err
}
