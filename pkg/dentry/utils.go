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

package dentry

import "github.com/basenana/nanafs/pkg/types"

func patchChangeableMetadata(oldMd, newMd *types.Metadata) {
	oldMd.Name = newMd.Name
	oldMd.Aliases = newMd.Aliases
	oldMd.ParentID = newMd.ParentID
	oldMd.RefID = newMd.RefID
	oldMd.RefCount = newMd.RefCount
	oldMd.Size = newMd.Size
	oldMd.Dev = newMd.Dev
	oldMd.Storage = newMd.Storage
	oldMd.Namespace = newMd.Namespace
	oldMd.CreatedAt = newMd.CreatedAt
	oldMd.ChangedAt = newMd.ChangedAt
	oldMd.ModifiedAt = newMd.ModifiedAt
	oldMd.AccessAt = newMd.AccessAt
	oldMd.Access = newMd.Access
}
