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

package stub

import (
	"github.com/basenana/nanafs/pkg/types"
	"time"
)

type Entry struct {
	Name    string
	Kind    types.Kind
	Size    int64
	IsGroup bool
}

type EntryAttr struct {
	Name   string
	Kind   types.Kind
	Access types.Access
}

type FreshOption struct {
	LastFreshAt time.Time
}
