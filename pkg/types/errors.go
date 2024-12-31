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

import "errors"

var (
	ErrNotFound    = errors.New("no record")
	ErrNameTooLong = errors.New("name too long")
	ErrIsExist     = errors.New("record existed")
	ErrNotEmpty    = errors.New("group not empty")
	ErrNoGroup     = errors.New("not group")
	ErrIsGroup     = errors.New("this object is a group")
	ErrNoAccess    = errors.New("no access")
	ErrNoPerm      = errors.New("no permission")
	ErrNoNamespace = errors.New("no namespace")
	ErrConflict    = errors.New("operation conflict")
	ErrUnsupported = errors.New("unsupported operation")
	ErrNotEnable   = errors.New("not enable")
)
