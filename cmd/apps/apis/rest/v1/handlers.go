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
	"io"
	"net/http"
	"path"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	"github.com/basenana/nanafs/cmd/apps/apis/apitool"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/workflow"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GroupTree retrieves the group tree structure
func (s *ServicesV1) GroupTree(ctx *gin.Context) {
	caller, err := s.caller(ctx)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	nsRoot, err := s.core.NamespaceRoot(ctx.Request.Context(), caller.Namespace)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	children, err := s.listEntryChildren(ctx.Request.Context(), caller.Namespace, nsRoot.ID)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
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
			apitool.ErrorResponse(ctx, err)
			return
		}
		root.Children = append(root.Children, grp)
	}

	apitool.JsonResponse(ctx, http.StatusOK, &GroupTreeResponse{Root: root})
}

func (s *ServicesV1) listGroupEntry(ctx context.Context, namespace string, name, groupURI string, groupID int64) (*GroupEntry, error) {
	children, err := s.listEntryChildren(ctx, namespace, groupID)
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

func (s *ServicesV1) listEntryChildren(ctx context.Context, namespace string, entryID int64) ([]*types.Entry, error) {
	grp, err := s.core.OpenGroup(ctx, namespace, entryID)
	if err != nil {
		return nil, err
	}
	children, err := grp.ListChildren(ctx)
	if err != nil {
		return nil, err
	}

	sort.Slice(children, func(i, j int) bool {
		return children[i].Name < children[j].Name
	})

	return children, nil
}

// GetEntryDetail retrieves entry details by URI
func (s *ServicesV1) GetEntryDetail(ctx *gin.Context) {
	caller, err := s.caller(ctx)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	uri := ctx.Param("uri")
	s.logger.Infow("request", "uri", uri)

	parentID, entryID, err := s.getEntryByPath(ctx.Request.Context(), caller.Namespace, uri)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	child, err := s.meta.GetChild(ctx.Request.Context(), caller.Namespace, parentID, entryID)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	en, err := s.core.GetEntry(ctx.Request.Context(), caller.Namespace, child.ChildID)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	err = core.HasAllPermissions(en.Access, caller.UID, caller.GID, types.PermOwnerRead, types.PermGroupRead, types.PermOthersRead)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	parentURI, name := path.Split(uri)
	detail, _, err := s.getEntryDetails(ctx.Request.Context(), caller.Namespace, parentURI, name, child.ChildID)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, gin.H{"entry": detail})
}

func (s *ServicesV1) getEntryDetails(ctx context.Context, namespace, uri, name string, id int64) (*EntryDetail, *Property, error) {
	en, err := s.core.GetEntry(ctx, namespace, id)
	if err != nil {
		return nil, nil, err
	}

	doc := types.DocumentProperties{}
	err = s.meta.GetEntryProperties(ctx, namespace, types.PropertyTypeDocument, id, &doc)
	if err != nil {
		return nil, nil, err
	}
	details := toEntryDetail(uri, name, en, doc)

	properties := &types.Properties{}
	err = s.meta.GetEntryProperties(ctx, namespace, types.PropertyTypeProperty, id, properties)
	if err != nil {
		return nil, nil, err
	}
	return details, &Property{
		Tags:       properties.Tags,
		Properties: properties.Properties,
	}, nil
}

// CreateEntry creates a new entry
func (s *ServicesV1) CreateEntry(ctx *gin.Context) {
	caller, err := s.caller(ctx)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	var req CreateEntryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	parentURI, name := path.Split(req.URI)
	_, parentID, err := s.getEntryByPath(ctx.Request.Context(), caller.Namespace, parentURI)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	if req.Kind == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "entry has unknown kind"})
		return
	}

	parent, err := s.core.GetEntry(ctx.Request.Context(), caller.Namespace, parentID)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	err = core.HasAllPermissions(parent.Access, caller.UID, caller.GID, types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	attr := types.EntryAttr{
		Name:   name,
		Kind:   s.pdKind2EntryKind(req.Kind),
		Access: &parent.Access,
	}

	if req.Rss != nil {
		s.setupRssConfig(req.Rss, &attr)
	}
	if req.Filter != nil {
		s.setupGroupFilterConfig(req.Filter, &attr)
	}

	en, err := s.core.CreateEntry(ctx.Request.Context(), caller.Namespace, parentID, attr)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	if attr.Properties != nil {
		if err = s.meta.UpdateEntryProperties(ctx.Request.Context(), caller.Namespace, types.PropertyTypeProperty, en.ID, attr.Properties); err != nil {
			apitool.ErrorResponse(ctx, err)
			return
		}
	}

	apitool.JsonResponse(ctx, http.StatusCreated, gin.H{"entry": toEntryInfo(parentURI, name, en, nil)})
}

func (s *ServicesV1) pdKind2EntryKind(k string) types.Kind {
	kindMap := map[types.Kind]struct{}{
		types.RawKind:           {},
		types.GroupKind:         {},
		types.SmartGroupKind:    {},
		types.FIFOKind:          {},
		types.SocketKind:        {},
		types.SymLinkKind:       {},
		types.BlkDevKind:        {},
		types.CharDevKind:       {},
		types.ExternalGroupKind: {},
	}
	_, ok := kindMap[types.Kind(k)]
	if !ok {
		return types.RawKind
	}
	return types.Kind(k)
}

func (s *ServicesV1) setupRssConfig(config *RssConfig, attr *types.EntryAttr) {
	if config == nil || config.Feed == "" {
		return
	}

	fileType := "html"
	switch config.FileType {
	case "url":
		fileType = "url"
	case "html":
		fileType = "html"
	case "rawhtml":
		fileType = "rawhtml"
	case "webarchive":
		fileType = "webarchive"
	}

	attr.GroupProperties = &types.GroupProperties{
		Source: "rss",
		RSS: &types.GroupRSS{
			Feed:     config.Feed,
			SiteName: config.SiteName,
			SiteURL:  config.SiteURL,
			FileType: fileType,
		},
	}

	if attr.Properties == nil {
		attr.Properties = &types.Properties{}
	}

	attr.Properties.SiteName = config.SiteName
	attr.Properties.URL = config.SiteURL
}

func (s *ServicesV1) setupGroupFilterConfig(config *FilterConfig, attr *types.EntryAttr) {
	attr.GroupProperties = &types.GroupProperties{
		Filter: &types.Filter{
			CELPattern: config.CELPattern,
		},
	}
}

// UpdateEntry updates an existing entry
func (s *ServicesV1) UpdateEntry(ctx *gin.Context) {
	caller, err := s.caller(ctx)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	uri := ctx.Param("uri")
	var req UpdateEntryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	_, entryID, err := s.getEntryByPath(ctx.Request.Context(), caller.Namespace, uri)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	en, err := s.core.GetEntry(ctx.Request.Context(), caller.Namespace, entryID)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	err = core.HasAllPermissions(en.Access, caller.UID, caller.GID, types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	update := types.UpdateEntry{}
	if req.Name != "" {
		update.Name = &req.Name
	}
	if req.Aliases != "" {
		update.Aliases = &req.Aliases
	}
	en, err = s.core.UpdateEntry(ctx.Request.Context(), caller.Namespace, en.ID, update)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	parentURI, name := path.Split(uri)
	detail, _, err := s.getEntryDetails(ctx.Request.Context(), caller.Namespace, parentURI, name, en.ID)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, gin.H{"entry": detail})
}

// DeleteEntry deletes a single entry
func (s *ServicesV1) DeleteEntry(ctx *gin.Context) {
	caller, err := s.caller(ctx)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	uri := ctx.Param("uri")
	parentID, entryID, err := s.getEntryByPath(ctx.Request.Context(), caller.Namespace, uri)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	en, err := s.deleteEntry(ctx.Request.Context(), caller.Namespace, caller.UID, parentID, entryID, path.Base(uri))
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	parentURI, name := path.Split(uri)
	apitool.JsonResponse(ctx, http.StatusOK, gin.H{"entry": toEntryInfo(parentURI, name, en, nil)})
}

func (s *ServicesV1) deleteEntry(ctx context.Context, namespace string, uid, parentID, entryID int64, name string) (*types.Entry, error) {
	en, err := s.core.GetEntry(ctx, namespace, entryID)
	if err != nil {
		return nil, status.Error(codes.Unknown, "query entry failed")
	}

	err = core.HasAllPermissions(en.Access, uid, 0, types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite)
	if err != nil {
		return nil, err
	}

	parent, err := s.core.GetEntry(ctx, namespace, parentID)
	if err != nil {
		return nil, status.Error(codes.Unknown, "query entry parent failed")
	}
	err = core.HasAllPermissions(parent.Access, uid, 0, types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite)
	if err != nil {
		return nil, err
	}

	var gid int64
	if err = core.IsAccess(parent.Access, uid, gid, 0x2); err != nil {
		return nil, types.ErrNoAccess
	}

	if uid != 0 && uid != en.Access.UID && uid != parent.Access.UID && parent.Access.HasPerm(types.PermSticky) {
		return nil, types.ErrNoAccess
	}

	s.logger.Debugw("destroy entry", "parent", parentID, "entry", entryID)
	return en, s.core.RemoveEntry(ctx, namespace, parentID, entryID, name, types.DeleteEntry{})
}

// DeleteEntries performs batch deletion of entries
func (s *ServicesV1) DeleteEntries(ctx *gin.Context) {
	caller, err := s.caller(ctx)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	var req DeleteEntriesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	deleted := make([]string, 0, len(req.URIList))
	var lastErr error
	for _, entryURI := range req.URIList {
		parentID, entryID, err := s.getEntryByPath(ctx.Request.Context(), caller.Namespace, entryURI)
		if err != nil {
			lastErr = err
			continue
		}
		_, err = s.deleteEntry(ctx.Request.Context(), caller.Namespace, caller.UID, parentID, entryID, path.Base(entryURI))
		if err != nil {
			lastErr = err
			continue
		}
		deleted = append(deleted, entryURI)
	}

	if lastErr != nil && len(deleted) == 0 {
		apitool.ErrorResponse(ctx, lastErr)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, &DeleteEntriesResponse{
		Deleted: deleted,
		Message: "Batch delete completed",
	})
}

// ListGroupChildren lists children of a group
func (s *ServicesV1) ListGroupChildren(ctx *gin.Context) {
	caller, err := s.caller(ctx)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	uri := ctx.Param("uri")
	_, parentID, err := s.getEntryByPath(ctx.Request.Context(), caller.Namespace, uri)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	children, err := s.listEntryChildren(ctx.Request.Context(), caller.Namespace, parentID)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	entries := make([]*EntryInfo, 0, len(children))
	for _, en := range children {
		var doc *types.DocumentProperties
		if !en.IsGroup {
			doc = &types.DocumentProperties{}
			if err = s.meta.GetEntryProperties(ctx.Request.Context(), caller.Namespace, types.PropertyTypeDocument, en.ID, doc); err != nil {
				s.logger.Errorw("get entry document properties failed", "entry", en.ID, "err", err)
				doc = nil
			}
		}
		entries = append(entries, toEntryInfo(uri, en.Name, en, doc))
	}

	apitool.JsonResponse(ctx, http.StatusOK, &ListEntriesResponse{Entries: entries})
}

// ChangeParent moves an entry to a new parent
func (s *ServicesV1) ChangeParent(ctx *gin.Context) {
	caller, err := s.caller(ctx)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	var req ChangeParentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	oldName := path.Base(req.EntryURI)
	oldParentID, entryID, err := s.getEntryByPath(ctx.Request.Context(), caller.Namespace, req.EntryURI)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	newParentURI, newName := path.Split(req.NewEntryURI)
	_, newParentID, err := s.getEntryByPath(ctx.Request.Context(), caller.Namespace, newParentURI)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	en, err := s.core.GetEntry(ctx.Request.Context(), caller.Namespace, entryID)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}
	err = core.HasAllPermissions(en.Access, caller.UID, caller.GID, types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	newParent, err := s.core.GetEntry(ctx.Request.Context(), caller.Namespace, newParentID)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}
	err = core.HasAllPermissions(newParent.Access, caller.UID, caller.GID, types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	var existObjID *int64
	existObj, err := s.core.FindEntry(ctx.Request.Context(), caller.Namespace, newParentID, newName)
	if err != nil && !errors.Is(err, types.ErrNotFound) {
		apitool.ErrorResponse(ctx, err)
		return
	}
	if existObj != nil {
		existObjID = &existObj.ChildID
	}

	err = s.core.ChangeEntryParent(ctx.Request.Context(), caller.Namespace, entryID, existObjID, oldParentID, newParentID, oldName, newName, types.ChangeParentAttr{
		Uid:      caller.UID,
		Gid:      caller.GID,
		Replace:  req.Replace,
		Exchange: req.Exchange,
	})
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	en, err = s.core.GetEntry(ctx.Request.Context(), caller.Namespace, entryID)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, gin.H{"entry": toEntryInfo(newParentURI, newName, en, nil)})
}

// FilterEntry filters entries using CEL pattern
func (s *ServicesV1) FilterEntry(ctx *gin.Context) {
	caller, err := s.caller(ctx)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	var req FilterEntryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	it, err := s.meta.FilterEntries(ctx.Request.Context(), caller.Namespace, types.Filter{CELPattern: req.CELPattern})
	if err != nil {
		s.logger.Errorw("list static children failed", "err", err)
		apitool.ErrorResponse(ctx, err)
		return
	}

	entries := make([]*EntryInfo, 0)
	for it.HasNext() {
		en, err := it.Next()
		if err != nil {
			apitool.ErrorResponse(ctx, err)
			return
		}

		var doc *types.DocumentProperties
		if !en.IsGroup {
			doc = &types.DocumentProperties{}
			if err = s.meta.GetEntryProperties(ctx.Request.Context(), caller.Namespace, types.PropertyTypeDocument, en.ID, doc); err != nil {
				s.logger.Errorw("get entry document properties failed", "entry", en.ID, "err", err)
				doc = nil
			}
		}

		uri, err := core.ProbableEntryPath(ctx.Request.Context(), s.core, en)
		if err != nil {
			s.logger.Errorw("guess entry uri error, hide this", "entry", en.ID, "err", err)
			continue
		}
		entries = append(entries, toEntryInfo(path.Dir(uri), path.Base(uri), en, doc))
	}

	apitool.JsonResponse(ctx, http.StatusOK, &ListEntriesResponse{Entries: entries})
}

// WriteFile writes file content via multipart upload
func (s *ServicesV1) WriteFile(ctx *gin.Context) {
	caller, err := s.caller(ctx)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	entryIDStr := ctx.Param("entry")
	entryIDInt, _ := strconv.ParseInt(entryIDStr, 10, 64)

	file, err := s.core.Open(ctx.Request.Context(), caller.Namespace, entryIDInt, types.OpenAttr{Write: true})
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}
	defer file.Close(ctx.Request.Context())

	form, err := ctx.MultipartForm()
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	fileHeader, ok := form.File["file"]
	if !ok || len(fileHeader) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "missing file"})
		return
	}

	src, err := fileHeader[0].Open()
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}
	defer src.Close()

	data, err := io.ReadAll(src)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	_, err = file.WriteAt(ctx.Request.Context(), data, 0)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	if err := file.Flush(ctx.Request.Context()); err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, &WriteFileResponse{Len: int64(len(data))})
}

// ReadFile reads file content
func (s *ServicesV1) ReadFile(ctx *gin.Context) {
	caller, err := s.caller(ctx)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	entryIDStr := ctx.Param("entry")
	entryIDInt, _ := strconv.ParseInt(entryIDStr, 10, 64)

	en, err := s.core.GetEntry(ctx.Request.Context(), caller.Namespace, entryIDInt)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	err = core.HasAllPermissions(en.Access, caller.UID, caller.GID, types.PermOwnerRead, types.PermGroupRead, types.PermOthersRead)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	file, err := s.core.Open(ctx.Request.Context(), caller.Namespace, entryIDInt, types.OpenAttr{Read: true})
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}
	defer file.Close(ctx.Request.Context())

	data := make([]byte, en.Size)
	_, err = file.ReadAt(ctx.Request.Context(), data, 0)
	if err != nil && err != io.EOF {
		apitool.ErrorResponse(ctx, err)
		return
	}

	ctx.Data(http.StatusOK, "application/octet-stream", data)
}

// UpdateProperty updates entry properties
func (s *ServicesV1) UpdateProperty(ctx *gin.Context) {
	caller, err := s.caller(ctx)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	uri := ctx.Param("uri")
	var req UpdatePropertyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	_, entryID, err := s.getEntryByPath(ctx.Request.Context(), caller.Namespace, uri)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	en, err := s.core.GetEntry(ctx.Request.Context(), caller.Namespace, entryID)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	properties := &types.Properties{}
	err = s.meta.GetEntryProperties(ctx.Request.Context(), caller.Namespace, types.PropertyTypeProperty, en.ID, properties)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	properties.Tags = req.Tags
	properties.Properties = req.Properties

	err = s.meta.UpdateEntryProperties(ctx.Request.Context(), caller.Namespace, types.PropertyTypeProperty, en.ID, properties)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, gin.H{
		"properties": &Property{
			Tags:       properties.Tags,
			Properties: properties.Properties,
		},
	})
}

// UpdateDocumentProperty updates document-specific properties
func (s *ServicesV1) UpdateDocumentProperty(ctx *gin.Context) {
	caller, err := s.caller(ctx)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	uri := ctx.Param("uri")
	var req UpdateDocumentPropertyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	_, entryID, err := s.getEntryByPath(ctx.Request.Context(), caller.Namespace, uri)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	en, err := s.core.GetEntry(ctx.Request.Context(), caller.Namespace, entryID)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	if en.IsGroup {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "group has no document properties"})
		return
	}

	properties := &types.DocumentProperties{}
	err = s.meta.GetEntryProperties(ctx.Request.Context(), caller.Namespace, types.PropertyTypeDocument, en.ID, properties)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	properties.Unread = req.Unread
	properties.Marked = req.Marked

	err = s.meta.UpdateEntryProperties(ctx.Request.Context(), caller.Namespace, types.PropertyTypeDocument, en.ID, properties)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, gin.H{
		"properties": &DocumentProperty{
			Title:       properties.Title,
			Author:      properties.Author,
			Year:        properties.Year,
			Source:      properties.Source,
			Abstract:    properties.Abstract,
			Keywords:    properties.Keywords,
			Notes:       properties.Notes,
			Unread:      properties.Unread,
			Marked:      properties.Marked,
			PublishAt:   time.Unix(properties.PublishAt, 0),
			URL:         properties.URL,
			HeaderImage: properties.HeaderImage,
		},
	})
}

// ListMessages retrieves notifications/messages
func (s *ServicesV1) ListMessages(ctx *gin.Context) {
	caller, err := s.caller(ctx)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	all := ctx.Query("all") == "true"

	notifications, err := s.notify.ListNotifications(ctx.Request.Context(), caller.Namespace)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	result := make([]*Message, 0, len(notifications))
	for _, m := range notifications {
		if !all && m.Status == types.NotificationRead {
			continue
		}
		result = append(result, &Message{
			ID:      m.ID,
			Title:   m.Title,
			Message: m.Message,
			Type:    string(m.Type),
			Source:  m.Source,
			Action:  "",
			Status:  string(m.Status),
			Time:    m.Time,
		})
	}

	apitool.JsonResponse(ctx, http.StatusOK, &ListMessagesResponse{Messages: result})
}

// ReadMessages marks messages as read
func (s *ServicesV1) ReadMessages(ctx *gin.Context) {
	caller, err := s.caller(ctx)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	var req ReadMessagesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	for _, id := range req.MessageIDList {
		if err := s.notify.MarkRead(ctx.Request.Context(), caller.Namespace, id); err != nil {
			apitool.ErrorResponse(ctx, err)
			return
		}
	}

	apitool.JsonResponse(ctx, http.StatusOK, &ReadMessagesResponse{Success: true})
}

// ListWorkflows retrieves available workflows
func (s *ServicesV1) ListWorkflows(ctx *gin.Context) {
	caller, err := s.caller(ctx)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	workflowList, err := s.workflow.ListWorkflows(ctx.Request.Context(), caller.Namespace)
	if err != nil {
		s.logger.Errorw("list workflow failed", "err", err)
		apitool.ErrorResponse(ctx, err)
		return
	}

	workflows := make([]*WorkflowInfo, 0, len(workflowList))
	for _, w := range workflowList {
		workflows = append(workflows, toWorkflowInfo(w))
	}

	apitool.JsonResponse(ctx, http.StatusOK, &ListWorkflowsResponse{Workflows: workflows})
}

// ListWorkflowJobs retrieves jobs for a specific workflow
func (s *ServicesV1) ListWorkflowJobs(ctx *gin.Context) {
	caller, err := s.caller(ctx)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	workflowID := ctx.Param("id")
	if workflowID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid workflow id"})
		return
	}

	jobs, err := s.workflow.ListJobs(ctx.Request.Context(), caller.Namespace, workflowID)
	if err != nil {
		s.logger.Errorw("list workflow job failed", "err", err)
		apitool.ErrorResponse(ctx, err)
		return
	}

	result := make([]*WorkflowJobDetail, 0, len(jobs))
	for _, j := range jobs {
		result = append(result, toWorkflowJobDetail(j))
	}

	apitool.JsonResponse(ctx, http.StatusOK, &ListWorkflowJobsResponse{Jobs: result})
}

// TriggerWorkflow 触发工作流
func (s *ServicesV1) TriggerWorkflow(ctx *gin.Context) {
	caller, err := s.caller(ctx)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	var req TriggerWorkflowRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	workflowID := ctx.Param("id")
	if workflowID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid workflow id"})
		return
	}

	s.logger.Infow("trigger workflow", "workflow", workflowID)
	_, err = s.workflow.GetWorkflow(ctx.Request.Context(), caller.Namespace, workflowID)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	timeout := time.Second * 60 * 10
	if req.Timeout > 0 {
		timeout = time.Second * time.Duration(req.Timeout)
	}

	job, err := s.workflow.TriggerWorkflow(ctx.Request.Context(), caller.Namespace, workflowID,
		types.WorkflowTarget{Entries: []string{req.URI}},
		workflow.JobAttr{Reason: req.Reason, Timeout: timeout},
	)
	if err != nil {
		apitool.ErrorResponse(ctx, err)
		return
	}

	apitool.JsonResponse(ctx, http.StatusOK, &TriggerWorkflowResponse{JobID: job.Id})
}

func (s *ServicesV1) getEntryByPath(ctx context.Context, namespace, p string) (int64, int64, error) {
	par, e, err := s.core.GetEntryByPath(ctx, namespace, p)
	if err != nil {
		return 0, 0, status.Error(codes.Unknown, "get entry failed: "+err.Error())
	}
	var pid, eid int64
	if par != nil {
		pid = par.ID
	}
	if e != nil {
		eid = e.ID
	}
	return pid, eid, nil
}
