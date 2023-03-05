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
	"github.com/basenana/nanafs/pkg/types"
	"sync"
)

type Iterator interface {
	HasNext() bool
	Next() *types.Object
}

type iterator struct {
	objects []*types.Object
	mux     sync.Mutex
}

func (i *iterator) HasNext() bool {
	i.mux.Lock()
	defer i.mux.Unlock()
	return len(i.objects) > 0
}

func (i *iterator) Next() *types.Object {
	i.mux.Lock()
	defer i.mux.Unlock()
	obj := i.objects[0]
	i.objects = i.objects[1:]
	return obj
}
