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

package fs

import (
	"github.com/basenana/nanafs/pkg/types"
	"time"
)

type Entry struct {
	*types.Entry
	Properties map[string]string
}

type BasicEntry struct {
	ID         int64
	Name       string
	RefCount   int
	Mode       uint32
	IsGroup    bool
	Size       int64
	Dev        int64
	UID        int64
	GID        int64
	CreatedAt  time.Time
	ChangedAt  time.Time
	ModifiedAt time.Time
	AccessAt   time.Time
}

type GroupEntry struct {
	Entry    *Entry        `json:"entry"`
	Children []*GroupEntry `json:"children"`
}

type GroupChildren struct {
	Entries []Entry
}
