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

package v1

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/basenana/nanafs/cmd/apps/apis/apitool"
	"github.com/basenana/nanafs/pkg/types"
)

// WriteFile writes file content via multipart upload
func (s *ServicesV1) WriteFile(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	en, _ := s.requireEntryWithPermission(ctx, caller, types.PermOwnerWrite, types.PermOwnerWrite, types.PermOwnerWrite)
	if en == nil {
		return
	}

	file, err := s.core.Open(ctx.Request.Context(), caller.Namespace, en.ID, types.OpenAttr{Write: true})
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}
	defer file.Close(ctx.Request.Context())

	form, err := ctx.MultipartForm()
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	fileHeader, ok := form.File["file"]
	if !ok || len(fileHeader) == 0 {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", errors.New("missing file"))
		return
	}

	src, err := fileHeader[0].Open()
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}
	defer src.Close()

	data, err := io.ReadAll(src)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	_, err = file.WriteAt(ctx.Request.Context(), data, 0)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	if err := file.Flush(ctx.Request.Context()); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, &WriteFileResponse{Len: int64(len(data))})
}

// ReadFile reads file content
func (s *ServicesV1) ReadFile(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	en, _ := s.requireEntryWithPermission(ctx, caller, types.PermOwnerRead, types.PermGroupRead, types.PermOthersRead)
	if en == nil {
		return
	}

	file, err := s.core.Open(ctx.Request.Context(), caller.Namespace, en.ID, types.OpenAttr{Read: true})
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}
	defer file.Close(ctx.Request.Context())

	data := make([]byte, en.Size)
	_, err = file.ReadAt(ctx.Request.Context(), data, 0)
	if err != nil && err != io.EOF {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	ctx.Data(http.StatusOK, "application/octet-stream", data)
}
