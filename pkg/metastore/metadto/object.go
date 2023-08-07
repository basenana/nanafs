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

package metadto

import "github.com/basenana/nanafs/pkg/types"

type Object struct {
	ID         int64
	Name       *string
	Aliases    *string
	ParentID   *int64
	RefID      *int64
	RefCount   *int
	Kind       *string
	KindMap    *int64
	Size       *int64
	Version    *int64
	Dev        *int64
	Owner      *int64
	GroupOwner *int64
	Permission *int64
	Storage    *string
	Namespace  *string
	CreatedAt  *int64
	ChangedAt  *int64
	ModifiedAt *int64
	AccessAt   *int64

	*types.ExtendData
	*types.Labels
}
