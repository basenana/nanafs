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

package types

type EntryAttr struct {
	Name       string
	Kind       Kind
	Access     *Access
	Dev        int64
	Properties Properties
	Labels     Labels
	ExtendData *ExtendData
}

type OpenAttr struct {
	EntryID int64
	Read    bool
	Write   bool
	Create  bool
	Trunc   bool
	Direct  bool

	FsWriteback bool
}

type DestroyObjectAttr struct {
	Uid int64
	Gid int64
}

type ChangeParentAttr struct {
	Uid      int64
	Gid      int64
	Replace  bool
	Exchange bool
}
