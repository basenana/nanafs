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
	"errors"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/document"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/notify"
	"github.com/basenana/nanafs/pkg/token"
	"io"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/basenana/nanafs/workflow"

	"github.com/basenana/nanafs/cmd/apps/apis/fsapi/common"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
)

type ServicesV1 interface {
	AuthServer
	DocumentServer
	EntriesServer
	PropertiesServer
	NotifyServer
	WorkflowServer
}

func InitServicesV1(server *grpc.Server, depends *common.Depends) (ServicesV1, error) {
	s := &servicesV1{
		meta:      depends.Meta,
		core:      depends.Core,
		doc:       depends.Document,
		token:     depends.Token,
		workflow:  depends.Workflow,
		caller:    common.CallerAuth,
		notify:    depends.Notify,
		cfgLoader: depends.ConfigLoader,
		logger:    logger.NewLogger("fsapi"),
	}

	RegisterAuthServer(server, s)
	RegisterDocumentServer(server, s)
	RegisterEntriesServer(server, s)
	RegisterPropertiesServer(server, s)
	RegisterNotifyServer(server, s)
	RegisterWorkflowServer(server, s)
	return s, nil
}

type servicesV1 struct {
	meta      metastore.Meta
	core      core.Core
	doc       document.Manager
	token     *token.Manager
	workflow  workflow.Workflow
	caller    common.CallerAuthGetter
	notify    *notify.Notify
	cfgLoader config.Config
	logger    *zap.SugaredLogger
}

var _ ServicesV1 = &servicesV1{}

func (s *servicesV1) AccessToken(ctx context.Context, request *AccessTokenRequest) (*AccessTokenResponse, error) {
	token, err := s.token.AccessToken(ctx, request.AccessTokenKey, request.SecretToken)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "access token error")
	}
	return &AccessTokenResponse{
		Namespace:      token.Namespace,
		UID:            token.UID,
		GID:            token.GID,
		ClientCrt:      token.ClientCrt,
		ClientKey:      token.ClientKey,
		CertExpiration: timestamppb.New(token.CertExpiration),
	}, nil
}

func (s *servicesV1) ListDocuments(ctx context.Context, request *ListDocumentsRequest) (*ListDocumentsResponse, error) {
	caller := s.caller(ctx)
	if !caller.Authenticated {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	filter := types.DocFilter{ParentID: request.ParentID}
	if request.Filter != nil {
		filter.FuzzyName = request.Filter.FuzzyName
		filter.Source = request.Filter.Source
		if request.Filter.Marked {
			filter.Marked = &request.Filter.Marked
		}
		if request.Filter.Unread {
			filter.Unread = &request.Filter.Unread
		}
		if request.Filter.Search != "" {
			filter.Search = request.Filter.Search
		}
		if request.Filter.CreatedAtStart != nil {
			createdStart := request.Filter.CreatedAtStart.AsTime()
			filter.CreatedAtStart = &createdStart
		}
		if request.Filter.CreatedAtEnd != nil {
			t := request.Filter.CreatedAtEnd.AsTime()
			filter.CreatedAtEnd = &t
		}
		if request.Filter.ChangedAtStart != nil {
			t := request.Filter.ChangedAtStart.AsTime()
			filter.ChangedAtStart = &t
		}
		if request.Filter.ChangedAtEnd != nil {
			t := request.Filter.ChangedAtEnd.AsTime()
			filter.ChangedAtEnd = &t
		}
	}
	if request.Pagination != nil {
		ctx = types.WithPagination(ctx, types.NewPagination(request.Pagination.Page, request.Pagination.PageSize))
	} else {
		ctx = types.WithPagination(ctx, types.NewPagination(1, 20))
	}
	order := types.DocumentOrder{
		Order: types.DocOrder(request.Order),
		Desc:  request.OrderDesc,
	}

	docList, err := s.doc.ListDocuments(ctx, filter, &order)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "filter document failed")
	}
	resp := &ListDocumentsResponse{Documents: make([]*DocumentInfo, 0, len(docList))}
	for _, doc := range docList {
		properties, err := s.queryEntryProperties(ctx, caller.Namespace, doc.EntryId, doc.ParentEntryID)
		if err != nil {
			return nil, status.Error(common.FsApiError(err), "query entry properties failed")
		}
		docInfo := documentInfo(doc)
		docInfo.Properties = properties
		parentEn, err := s.core.GetEntry(ctx, doc.Namespace, doc.ParentEntryID)
		if err != nil {
			return nil, status.Error(common.FsApiError(err), "query parent entry failed")
		}
		docInfo.Parent = entryInfo(parentEn)
		resp.Documents = append(resp.Documents, docInfo)
	}
	return resp, nil
}

func (s *servicesV1) GetDocumentParents(ctx context.Context, request *GetDocumentParentsRequest) (*GetDocumentParentsResponse, error) {
	caller := s.caller(ctx)
	if !caller.Authenticated {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}
	filter := types.DocFilter{}
	if request.Filter != nil {
		filter.FuzzyName = request.Filter.FuzzyName
		filter.Source = request.Filter.Source
		if request.Filter.Marked {
			filter.Marked = &request.Filter.Marked
		}
		if request.Filter.Unread {
			filter.Unread = &request.Filter.Unread
		}
		if request.Filter.CreatedAtStart != nil {
			createdStart := request.Filter.CreatedAtStart.AsTime()
			filter.CreatedAtStart = &createdStart
		}
		if request.Filter.CreatedAtEnd != nil {
			t := request.Filter.CreatedAtEnd.AsTime()
			filter.CreatedAtEnd = &t
		}
		if request.Filter.ChangedAtStart != nil {
			t := request.Filter.ChangedAtStart.AsTime()
			filter.ChangedAtStart = &t
		}
		if request.Filter.ChangedAtEnd != nil {
			t := request.Filter.ChangedAtEnd.AsTime()
			filter.ChangedAtEnd = &t
		}
	}
	entries, err := s.doc.ListDocumentGroups(ctx, request.ParentId, filter)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query groups of documents parent failed")
	}

	resp := &GetDocumentParentsResponse{}
	for _, en := range entries {
		resp.Entries = append(resp.Entries, entryInfo(en))
	}
	return resp, nil
}

func (s *servicesV1) GetDocumentDetail(ctx context.Context, request *GetDocumentDetailRequest) (*GetDocumentDetailResponse, error) {
	caller := s.caller(ctx)
	if !caller.Authenticated {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	if request.DocumentID == 0 && request.EntryID == 0 {
		return nil, status.Error(codes.InvalidArgument, "entry id and document are all empty")
	}
	var (
		doc        *types.Document
		en         *types.Entry
		properties []*Property
		err        error
	)
	if request.DocumentID != 0 {
		doc, err = s.doc.GetDocument(ctx, request.DocumentID)
		if err != nil {
			return nil, status.Error(common.FsApiError(err), "query document with document id failed")
		}
	}
	if request.EntryID != 0 {
		doc, err = s.doc.GetDocumentByEntryId(ctx, request.EntryID)
		if err != nil {
			return nil, status.Error(common.FsApiError(err), "query document with entry id failed")
		}
		properties, err = s.queryEntryProperties(ctx, caller.Namespace, request.EntryID, doc.ParentEntryID)
		if err != nil {
			return nil, status.Error(common.FsApiError(err), "query entry properties failed")
		}
	}
	if doc == nil {
		return nil, status.Error(common.FsApiError(err), "document not found")
	}

	en, err = s.core.GetEntry(ctx, doc.Namespace, doc.EntryId)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry of document failed")
	}

	return &GetDocumentDetailResponse{
		Document: &DocumentDescribe{
			Id:            doc.EntryId,
			Name:          doc.Name,
			EntryID:       doc.EntryId,
			ParentEntryID: doc.ParentEntryID,
			Source:        doc.Source,
			Marked:        *doc.Marked,
			Unread:        *doc.Unread,
			Namespace:     doc.Namespace,
			HtmlContent:   doc.Content,
			Summary:       doc.Summary,
			EntryInfo:     entryInfo(en),
			CreatedAt:     timestamppb.New(doc.CreatedAt),
			ChangedAt:     timestamppb.New(doc.ChangedAt),
		},
		Properties: properties,
	}, nil
}

func (s *servicesV1) UpdateDocument(ctx context.Context, request *UpdateDocumentRequest) (*UpdateDocumentResponse, error) {
	if request.Document == nil {
		return nil, status.Error(codes.InvalidArgument, "document is empty")
	}
	doc := request.Document
	if doc.Id == 0 {
		return nil, status.Error(codes.InvalidArgument, "document id is empty")
	}
	newDoc := &types.Document{
		EntryId:       doc.Id,
		Name:          doc.Name,
		ParentEntryID: doc.ParentEntryID,
		Source:        doc.Source,
		Content:       doc.HtmlContent,
		Summary:       doc.Summary,
	}
	t := true
	f := false
	switch request.SetMark {
	case UpdateDocumentRequest_Marked:
		newDoc.Marked = &t
	case UpdateDocumentRequest_Unmarked:
		newDoc.Marked = &f
	case UpdateDocumentRequest_Read:
		newDoc.Unread = &f
	case UpdateDocumentRequest_Unread:
		newDoc.Unread = &t
	}
	err := s.doc.UpdateDocument(ctx, newDoc)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "update document failed")
	}
	return &UpdateDocumentResponse{Document: doc}, nil
}

func (s *servicesV1) SearchDocuments(ctx context.Context, request *SearchDocumentsRequest) (*SearchDocumentsResponse, error) {
	if len(request.Query) == 0 {
		return nil, status.Error(codes.InvalidArgument, "query is empty")
	}
	docList, err := s.doc.QueryDocuments(ctx, request.Query)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "search document failed")
	}

	resp := &SearchDocumentsResponse{}
	for _, doc := range docList {
		resp.Documents = append(resp.Documents, &DocumentInfo{
			Id:            doc.EntryId,
			Name:          doc.Name,
			EntryID:       doc.EntryId,
			ParentEntryID: doc.ParentEntryID,
			Source:        doc.Source,
			Marked:        *doc.Marked,
			Unread:        *doc.Unread,
			Namespace:     doc.Namespace,
			SubContent:    doc.SubContent,
			HeaderImage:   doc.HeaderImage,
			CreatedAt:     timestamppb.New(doc.CreatedAt),
			ChangedAt:     timestamppb.New(doc.ChangedAt),
		})
	}
	return resp, nil
}

func (s *servicesV1) GroupTree(ctx context.Context, request *GetGroupTreeRequest) (*GetGroupTreeResponse, error) {
	caller := s.caller(ctx)
	if !caller.Authenticated {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	root, err := s.getGroupTree(ctx, caller.Namespace)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query root entry failed")
	}
	resp := &GetGroupTreeResponse{Root: root}
	return resp, nil
}

func (s *servicesV1) FindEntryDetail(ctx context.Context, request *FindEntryDetailRequest) (*GetEntryDetailResponse, error) {
	var (
		en  *types.Entry
		err error
	)

	caller := s.caller(ctx)
	if !caller.Authenticated {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	if !request.Root {
		return nil, status.Errorf(codes.InvalidArgument, "root entry only")
	}

	en, err = s.core.NamespaceRoot(ctx, caller.Namespace)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query root entry failed")
	}

	details, properties, err := s.getEntryDetails(ctx, caller.Namespace, en.ID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry details failed")
	}
	return &GetEntryDetailResponse{Entry: details, Properties: properties}, nil
}

func (s *servicesV1) GetEntryDetail(ctx context.Context, request *GetEntryDetailRequest) (*GetEntryDetailResponse, error) {
	caller := s.caller(ctx)
	if !caller.Authenticated {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}
	en, err := s.core.GetEntry(ctx, caller.Namespace, request.EntryID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}
	err = core.HasAllPermissions(en.Access, caller.UID, caller.GID, types.PermOwnerRead, types.PermGroupRead, types.PermOthersRead)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "has no permission")
	}

	if en.ParentID != core.RootEntryID {
		_, err = s.core.GetEntry(ctx, caller.Namespace, en.ParentID)
		if err != nil {
			return nil, status.Error(common.FsApiError(err), "query entry parent failed")
		}
	}

	detail, properties, err := s.getEntryDetails(ctx, caller.Namespace, en.ID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry details failed")
	}
	return &GetEntryDetailResponse{Entry: detail, Properties: properties}, nil
}

func (s *servicesV1) CreateEntry(ctx context.Context, request *CreateEntryRequest) (*CreateEntryResponse, error) {
	caller := s.caller(ctx)
	if !caller.Authenticated {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	if request.ParentID == 0 {
		return nil, status.Error(codes.InvalidArgument, "parent id is 0")
	}
	if len(request.Name) == 0 {
		return nil, status.Error(codes.InvalidArgument, "entry name is empty")
	}
	if request.Kind == "" {
		return nil, status.Error(codes.InvalidArgument, "entry has unknown kind")
	}

	parent, err := s.core.GetEntry(ctx, caller.Namespace, request.ParentID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry parent failed")
	}

	err = core.HasAllPermissions(parent.Access, caller.UID, caller.GID, types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "has no permission")
	}

	attr := types.EntryAttr{
		Name:   request.Name,
		Kind:   pdKind2EntryKind(request.Kind),
		Access: &parent.Access,
		Dev:    parent.Dev,
	}
	if request.Rss != nil {
		s.logger.Infow("setup rss feed to dir", "feed", request.Rss.Feed, "siteName", request.Rss.SiteName)
		setupRssConfig(request.Rss, &attr)
	}
	en, err := s.core.CreateEntry(ctx, caller.Namespace, request.ParentID, attr)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "create entry failed")
	}
	return &CreateEntryResponse{Entry: coreEntryInfo(parent.ID, request.Name, en)}, nil
}

func (s *servicesV1) UpdateEntry(ctx context.Context, request *UpdateEntryRequest) (*UpdateEntryResponse, error) {
	caller := s.caller(ctx)
	if !caller.Authenticated {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	if request.EntryID == 0 {
		return nil, status.Error(codes.InvalidArgument, "entry id is 0")
	}

	en, err := s.core.GetEntry(ctx, caller.Namespace, request.EntryID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}

	err = core.HasAllPermissions(en.Access, caller.UID, caller.GID, types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "has no permission")
	}

	_, err = s.core.GetEntry(ctx, caller.Namespace, en.ParentID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry parent failed")
	}

	update := types.UpdateEntry{}
	if request.Name != "" {
		update.Name = &request.Name
	}
	if request.Aliases != "" {
		update.Aliases = &request.Aliases
	}
	en, err = s.core.UpdateEntry(ctx, caller.Namespace, en.ID, update)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "update entry failed")
	}

	detail, _, err := s.getEntryDetails(ctx, caller.Namespace, en.ID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry detail failed")
	}
	return &UpdateEntryResponse{Entry: detail}, nil
}

func (s *servicesV1) DeleteEntry(ctx context.Context, request *DeleteEntryRequest) (*DeleteEntryResponse, error) {
	caller := s.caller(ctx)
	if !caller.Authenticated {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}
	en, err := s.deleteEntry(ctx, caller.Namespace, caller.UID, request.EntryID)
	if err != nil {
		return nil, err
	}
	return &DeleteEntryResponse{Entry: toEntryInfo(en)}, nil
}

func (s *servicesV1) deleteEntry(ctx context.Context, namespace string, uid, entryId int64) (en *types.Entry, err error) {
	en, err = s.core.GetEntry(ctx, namespace, entryId)
	if err != nil {
		err = status.Error(common.FsApiError(err), "query entry failed")
		return
	}
	err = core.HasAllPermissions(en.Access, uid, 0, types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite)
	if err != nil {
		err = status.Error(common.FsApiError(err), "has no permission")
		return
	}

	parent, err := s.core.GetEntry(ctx, namespace, en.ParentID)
	if err != nil {
		err = status.Error(common.FsApiError(err), "query entry parent failed")
		return
	}
	err = core.HasAllPermissions(parent.Access, uid, 0, types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite)
	if err != nil {
		err = status.Error(common.FsApiError(err), "has no permission")
		return
	}

	// TODO: fix gid
	var gid int64
	if err = core.IsAccess(parent.Access, uid, gid, 0x2); err != nil {
		return nil, types.ErrNoAccess
	}

	if uid != 0 && uid != en.Access.UID && uid != parent.Access.UID && parent.Access.HasPerm(types.PermSticky) {
		return nil, types.ErrNoAccess
	}

	s.logger.Debugw("destroy entry", "parent", en.ParentID, "entry", entryId)
	return en, s.core.RemoveEntry(ctx, namespace, en.ParentID, entryId)
}

func (s *servicesV1) DeleteEntries(ctx context.Context, request *DeleteEntriesRequest) (*DeleteEntriesResponse, error) {
	caller := s.caller(ctx)
	if !caller.Authenticated {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	entryIds := make([]int64, 0, len(request.EntryIDs))
	for _, entryId := range request.EntryIDs {
		en, err := s.deleteEntry(ctx, caller.Namespace, caller.UID, entryId)
		if err != nil {
			return nil, err
		}
		entryIds = append(entryIds, en.ID)
	}
	return &DeleteEntriesResponse{EntryIDs: entryIds}, nil
}

func (s *servicesV1) ListGroupChildren(ctx context.Context, request *ListGroupChildrenRequest) (*ListGroupChildrenResponse, error) {
	caller := s.caller(ctx)
	if !caller.Authenticated {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	if request.Pagination != nil {
		ctx = types.WithPagination(ctx, types.NewPagination(request.Pagination.Page, request.Pagination.PageSize))
	}
	filter := types.Filter{}
	if request.Filter != nil {
		filter.FuzzyName = request.Filter.FuzzyName
		filter.Kind = types.Kind(request.Filter.Kind)
		t := true
		f := false
		switch request.Filter.IsGroup {
		case EntryFilter_All:
			filter.IsGroup = nil
		case EntryFilter_Group:
			filter.IsGroup = &t
		case EntryFilter_File:
			filter.IsGroup = &f
		}
		if request.Filter.CreatedAtStart != nil {
			createdStart := request.Filter.CreatedAtStart.AsTime()
			filter.CreatedAtStart = &createdStart
		}
		if request.Filter.CreatedAtEnd != nil {
			ct := request.Filter.CreatedAtEnd.AsTime()
			filter.CreatedAtEnd = &ct
		}
		if request.Filter.ModifiedAtStart != nil {
			ct := request.Filter.ModifiedAtStart.AsTime()
			filter.ModifiedAtStart = &ct
		}
		if request.Filter.ModifiedAtEnd != nil {
			ct := request.Filter.ModifiedAtEnd.AsTime()
			filter.ModifiedAtEnd = &ct
		}
	}
	order := types.EntryOrder{
		Order: types.EnOrder(request.Order),
		Desc:  request.OrderDesc,
	}
	children, err := s.listEntryChildren(ctx, caller.Namespace, request.ParentID, &order, filter)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "list children failed")
	}

	resp := &ListGroupChildrenResponse{}
	for _, en := range children {
		resp.Entries = append(resp.Entries, toEntryInfo(en))
	}
	return resp, nil
}

func (s *servicesV1) ChangeParent(ctx context.Context, request *ChangeParentRequest) (*ChangeParentResponse, error) {
	caller := s.caller(ctx)
	if !caller.Authenticated {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	en, err := s.core.GetEntry(ctx, caller.Namespace, request.EntryID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}
	err = core.HasAllPermissions(en.Access, caller.UID, caller.GID, types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "has no permission")
	}

	newParent, err := s.core.GetEntry(ctx, caller.Namespace, request.NewParentID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}
	err = core.HasAllPermissions(newParent.Access, caller.UID, caller.GID, types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "has no permission")
	}

	var (
		existObjId *int64
		newName    = en.Name
	)
	if request.NewName != "" {
		newName = request.NewName
	}
	existObj, err := s.core.FindEntry(ctx, caller.Namespace, newParent.ID, newName)
	if err != nil {
		if !errors.Is(err, types.ErrNotFound) {
			return nil, status.Error(common.FsApiError(err), "check existing entry failed")
		}
	}
	if existObj != nil {
		existObjId = &existObj.ID
	}

	if request.Option == nil {
		request.Option = &ChangeParentRequest_Option{}
	}
	err = s.core.ChangeEntryParent(ctx, caller.Namespace, request.EntryID, existObjId, en.ParentID, request.NewParentID, request.NewName, types.ChangeParentAttr{
		Uid:      caller.UID,
		Gid:      caller.GID,
		Replace:  request.Option.Replace,
		Exchange: request.Option.Exchange,
	})
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "change entry parent failed")
	}

	en, err = s.core.GetEntry(ctx, caller.Namespace, request.EntryID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}
	return &ChangeParentResponse{Entry: toEntryInfo(en)}, nil
}

func (s *servicesV1) WriteFile(reader Entries_WriteFileServer) error {
	var (
		ctx       = reader.Context()
		recvTotal int64
		accessEn  int64
		file      core.RawFile
	)

	caller := s.caller(ctx)
	if !caller.Authenticated {
		return status.Error(codes.Unauthenticated, "unauthenticated")
	}

	defer func() {
		if file != nil {
			_ = file.Close(context.Background())
		}
	}()

	for {
		writeRequest, err := reader.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
		}

		if len(writeRequest.Data) == 0 || writeRequest.Len == 0 {
			break
		}

		if writeRequest.EntryID != accessEn {
			s.logger.Debugw("handle write data to file", "entry", writeRequest.EntryID)
			en, err := s.core.GetEntry(ctx, caller.Namespace, writeRequest.EntryID)
			if err != nil {
				return status.Error(common.FsApiError(err), "query entry failed")
			}

			err = core.HasAllPermissions(en.Access, caller.UID, caller.GID, types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite)
			if err != nil {
				return status.Error(common.FsApiError(err), "has no permission")
			}
			accessEn = writeRequest.EntryID
			s.logger.Infow("start to write data", "entry", accessEn)
		}

		if file == nil {
			file, err = s.core.Open(ctx, caller.Namespace, accessEn, types.OpenAttr{Write: true})
			if err != nil {
				s.logger.Errorw("open file error", "entry", accessEn, "err", err)
				return status.Error(common.FsApiError(err), "open file failed")
			}
		}

		_, err = file.WriteAt(ctx, writeRequest.Data[:writeRequest.Len], writeRequest.Off)
		if err != nil {
			return status.Error(common.FsApiError(err), "write file failed")
		}
		recvTotal += writeRequest.Len
	}

	if file != nil {
		if err := file.Flush(ctx); err != nil {
			s.logger.Errorw("flush data to file error", "entry", accessEn, "recv", recvTotal, "err", err)
			return err
		}
	}
	return reader.SendAndClose(&WriteFileResponse{Len: recvTotal})
}

func (s *servicesV1) ReadFile(request *ReadFileRequest, writer Entries_ReadFileServer) error {
	var (
		ctx      = writer.Context()
		off      = request.Off
		readOnce int64
	)

	caller := s.caller(ctx)
	if !caller.Authenticated {
		return status.Error(codes.Unauthenticated, "unauthenticated")
	}

	en, err := s.core.GetEntry(ctx, caller.Namespace, request.EntryID)
	if err != nil {
		return status.Error(common.FsApiError(err), "query entry failed")
	}

	err = core.HasAllPermissions(en.Access, caller.UID, caller.GID, types.PermOwnerRead, types.PermGroupRead, types.PermOthersRead)
	if err != nil {
		return status.Error(common.FsApiError(err), "has no permission")
	}

	file, err := s.core.Open(ctx, caller.Namespace, request.EntryID, types.OpenAttr{Read: true})
	if err != nil {
		return status.Error(common.FsApiError(err), "open file failed")
	}
	defer file.Close(context.Background())

	buffer := make([]byte, request.Len)
	for {
		readOnce, err = file.ReadAt(ctx, buffer, off)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		err = writer.Send(&ReadFileResponse{
			Off:  off,
			Len:  readOnce,
			Data: buffer[:readOnce],
		})
		if err != nil {
			return err
		}
		off += readOnce
	}
	return nil
}

func (s *servicesV1) AddProperty(ctx context.Context, request *AddPropertyRequest) (*AddPropertyResponse, error) {
	caller := s.caller(ctx)
	if !caller.Authenticated {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	en, err := s.core.GetEntry(ctx, caller.Namespace, request.EntryID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}
	err = s.meta.AddEntryProperty(ctx, caller.Namespace, en.ID, request.Key, types.PropertyItem{Value: request.Value, Encoded: false})
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "add entry extend field failed")
	}

	en, err = s.core.GetEntry(ctx, caller.Namespace, request.EntryID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}

	properties, err := s.queryEntryProperties(ctx, caller.Namespace, en.ID, en.ParentID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry properties failed")
	}
	return &AddPropertyResponse{
		Entry:      toEntryInfo(en),
		Properties: properties,
	}, nil
}

func (s *servicesV1) UpdateProperty(ctx context.Context, request *UpdatePropertyRequest) (*UpdatePropertyResponse, error) {
	caller := s.caller(ctx)
	if !caller.Authenticated {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	en, err := s.core.GetEntry(ctx, caller.Namespace, request.EntryID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}

	err = s.meta.AddEntryProperty(ctx, caller.Namespace, en.ID, request.Key, types.PropertyItem{Value: request.Value, Encoded: false})
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "set entry extend field failed")
	}

	en, err = s.core.GetEntry(ctx, caller.Namespace, request.EntryID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}
	properties, err := s.queryEntryProperties(ctx, caller.Namespace, en.ID, en.ParentID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry properties failed")
	}

	return &UpdatePropertyResponse{
		Entry:      toEntryInfo(en),
		Properties: properties,
	}, nil
}

func (s *servicesV1) DeleteProperty(ctx context.Context, request *DeletePropertyRequest) (*DeletePropertyResponse, error) {
	caller := s.caller(ctx)
	if !caller.Authenticated {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	en, err := s.core.GetEntry(ctx, caller.Namespace, request.EntryID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}
	err = s.meta.RemoveEntryProperty(ctx, caller.Namespace, en.ID, request.Key)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "set entry extend field failed")
	}

	en, err = s.core.GetEntry(ctx, caller.Namespace, request.EntryID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}
	properties, err := s.queryEntryProperties(ctx, caller.Namespace, en.ID, en.ParentID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry properties failed")
	}

	return &DeletePropertyResponse{
		Entry:      toEntryInfo(en),
		Properties: properties,
	}, nil
}

func (s *servicesV1) ListMessages(ctx context.Context, request *ListMessagesRequest) (*ListMessagesResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s *servicesV1) ReadMessages(ctx context.Context, request *ReadMessagesRequest) (*ReadMessagesResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s *servicesV1) ListWorkflows(ctx context.Context, request *ListWorkflowsRequest) (*ListWorkflowsResponse, error) {
	caller := s.caller(ctx)
	if !caller.Authenticated {
		return nil, status.Errorf(codes.Unauthenticated, "unauthenticated")
	}

	workflowList, err := s.workflow.ListWorkflows(ctx, caller.Namespace)
	if err != nil {
		s.logger.Errorw("list workflow failed", "err", err)
		return nil, status.Error(common.FsApiError(err), "list workflow failed")
	}

	resp := &ListWorkflowsResponse{}
	for _, w := range workflowList {
		resp.Workflows = append(resp.Workflows, buildWorkflow(w))
	}
	return resp, nil
}

func (s *servicesV1) ListWorkflowJobs(ctx context.Context, request *ListWorkflowJobsRequest) (*ListWorkflowJobsResponse, error) {
	caller := s.caller(ctx)
	if !caller.Authenticated {
		return nil, status.Errorf(codes.Unauthenticated, "unauthenticated")
	}

	if request.WorkflowID == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid workflow id")
	}

	jobs, err := s.workflow.ListJobs(ctx, caller.Namespace, request.WorkflowID)
	if err != nil {
		s.logger.Errorw("list workflow job failed", "err", err)
		return nil, status.Error(common.FsApiError(err), "list workflow job failed")
	}

	resp := &ListWorkflowJobsResponse{}
	for _, j := range jobs {
		resp.Jobs = append(resp.Jobs, jobDetail(j))
	}
	return resp, nil
}

func (s *servicesV1) TriggerWorkflow(ctx context.Context, request *TriggerWorkflowRequest) (*TriggerWorkflowResponse, error) {
	caller := s.caller(ctx)
	if !caller.Authenticated {
		return nil, status.Errorf(codes.Unauthenticated, "unauthenticated")
	}

	if request.WorkflowID == "" {
		return nil, status.Error(codes.InvalidArgument, "workflow id is empty")
	}
	if request.Target == nil {
		return nil, status.Error(codes.InvalidArgument, "workflow target is empty")
	}

	s.logger.Infow("trigger workflow", "workflow", request.WorkflowID)
	if _, err := s.workflow.GetWorkflow(ctx, caller.Namespace, request.WorkflowID); err != nil {
		return nil, status.Error(common.FsApiError(err), "fetch workflow failed")
	}

	if request.Attr == nil {
		request.Attr = &TriggerWorkflowRequest_WorkflowJobAttr{
			Reason:  "fsapi trigger",
			Timeout: 60 * 10, // default 10min
		}
	}

	job, err := s.workflow.TriggerWorkflow(ctx, caller.Namespace, request.WorkflowID,
		types.WorkflowTarget{Entries: []int64{request.Target.EntryID}, ParentEntryID: request.Target.ParentEntryID},
		workflow.JobAttr{Reason: request.Attr.Reason, Timeout: time.Second * time.Duration(request.Attr.Timeout)},
	)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "trigger workflow failed")
	}
	return &TriggerWorkflowResponse{JobID: job.Id}, nil
}
