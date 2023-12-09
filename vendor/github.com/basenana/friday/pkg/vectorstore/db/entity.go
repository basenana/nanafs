/*
 * Copyright 2023 Friday Author.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package db

import (
	"gorm.io/gorm"
)

type Entity struct {
	*gorm.DB
}

func NewDbEntity(db *gorm.DB, migrate func(db *gorm.DB) error) (*Entity, error) {
	ent := &Entity{DB: db}
	if err := migrate(db); err != nil {
		return nil, err
	}
	return ent, nil
}
