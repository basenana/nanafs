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

package config

import (
	"context"
	"fmt"
	"sync"
)

type CMDB interface {
	GetConfigValue(ctx context.Context, group, name string) (string, error)
	SetConfigValue(ctx context.Context, group, name, value string) error
}

type fakeCmdb struct {
	db  map[string]string
	mux sync.Mutex
}

func (f *fakeCmdb) GetConfigValue(ctx context.Context, group, name string) (string, error) {
	f.mux.Lock()
	defer f.mux.Unlock()

	data, ok := f.db[f.key(group, name)]
	if !ok {
		return "", fmt.Errorf("mock: no record")
	}
	return data, nil
}

func (f *fakeCmdb) SetConfigValue(ctx context.Context, group, name, value string) error {
	f.mux.Lock()
	defer f.mux.Unlock()
	f.db[f.key(group, name)] = value
	return nil
}

func (f *fakeCmdb) key(group, name string) string {
	return fmt.Sprintf("%s.%s", group, name)
}

func NewFakeCmdb() CMDB {
	return &fakeCmdb{db: map[string]string{}}
}
