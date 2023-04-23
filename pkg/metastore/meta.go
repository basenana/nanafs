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

func NewMetaStorage(metaType string, meta config.Meta) (Meta, error) {
	switch metaType {
	case MemoryMeta:
		meta.Path = ":memory:"
		return newSqliteMetaStore(meta)
	case SqliteMeta:
		return newSqliteMetaStore(meta)
	case PostgresMeta:
		return newPostgresMetaStore(meta)
	default:
		return nil, fmt.Errorf("unknow meta store type: %s", metaType)
	}
}
