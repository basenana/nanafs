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

package core

import (
	"context"
	"github.com/basenana/nanafs/pkg/metastore"
)

type DCache interface {
	GetDEntry(ctx context.Context, namespace string, path string) (*DEntry, error)
	FindDEntry(ctx context.Context, namespace string, parentId int64, child string) (*DEntry, error)
	Invalid(entries ...int64)
}

func NewDCache(store metastore.Meta) DCache {
	return &dentry{store: store}
}

type dentry struct {
	store metastore.Meta
}

func (d *dentry) GetDEntry(ctx context.Context, namespace string, path string) (*DEntry, error) {
	//TODO implement me
	panic("implement me")
}

func (d *dentry) FindDEntry(ctx context.Context, namespace string, parentId int64, child string) (*DEntry, error) {
	//TODO implement me
	panic("implement me")
}

func (d *dentry) Invalid(entries ...int64) {
	//TODO implement me
	panic("implement me")
}

type hashKey struct {
	parentID int64
	name     string
}
