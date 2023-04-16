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

package storage

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/config"
	"io"
)

type Info struct {
	Key  string
	Size int64
}

type Storage interface {
	ID() string
	Get(ctx context.Context, key, idx int64) (io.ReadCloser, error)
	Put(ctx context.Context, key, idx int64, dataReader io.Reader) error
	Delete(ctx context.Context, key int64) error
	Head(ctx context.Context, key int64, idx int64) (Info, error)
}

func NewStorage(storageID, storageType string, cfg config.Storage) (Storage, error) {
	switch storageType {
	case MinioStorage:
		return newMinioStorage(storageID, cfg.MinIO)
	case LocalStorage:
		return newLocalStorage(storageID, cfg.LocalDir)
	case MemoryStorage:
		return newMemoryStorage(storageID), nil
	default:
		return nil, fmt.Errorf("unknow storage id: %s", storageID)
	}
}

var maxConcurrentUploads = make(chan struct{}, 5)
