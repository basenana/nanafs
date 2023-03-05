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
	"os/user"
	"strconv"
)

type FsOwner struct {
	Uid int64 `json:"uid"`
	Gid int64 `json:"gid"`
}

func defaultOwner() *FsOwner {
	u, err := user.Current()
	if err != nil {
		return nil
	}
	result := &FsOwner{}
	result.Uid, _ = strconv.ParseInt(u.Uid, 10, 64)
	result.Gid, _ = strconv.ParseInt(u.Gid, 10, 64)
	return result
}
