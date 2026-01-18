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
	"net/http"
	"path"

	"github.com/gin-gonic/gin"

	"github.com/basenana/nanafs/cmd/apps/apis/apitool"
	"github.com/basenana/nanafs/pkg/types"
)

// @Summary Get group tree
// @Description Retrieve the complete group tree structure
// @Tags Groups
// @Accept json
// @Produce json
// @Param uri query string false "Group URI"
// @Success 200 {object} GroupTreeResponse
// @Router /api/v1/groups/tree [get]
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

	allGroupChildren, err := s.meta.ListNamespaceGroups(ctx.Request.Context(), caller.Namespace)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	rootChildren := buildGroupTreeFromChildren(allGroupChildren, nsRoot.ID)

	apitool.JsonResponse(ctx, http.StatusOK, &GroupTreeResponse{
		Root: &GroupEntry{
			Name:     "/",
			URI:      "/",
			Children: rootChildren,
		},
	})
}

// @Summary List group children
// @Description List children entries of a group with pagination
// @Tags Groups
// @Accept json
// @Produce json
// @Param uri query string false "Group URI"
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Param order query string false "Order field"
// @Param desc query bool false "Sort descending"
// @Success 200 {object} ListEntriesResponse
// @Router /api/v1/groups/children [get]
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

	uri := decodeMagicURI(ctx.Query("uri"))
	_, parentID, err := s.getEntryByPath(ctx.Request.Context(), caller.Namespace, uri)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	pg := types.NewPaginationWithSort(req.Page, req.PageSize, req.Sort, req.Order)
	pagedCtx := types.WithPagination(ctx.Request.Context(), pg)

	children, err := s.listChildren(pagedCtx, caller.Namespace, parentID)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	page := pg.Page
	pageSize := pg.PageSize

	entries := make([]*EntryInfo, 0, len(children))
	for _, en := range children {
		doc := s.getDocumentProperty(ctx.Request.Context(), caller.Namespace, en)
		entries = append(entries, toEntryInfo(uri, en.Name, en, doc))
	}

	apitool.JsonResponse(ctx, http.StatusOK, &ListEntriesResponse{
		Entries:    entries,
		Pagination: &PaginationInfo{Page: page, PageSize: pageSize},
	})
}

// buildGroupTreeFromChildren builds group tree from flat children list.
// allChildren: all group children in namespace (flat list)
// rootID: root node ID (namespace root entry ID)
func buildGroupTreeFromChildren(allChildren []*types.Child, rootID int64) []*GroupEntry {
	childrenByParent := make(map[int64][]*types.Child)
	for _, child := range allChildren {
		childrenByParent[child.ParentID] = append(childrenByParent[child.ParentID], child)
	}

	var build func(parentID int64, parentURI string) []*GroupEntry
	build = func(parentID int64, parentURI string) []*GroupEntry {
		children := childrenByParent[parentID]
		if len(children) == 0 {
			return nil
		}
		result := make([]*GroupEntry, 0, len(children))
		for _, child := range children {
			entryURI := path.Join(parentURI, child.Name)
			result = append(result, &GroupEntry{
				Name:     child.Name,
				URI:      entryURI,
				Children: build(child.ChildID, entryURI),
			})
		}
		return result
	}

	return build(rootID, "/")
}
