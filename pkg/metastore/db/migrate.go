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
			ID: "2023040600",
			Migrate: func(db *gorm.DB) error {
				return db.AutoMigrate(
					&SystemInfo{},
					&Object{},
					&ObjectProperty{},
					&ObjectExtend{},
					&ObjectChunk{},
					&PluginData{},
					&Label{},
				)
			},
			Rollback: func(db *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "2023050100",
			Migrate: func(db *gorm.DB) error {
				return db.AutoMigrate(&ScheduledTask{})
			},
			Rollback: func(db *gorm.DB) error {
				return db.Migrator().DropTable(&ScheduledTask{})
			},
		},
		{
			ID: "2023051400",
			Migrate: func(db *gorm.DB) error {
				return db.AutoMigrate(&Workflow{}, &WorkflowJob{}, &Notification{})
			},
			Rollback: func(db *gorm.DB) error {
				return db.Migrator().DropTable(&Workflow{}, &WorkflowJob{}, &Notification{})
			},
		},
		{
			ID: "2023072200",
			Migrate: func(db *gorm.DB) error {
				err := db.AutoMigrate(&Object{})
				_ = db.Exec("UPDATE object SET version=1 WHERE 1=1;")
				return err
			},
			Rollback: func(db *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "2023101400",
			Migrate: func(db *gorm.DB) error {
				return db.AutoMigrate(&Document{})
			},
			Rollback: func(db *gorm.DB) error {
				return nil
			},
		},
		{
			ID: "2023111200",
			Migrate: func(db *gorm.DB) error {
				err := db.AutoMigrate(&ObjectProperty{})
				_ = db.Exec("UPDATE object_property SET encoded=true WHERE 1=1;")
				err = db.AutoMigrate(&ObjectURI{})
				return err
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
