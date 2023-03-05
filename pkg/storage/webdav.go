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
	"io"
)

type webdav struct{}

var _ Storage = &webdav{}

func (w webdav) ID() string {
	//TODO implement me
	panic("implement me")
}

func (w webdav) Get(ctx context.Context, key int64, idx, offset int64) (io.ReadCloser, error) {
	//TODO implement me
	panic("implement me")
}

func (w webdav) Put(ctx context.Context, key int64, idx, offset int64, in io.Reader) error {
	//TODO implement me
	panic("implement me")
}

func (w webdav) Delete(ctx context.Context, key int64) error {
	//TODO implement me
	panic("implement me")
}

func (w webdav) Head(ctx context.Context, key int64, idx int64) (Info, error) {
	//TODO implement me
	panic("implement me")
}
