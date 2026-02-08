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
// @Param request body ListGroupChildrenRequest true "Group selector with pagination"
// @Success 200 {object} ListEntriesResponse
// @Router /api/v1/groups/children [post]
func (s *ServicesV1) ListGroupChildren(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	var (
		req      ListGroupChildrenRequest
		parentID int64
		err      error
		pageInfo *PaginationInfo
	)
	if err = ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	if req.URI != "" {
		_, parentID, err = s.getEntryByPath(ctx.Request.Context(), caller.Namespace, req.URI)
		if err != nil {
			apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
			return
		}
	} else if req.ID != 0 {
		parentID = req.ID
	} else {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", errors.New("missing uri or id parameter"))
		return
	}

	pg := types.NewPaginationWithSort(req.Page, req.PageSize, req.Sort, req.Order)
	pagedCtx := types.WithPagination(ctx.Request.Context(), pg)

	children, err := s.listChildren(pagedCtx, caller.Namespace, parentID)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	if pg != nil {
		pageInfo = &PaginationInfo{Page: pg.Page, PageSize: pg.PageSize}
	}
	entries := make([]*EntryInfo, 0, len(children))
	for _, en := range children {
		doc := s.getDocumentProperty(ctx.Request.Context(), caller.Namespace, en)
		entries = append(entries, toEntryInfo(req.URI, en.Name, en, doc))
	}

	apitool.JsonResponse(ctx, http.StatusOK, &ListEntriesResponse{Entries: entries, Pagination: pageInfo})
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

// @Summary Get group config
// @Description Get configuration of a group (RSS or filter)
// @Tags Groups
// @Accept json
// @Produce json
// @Param request body GetGroupConfigRequest true "Group config request"
// @Success 200 {object} GroupConfigResponse
// @Router /api/v1/groups/configs [post]
func (s *ServicesV1) GetGroupConfig(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	var req GetGroupConfigRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	en, _ := s.requireEntryWithPermission(ctx, caller, &req.EntrySelector, types.PermOwnerRead, types.PermGroupRead, types.PermOthersRead)
	if en == nil {
		return
	}

	if !en.IsGroup {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", errors.New("entry is not a group"))
		return
	}

	groupProps := &types.GroupProperties{}
	err := s.meta.GetEntryProperties(ctx.Request.Context(), caller.Namespace, types.PropertyTypeGroupAttr, en.ID, groupProps)
	if err != nil {
		if !errors.Is(err, types.ErrNotFound) {
			apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
			return
		}
	}

	resp := &GroupConfigResponse{
		Kind:   string(en.Kind),
		Source: groupProps.Source,
	}

	if groupProps.RSS != nil {
		resp.RSS = &RssConfig{
			Feed:     groupProps.RSS.Feed,
			SiteName: groupProps.RSS.SiteName,
			SiteURL:  groupProps.RSS.SiteURL,
			FileType: groupProps.RSS.FileType,
		}
	}

	if groupProps.Filter != nil {
		resp.Filter = &FilterConfig{
			CELPattern: groupProps.Filter.CELPattern,
		}
	}

	apitool.JsonResponse(ctx, http.StatusOK, resp)
}

// @Summary Set group config
// @Description Set configuration of a group (filter)
// @Tags Groups
// @Accept json
// @Produce json
// @Param request body SetGroupConfigRequest true "Group config request"
// @Success 200 {object} GroupConfigResponse
// @Router /api/v1/groups/configs/update [post]
func (s *ServicesV1) SetGroupConfig(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	var req SetGroupConfigRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	en, _ := s.requireEntryWithPermission(ctx, caller, &req.EntrySelector, types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite)
	if en == nil {
		return
	}

	if !en.IsGroup {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", errors.New("entry is not a group"))
		return
	}

	groupProps := &types.GroupProperties{}
	err := s.meta.GetEntryProperties(ctx.Request.Context(), caller.Namespace, types.PropertyTypeGroupAttr, en.ID, groupProps)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	if req.Filter != nil && groupProps.Filter != nil {
		groupProps.Filter = &types.Filter{
			CELPattern: req.Filter.CELPattern,
		}
	}
	if req.Rss != nil && groupProps.RSS != nil {
		groupProps.RSS = &types.GroupRSS{
			Feed:     req.Rss.Feed,
			SiteName: req.Rss.SiteName,
			SiteURL:  req.Rss.SiteURL,
			FileType: req.Rss.FileType,
		}
	}

	err = s.meta.UpdateEntryProperties(ctx.Request.Context(), caller.Namespace, types.PropertyTypeGroupAttr, en.ID, groupProps)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	resp := &GroupConfigResponse{
		Kind:   string(en.Kind),
		Source: groupProps.Source,
	}

	if groupProps.RSS != nil {
		resp.RSS = &RssConfig{
			Feed:     groupProps.RSS.Feed,
			SiteName: groupProps.RSS.SiteName,
			SiteURL:  groupProps.RSS.SiteURL,
			FileType: groupProps.RSS.FileType,
		}
	}

	if groupProps.Filter != nil {
		resp.Filter = &FilterConfig{
			CELPattern: groupProps.Filter.CELPattern,
		}
	}

	apitool.JsonResponse(ctx, http.StatusOK, resp)
}
