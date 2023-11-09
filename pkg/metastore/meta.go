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

package metastore

import (
	"fmt"
	"github.com/basenana/nanafs/config"
)

func NewMetaStorage(metaType string, meta config.Meta) (m Meta, err error) {
	switch metaType {
	case MemoryMeta:
		meta.Path = ":memory:"
		m, err = newSqliteMetaStore(meta)
	case SqliteMeta:
		m, err = newSqliteMetaStore(meta)
	case PostgresMeta:
		m, err = newPostgresMetaStore(meta)
	default:
		err = fmt.Errorf("unknow meta store type: %s", metaType)
	}
	if err != nil {
		logOperationError("init", err)
		return nil, err
	}
	if disableMetrics {
		return m, nil
	}
	return instrumentalStore{store: m}, nil
}
