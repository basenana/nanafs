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
	"runtime/trace"
)

const (
	OSSStorage = "oss"
)

type aliyunOSSStorage struct {
}

var _ Storage = &aliyunOSSStorage{}

func (a *aliyunOSSStorage) ID() string {
	//TODO implement me
	panic("implement me")
}

func (a *aliyunOSSStorage) Get(ctx context.Context, key, idx int64) (io.ReadCloser, error) {
	defer trace.StartRegion(ctx, "storage.oss.Get").End()
	//TODO implement me
	panic("implement me")
}

func (a *aliyunOSSStorage) Put(ctx context.Context, key, idx int64, dataReader io.Reader) error {
	defer trace.StartRegion(ctx, "storage.oss.Put").End()
	//TODO implement me
	panic("implement me")
}

func (a *aliyunOSSStorage) Delete(ctx context.Context, key int64) error {
	defer trace.StartRegion(ctx, "storage.oss.Delete").End()
	//TODO implement me
	panic("implement me")
}

func (a *aliyunOSSStorage) Head(ctx context.Context, key int64, idx int64) (Info, error) {
	defer trace.StartRegion(ctx, "storage.oss.Head").End()
	//TODO implement me
	panic("implement me")
}
