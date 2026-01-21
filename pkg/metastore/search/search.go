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

	"github.com/basenana/nanafs/pkg/types"
	"gorm.io/gorm"
)

func IndexDocument(ctx context.Context, db *gorm.DB, namespace string, document *types.IndexDocument) error {
	switch db.Dialector.Name() {
	case "sqlite":
		return SqliteIndexDocument(ctx, db, namespace, document)
	case "postgres":
		return PostgresIndexDocument(ctx, db, namespace, document)
	default:
		return fmt.Errorf("unknown dialector %s", db.Dialector.Name())
	}
}

func QueryLanguage(ctx context.Context, db *gorm.DB, namespace, query string) ([]*types.IndexDocument, error) {
	switch db.Dialector.Name() {
	case "sqlite":
		return SqliteQueryLanguage(ctx, db, namespace, query)
	case "postgres":
		return PostgresQueryLanguage(ctx, db, namespace, query)
	default:
		return nil, fmt.Errorf("unknown dialector %s", db.Dialector.Name())
	}
}

func DeleteDocument(ctx context.Context, db *gorm.DB, namespace string, id int64) error {
	switch db.Dialector.Name() {
	case "sqlite":
		return SqliteDeleteDocument(ctx, db, namespace, id)
	case "postgres":
		return PostgresDeleteDocument(ctx, db, namespace, id)
	default:
		return fmt.Errorf("unknown dialector %s", db.Dialector.Name())
	}
}
