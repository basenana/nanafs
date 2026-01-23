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
	"fmt"
	"net/http"
	"path"

	"github.com/gin-gonic/gin"

	"github.com/basenana/nanafs/cmd/apps/apis/apitool"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/types"
)

// @Summary Create a new entry
// @Description Create a new entry with specified URI, kind and optional configuration
// @Tags Entries
// @Accept json
// @Produce json
// @Param request body CreateEntryRequest true "Entry creation request"
// @Success 201 {object} EntryResponse
// @Router /api/v1/entries [post]
func (s *ServicesV1) CreateEntry(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	var req CreateEntryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	if req.Kind == "" {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", errors.New("entry has unknown kind"))
		return
	}

	parentURI, name := path.Split(req.URI)
	_, parent, err := s.core.GetEntryByPath(ctx, caller.Namespace, parentURI)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	err = core.HasAllPermissions(parent.Access, caller.UID, caller.GID, types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	attr := types.EntryAttr{
		Name:       name,
		Kind:       s.pdKind2EntryKind(req.Kind),
		Access:     &parent.Access,
		Properties: req.Properties,
	}

	if req.Rss != nil {
		s.setupRssConfig(req.Rss, &attr)
	}
	if req.Filter != nil {
		s.setupGroupFilterConfig(req.Filter, &attr)
	}

	en, err := s.core.CreateEntry(ctx.Request.Context(), caller.Namespace, parentURI, attr)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	if req.Document != nil && !en.IsGroup {
		if err = s.meta.UpdateEntryProperties(ctx.Request.Context(), caller.Namespace, types.PropertyTypeDocument, en.ID, req.Document); err != nil {
			apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
			return
		}
	}

	doc := s.getDocumentProperty(ctx.Request.Context(), caller.Namespace, en)
	apitool.JsonResponse(ctx, http.StatusCreated, &EntryResponse{
		Entry: toEntryInfo(parentURI, name, en, doc),
	})
}

// @Summary Update entry
// @Description Update an existing entry's aliases
// @Tags Entries
// @Accept json
// @Produce json
// @Param request body UpdateEntryRequest true "Update entry request"
// @Param uri query string false "Entry URI"
// @Param id query string false "Entry ID"
// @Success 200 {object} EntryDetailResponse
// @Router /api/v1/entries [put]
func (s *ServicesV1) UpdateEntry(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	var req UpdateEntryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	en, uri := s.requireEntryWithPermission(ctx, caller, &req.EntrySelector, types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite)
	if en == nil {
		return
	}

	update := types.UpdateEntry{}
	if req.Aliases != "" {
		update.Aliases = &req.Aliases
	}
	en, err := s.core.UpdateEntry(ctx.Request.Context(), caller.Namespace, en.ID, update)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	parentURI := uri
	if parentURI == "" {
		parentURI = "/"
	}
	detail, err := s.getEntryDetails(ctx.Request.Context(), caller.Namespace, path.Dir(parentURI), path.Base(parentURI), en.ID)
	if err != nil {
		apitool.JsonResponse(ctx, http.StatusOK, &EntryResponse{
			Entry: toEntryInfo("", en.Name, en, nil),
		})
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, &EntryDetailResponse{
		Entry: detail,
	})
}

// @Summary Delete entry
// @Description Delete a single entry by URI or ID
// @Tags Entries
// @Accept json
// @Produce json
// @Param request body EntryDetailRequest true "Entry selector"
// @Success 200 {object} EntryResponse
// @Router /api/v1/entries/delete [post]
func (s *ServicesV1) DeleteEntry(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	var req EntryDetailRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	en, uri := s.requireEntryWithPermission(ctx, caller, &req.EntrySelector, types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite)
	if en == nil {
		return
	}

	deletedEntry, err := s.deleteEntry(ctx.Request.Context(), caller.Namespace, caller.UID, uri)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	parentURI, name := path.Split(uri)
	apitool.JsonResponse(ctx, http.StatusOK, &EntryResponse{
		Entry: toEntryInfo(parentURI, name, deletedEntry, nil),
	})
}

// @Summary Delete entries
// @Description Batch delete multiple entries by URI list
// @Tags Entries
// @Accept json
// @Produce json
// @Param request body DeleteEntriesRequest true "Delete entries request"
// @Success 200 {object} DeleteEntriesResponse
// @Router /api/v1/entries/batch-delete [post]
func (s *ServicesV1) DeleteEntries(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	var req DeleteEntriesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	deleted := make([]string, 0, len(req.URIList))
	var lastErr error
	for _, entryURI := range req.URIList {
		_, err := s.deleteEntry(ctx.Request.Context(), caller.Namespace, caller.UID, entryURI)
		if err != nil {
			lastErr = err
			continue
		}
		deleted = append(deleted, entryURI)
	}

	if lastErr != nil && len(deleted) == 0 {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", lastErr)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, &DeleteEntriesResponse{
		Deleted: deleted,
		Message: "Batch delete completed",
	})
}

// @Summary Change entry parent
// @Description Move an entry to a new parent directory
// @Tags Entries
// @Accept json
// @Produce json
// @Param request body ChangeParentRequest true "Change parent request"
// @Success 200 {object} EntryResponse
// @Router /api/v1/entries/parent [put]
func (s *ServicesV1) ChangeParent(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	var req ChangeParentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	s.logger.Infow("change parent", "old", req.EntryURI, "new", req.NewEntryURI)

	targetEntryURI := req.EntryURI
	newParentURI, newName := path.Split(req.NewEntryURI)

	if newParentURI == req.EntryURI {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", fmt.Errorf("new entry uri is the same as old"))
		return
	}

	// Get target entry for permission check
	_, en, err := s.core.GetEntryByPath(ctx.Request.Context(), caller.Namespace, targetEntryURI)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}
	err = core.HasAllPermissions(en.Access, caller.UID, caller.GID, types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	if req.EntryURI == req.NewEntryURI {
		apitool.JsonResponse(ctx, http.StatusOK, &EntryResponse{
			Entry: toEntryInfo(newParentURI, newName, en, nil),
		})
		return
	}

	// Get new parent for permission check
	_, newParent, err := s.core.GetEntryByPath(ctx.Request.Context(), caller.Namespace, newParentURI)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}
	err = core.HasAllPermissions(newParent.Access, caller.UID, caller.GID, types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	err = s.core.ChangeEntryParent(ctx.Request.Context(), caller.Namespace, targetEntryURI, newParentURI, newName, types.ChangeParentAttr{
		Uid:      caller.UID,
		Gid:      caller.GID,
		Replace:  req.Replace,
		Exchange: req.Exchange,
	})
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	// After changing parent, use the NEW path to get the entry
	newEntryURI := path.Join(newParentURI, newName)
	_, en, err = s.core.GetEntryByPath(ctx.Request.Context(), caller.Namespace, newEntryURI)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, &EntryResponse{
		Entry: toEntryInfo(newParentURI, newName, en, nil),
	})
}

// @Summary Filter entries
// @Description Filter entries using CEL pattern matching with pagination support
// @Tags Entries
// @Accept json
// @Produce json
// @Param request body FilterEntryRequest true "Filter entry request"
// @Success 200 {object} ListEntriesResponse
// @Router /api/v1/entries/filter [post]
func (s *ServicesV1) FilterEntry(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	var req FilterEntryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	pg := types.NewPaginationWithSort(req.Page, req.PageSize, req.Sort, req.Order)
	pctx := types.WithPagination(ctx.Request.Context(), pg)

	it, err := s.meta.FilterEntries(pctx, caller.Namespace, types.Filter{CELPattern: req.CELPattern})
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	entries := make([]*EntryInfo, 0)
	for it.HasNext() {
		en, err := it.Next()
		if err != nil {
			apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
			return
		}

		doc := s.getDocumentProperty(pctx, caller.Namespace, en)

		uri, err := core.ProbableEntryPath(pctx, s.core, en)
		if err != nil {
			continue
		}
		entries = append(entries, toEntryInfo(path.Dir(uri), path.Base(uri), en, doc))
	}

	apitool.JsonResponse(ctx, http.StatusOK, &ListEntriesResponse{
		Entries:    entries,
		Pagination: &PaginationInfo{Page: req.Page, PageSize: req.PageSize},
	})
}

// @Summary Search documents
// @Description Search documents using full-text search with pagination support
// @Tags Entries
// @Accept json
// @Produce json
// @Param request body SearchDocumentsRequest true "Search documents request"
// @Success 200 {object} SearchDocumentsResponse
// @Router /api/v1/entries/search [post]
func (s *ServicesV1) SearchEntry(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	var req SearchDocumentsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	pg := types.NewPagination(req.Page, req.PageSize)
	pctx := types.WithPagination(ctx.Request.Context(), pg)

	var docs []*types.IndexDocument
	var err error
	if s.indexer != nil {
		docs, err = s.indexer.QueryLanguage(pctx, caller.Namespace, req.Query)
		if err != nil {
			apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
			return
		}
	}

	documents := make([]*DocumentInfo, 0, len(docs))
	for _, d := range docs {
		documents = append(documents, &DocumentInfo{
			ID:               d.ID,
			URI:              d.URI,
			Title:            d.Title,
			HighlightTitle:   d.HighlightTitle,
			HighlightContent: d.HighlightContent,
			CreateAt:         nanoTimestampToTime(d.CreateAt),
			ChangedAt:        nanoTimestampToTime(d.ChangedAt),
		})
	}

	apitool.JsonResponse(ctx, http.StatusOK, &SearchDocumentsResponse{
		Documents:  documents,
		Pagination: &PaginationInfo{Page: req.Page, PageSize: req.PageSize},
	})
}

// @Summary Get entry details
// @Description Get detailed information of an entry by URI or ID
// @Tags Entries
// @Accept json
// @Produce json
// @Param request body EntryDetailRequest true "Entry selector"
// @Success 200 {object} EntryDetailResponse
// @Router /api/v1/entries/details [post]
func (s *ServicesV1) EntryDetails(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	var req EntryDetailRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	en, uri := s.requireEntryWithPermission(ctx, caller, &req.EntrySelector, types.PermOwnerRead, types.PermGroupRead, types.PermOthersRead)
	if en == nil {
		return
	}

	parentURI, name := path.Split(uri)
	detail, err := s.getEntryDetails(ctx.Request.Context(), caller.Namespace, parentURI, name, en.ID)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, &EntryDetailResponse{Entry: detail})
}

// @Summary Get entry property
// @Description Get properties and tags of an entry
// @Tags Entries
// @Accept json
// @Produce json
// @Param request body EntryDetailRequest true "Entry selector"
// @Success 200 {object} PropertyResponse
// @Router /api/v1/entries/property [post]
func (s *ServicesV1) EntryProperty(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	var req EntryDetailRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	en, _ := s.requireEntryWithPermission(ctx, caller, &req.EntrySelector, types.PermOwnerRead, types.PermGroupRead, types.PermOthersRead)
	if en == nil {
		return
	}

	properties := &types.Properties{}
	err := s.meta.GetEntryProperties(ctx.Request.Context(), caller.Namespace, types.PropertyTypeProperty, en.ID, properties)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, &PropertyResponse{
		Property: toProperty(properties),
	})
}

// @Summary Get entry document
// @Description Get document properties of an entry
// @Tags Entries
// @Accept json
// @Produce json
// @Param uri query string false "Entry URI"
// @Param id query string false "Entry ID"
// @Success 200 {object} DocumentPropertyResponse
// @Router /api/v1/entries/document [get]
func (s *ServicesV1) EntryDocument(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	var req UpdateDocumentPropertyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	en, _ := s.requireEntryWithPermission(ctx, caller, &req.EntrySelector, types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite)
	if en == nil {
		return
	}

	if en.IsGroup {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", errors.New("group has no document properties"))
		return
	}

	properties := &types.DocumentProperties{}
	err := s.meta.GetEntryProperties(ctx.Request.Context(), caller.Namespace, types.PropertyTypeDocument, en.ID, properties)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	if req.Title != nil {
		properties.Title = *req.Title
	}
	if req.Author != nil {
		properties.Author = *req.Author
	}
	if req.Year != nil {
		properties.Year = *req.Year
	}
	if req.Source != nil {
		properties.Source = *req.Source
	}
	if req.Abstract != nil {
		properties.Abstract = *req.Abstract
	}
	if req.Notes != nil {
		properties.Notes = *req.Notes
	}
	if req.Keywords != nil {
		properties.Keywords = req.Keywords
	}
	if req.URL != nil {
		properties.URL = *req.URL
	}
	if req.SiteName != nil {
		properties.SiteName = *req.SiteName
	}
	if req.SiteURL != nil {
		properties.SiteURL = *req.SiteURL
	}
	if req.HeaderImage != nil {
		properties.HeaderImage = *req.HeaderImage
	}
	if req.Unread != nil {
		properties.Unread = *req.Unread
	}
	if req.Marked != nil {
		properties.Marked = *req.Marked
	}
	if req.PublishAt != nil {
		properties.PublishAt = timeToTimestamp(*req.PublishAt)
	}

	err = s.meta.UpdateEntryProperties(ctx.Request.Context(), caller.Namespace, types.PropertyTypeDocument, en.ID, properties)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, &DocumentPropertyResponse{
		Property: &DocumentProperty{
			Title:       properties.Title,
			Author:      properties.Author,
			Year:        properties.Year,
			Source:      properties.Source,
			Abstract:    properties.Abstract,
			Keywords:    properties.Keywords,
			Notes:       properties.Notes,
			Unread:      properties.Unread,
			Marked:      properties.Marked,
			PublishAt:   timestampTime(properties.PublishAt),
			URL:         properties.URL,
			HeaderImage: properties.HeaderImage,
		},
	})
}

// @Summary Update entry property
// @Description Update properties and tags of an entry
// @Tags Entries
// @Accept json
// @Produce json
// @Param request body UpdatePropertyRequest true "Update property request"
// @Param uri query string false "Entry URI"
// @Param id query string false "Entry ID"
// @Success 200 {object} PropertyResponse
// @Router /api/v1/entries/property [put]
func (s *ServicesV1) UpdateProperty(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	var req UpdatePropertyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	en, _ := s.requireEntryWithPermission(ctx, caller, &req.EntrySelector, types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite)
	if en == nil {
		return
	}

	properties := &types.Properties{}
	err := s.meta.GetEntryProperties(ctx.Request.Context(), caller.Namespace, types.PropertyTypeProperty, en.ID, properties)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	properties.Tags = req.Tags
	properties.Properties = req.Properties

	err = s.meta.UpdateEntryProperties(ctx.Request.Context(), caller.Namespace, types.PropertyTypeProperty, en.ID, properties)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, &PropertyResponse{
		Property: toProperty(properties),
	})
}

// @Summary Get entry friday property
// @Description Get friday processing summary of an entry
// @Tags Entries
// @Accept json
// @Produce json
// @Param request body EntryDetailRequest true "Entry selector"
// @Success 200 {object} FridayPropertyResponse
// @Router /api/v1/entries/friday [post]
func (s *ServicesV1) GetFridayProperty(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	var req EntryDetailRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	en, _ := s.requireEntryWithPermission(ctx, caller, &req.EntrySelector, types.PermOwnerRead, types.PermGroupRead, types.PermOthersRead)
	if en == nil {
		return
	}

	properties := &types.FridayProcessProperties{}
	err := s.meta.GetEntryProperties(ctx.Request.Context(), caller.Namespace, types.PropertyTypeFriday, en.ID, properties)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, &FridayPropertyResponse{
		Property: toFridayProperty(properties),
	})
}

// @Summary Update document property
// @Description Update document properties (unread, marked) of an entry
// @Tags Entries
// @Accept json
// @Produce json
// @Param request body UpdateDocumentPropertyRequest true "Update document request"
// @Param uri query string false "Entry URI"
// @Param id query string false "Entry ID"
// @Success 200 {object} DocumentPropertyResponse
// @Router /api/v1/entries/document [put]
func (s *ServicesV1) UpdateDocumentProperty(ctx *gin.Context) {
	caller := s.requireCaller(ctx)
	if caller == nil {
		return
	}

	var req UpdateDocumentPropertyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	en, _ := s.requireEntryWithPermission(ctx, caller, &req.EntrySelector, types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite)
	if en == nil {
		return
	}

	if en.IsGroup {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", errors.New("group has no document properties"))
		return
	}

	properties := &types.DocumentProperties{}
	err := s.meta.GetEntryProperties(ctx.Request.Context(), caller.Namespace, types.PropertyTypeDocument, en.ID, properties)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	if req.Title != nil {
		properties.Title = *req.Title
	}
	if req.Author != nil {
		properties.Author = *req.Author
	}
	if req.Year != nil {
		properties.Year = *req.Year
	}
	if req.Source != nil {
		properties.Source = *req.Source
	}
	if req.Abstract != nil {
		properties.Abstract = *req.Abstract
	}
	if req.Notes != nil {
		properties.Notes = *req.Notes
	}
	if req.Keywords != nil {
		properties.Keywords = req.Keywords
	}
	if req.URL != nil {
		properties.URL = *req.URL
	}
	if req.SiteName != nil {
		properties.SiteName = *req.SiteName
	}
	if req.SiteURL != nil {
		properties.SiteURL = *req.SiteURL
	}
	if req.HeaderImage != nil {
		properties.HeaderImage = *req.HeaderImage
	}
	if req.Unread != nil {
		properties.Unread = *req.Unread
	}
	if req.Marked != nil {
		properties.Marked = *req.Marked
	}
	if req.PublishAt != nil {
		properties.PublishAt = timeToTimestamp(*req.PublishAt)
	}

	err = s.meta.UpdateEntryProperties(ctx.Request.Context(), caller.Namespace, types.PropertyTypeDocument, en.ID, properties)
	if err != nil {
		apitool.ErrorResponse(ctx, http.StatusBadRequest, "INVALID_ARGUMENT", err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, &DocumentPropertyResponse{
		Property: &DocumentProperty{
			Title:       properties.Title,
			Author:      properties.Author,
			Year:        properties.Year,
			Source:      properties.Source,
			Abstract:    properties.Abstract,
			Keywords:    properties.Keywords,
			Notes:       properties.Notes,
			Unread:      properties.Unread,
			Marked:      properties.Marked,
			PublishAt:   timestampTime(properties.PublishAt),
			URL:         properties.URL,
			HeaderImage: properties.HeaderImage,
		},
	})
}
