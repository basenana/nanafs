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
)

type Storage interface {
	ID() string
	Get(ctx context.Context, key, idx, offset int64, dest []byte) (int64, error)
	Put(ctx context.Context, key, idx, offset int64, data []byte) error
	Delete(ctx context.Context, key int64) error
	Head(ctx context.Context, key int64, idx int64) (Info, error)
}
