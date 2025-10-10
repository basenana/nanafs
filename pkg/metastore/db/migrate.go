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
				)
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
