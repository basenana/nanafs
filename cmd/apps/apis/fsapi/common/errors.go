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

package common

import (
	"github.com/basenana/nanafs/pkg/types"
	"google.golang.org/grpc/codes"
)

func FsApiError(err error) codes.Code {
	if err == nil {
		return codes.OK
	}

	switch err {
	case types.ErrNotFound:
		return codes.NotFound
	case types.ErrIsExist:
		return codes.AlreadyExists
	case types.ErrNameTooLong,
		types.ErrNoGroup,
		types.ErrNotEmpty,
		types.ErrIsGroup:
		return codes.InvalidArgument
	case types.ErrNoAccess:
		return codes.Unauthenticated
	case types.ErrNoPerm:
		return codes.PermissionDenied
	case types.ErrUnsupported:
		return codes.Unimplemented
	}

	return codes.Unknown
}
