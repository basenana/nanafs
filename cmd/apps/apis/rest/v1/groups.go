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
	"context"
	"net/http"
	"path"

	"github.com/gin-gonic/gin"

	"github.com/basenana/nanafs/cmd/apps/apis/apitool"
)

func (s *ServicesV1) listGroupEntry(ctx context.Context, namespace string, name, groupURI string, groupID int64) (*GroupEntry, error) {
	children, err := s.listChildren(ctx, namespace, groupID)
	if err != nil {
		return nil, err
	}

	result := &GroupEntry{
		Name:     name,
		URI:      groupURI,
		Children: nil,
	}

	if len(children) > 0 {
		result.Children = make([]*GroupEntry, 0, len(children))
		for _, child := range children {
			if !child.IsGroup {
				continue
			}
			grp, err := s.listGroupEntry(ctx, namespace, child.Name, path.Join(groupURI, child.Name), child.ID)
			if err != nil {
				return nil, err
			}
			result.Children = append(result.Children, grp)
		}
	}
	return result, nil
}

// GroupTree retrieves the group tree structure
func (s *ServicesV1) GroupTree(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	nsRoot, err := s.core.NamespaceRoot(ctx.Request.Context(), caller.Namespace)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	children, err := s.listChildren(ctx.Request.Context(), caller.Namespace, nsRoot.ID)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	root := &GroupEntry{
		URI:      "/",
		Name:     "/",
		Children: make([]*GroupEntry, 0, len(children)),
	}

	for _, child := range children {
		if !child.IsGroup {
			continue
		}
		grp, err := s.listGroupEntry(ctx.Request.Context(), caller.Namespace, child.Name, path.Join(root.URI, child.Name), child.ID)
		if err != nil {
			apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
			return
		}
		root.Children = append(root.Children, grp)
	}

	apitool.JsonResponse(ctx, http.StatusOK, &GroupTreeResponse{Root: root})
}

// ListGroupChildren lists children of a group
func (s *ServicesV1) ListGroupChildren(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	var req ListGroupChildrenRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	uri := ctx.Query("uri")
	_, parentID, err := s.getEntryByPath(ctx.Request.Context(), caller.Namespace, uri)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	children, err := s.listChildren(ctx.Request.Context(), caller.Namespace, parentID)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = int64(len(children))
	}

	offset := (page - 1) * pageSize
	if offset > int64(len(children)) {
		offset = int64(len(children))
	}

	end := offset + pageSize
	if end > int64(len(children)) {
		end = int64(len(children))
	}

	entries := make([]*EntryInfo, 0)
	for _, en := range children[offset:end] {
		doc := s.getDocumentProperty(ctx.Request.Context(), caller.Namespace, en)
		entries = append(entries, toEntryInfo(uri, en.Name, en, doc))
	}

	apitool.JsonResponse(ctx, http.StatusOK, &ListEntriesResponse{
		Entries: entries,
		Pagination: &PaginationInfo{Page: page, PageSize: pageSize},
	})
}

// GetGroupTree retrieves the group tree structure (alias for GroupTree)
func (s *ServicesV1) GetGroupTree(ctx *gin.Context) {
	s.GroupTree(ctx)
}

// ListChildren lists children of a group (alias for ListGroupChildren)
func (s *ServicesV1) ListChildren(ctx *gin.Context) {
	s.ListGroupChildren(ctx)
}
