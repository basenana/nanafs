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
	"fmt"
	"io"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/basenana/nanafs/pkg/workflow"

	"github.com/basenana/nanafs/cmd/apps/apis/fsapi/common"
	"github.com/basenana/nanafs/cmd/apps/apis/pathmgr"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/inbox"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
)

type Services interface {
	AuthServer
	DocumentServer
	EntriesServer
	InboxServer
	PropertiesServer
	NotifyServer
	WorkflowServer
}

func InitServices(server *grpc.Server, ctrl controller.Controller, pathEntryMgr *pathmgr.PathManager) (Services, error) {
	s := &services{
		ctrl:         ctrl,
		pathEntryMgr: pathEntryMgr,
		callerAuthFn: callerAuthGetter,
		logger:       logger.NewLogger("fsapi"),
	}

	RegisterAuthServer(server, s)
	RegisterDocumentServer(server, s)
	RegisterEntriesServer(server, s)
	RegisterInboxServer(server, s)
	RegisterRoomServer(server, s)
	RegisterPropertiesServer(server, s)
	RegisterNotifyServer(server, s)
	RegisterWorkflowServer(server, s)
	return s, nil
}

type services struct {
	ctrl         controller.Controller
	pathEntryMgr *pathmgr.PathManager
	callerAuthFn common.CallerAuthGetter
	logger       *zap.SugaredLogger
}

func (s *services) ListRooms(ctx context.Context, request *ListRoomsRequest) (*ListRoomsResponse, error) {
	if request.EntryID == 0 {
		return nil, status.Error(codes.InvalidArgument, "entry id is empty")
	}
	rooms, err := s.ctrl.ListRooms(ctx, request.EntryID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "list rooms failed")
	}
	resp := &ListRoomsResponse{Rooms: make([]*RoomInfo, 0, len(rooms))}
	for _, room := range rooms {
		resp.Rooms = append(resp.Rooms, &RoomInfo{
			Id:        room.ID,
			EntryID:   room.EntryId,
			Title:     room.Title,
			Prompt:    room.Prompt,
			Namespace: room.Namespace,
			CreatedAt: timestamppb.New(room.CreatedAt),
		})
	}
	return resp, nil
}

func (s *services) OpenRoom(ctx context.Context, request *OpenRoomRequest) (*OpenRoomResponse, error) {
	if request.EntryID == 0 {
		return nil, status.Error(codes.InvalidArgument, "entry id is empty")
	}

	room, err := s.ctrl.FindRoom(ctx, request.EntryID)
	if err != nil {
		if err == types.ErrNotFound {
			// need create a new one
			prompt := ""
			if request.Option != nil {
				prompt = request.Option.Prompt
			}
			room, err := s.ctrl.CreateRoom(ctx, request.EntryID, prompt)
			if err != nil {
				return nil, status.Error(common.FsApiError(err), "create room failed")
			}
			return &OpenRoomResponse{
				Room: &RoomInfo{Id: room.ID, EntryID: room.EntryId, Namespace: room.Namespace, Title: room.Title, Prompt: room.Prompt, CreatedAt: timestamppb.New(room.CreatedAt)},
			}, nil
		}
		return nil, status.Error(common.FsApiError(err), "get room failed")
	}

	msg := make([]*RoomMessage, 0, len(room.Messages))
	for _, m := range room.Messages {
		msg = append(msg, &RoomMessage{
			Id:        m.ID,
			Namespace: m.Namespace,
			RoomID:    m.RoomID,
			Sender:    m.Sender,
			Message:   m.Message,
			SendAt:    timestamppb.New(m.SendAt),
			CreatedAt: timestamppb.New(m.CreatedAt),
		})
	}
	return &OpenRoomResponse{
		Room: &RoomInfo{Id: room.ID, EntryID: room.EntryId, Namespace: room.Namespace, Title: room.Title, Prompt: room.Prompt, CreatedAt: timestamppb.New(room.CreatedAt), Messages: msg},
	}, nil
}

func (s *services) UpdateRoom(ctx context.Context, request *UpdateRoomRequest) (*UpdateRoomResponse, error) {
	if request.RoomID == 0 {
		return nil, status.Error(codes.InvalidArgument, "entry id is empty")
	}
	err := s.ctrl.UpdateRoom(ctx, request.RoomID, request.Prompt)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "update room failed")
	}
	return &UpdateRoomResponse{RoomID: request.RoomID}, nil
}

func (s *services) ChatInRoom(request *ChatRequest, server Room_ChatInRoomServer) error {
	if request.RoomID == 0 {
		return status.Error(codes.InvalidArgument, "room id is empty")
	}
	if request.NewRequest == "" {
		return status.Error(codes.InvalidArgument, "message is empty")
	}

	// save user message
	msg, err := s.ctrl.CreateRoomMessage(server.Context(), request.RoomID, "user", request.NewRequest, request.SendAt.AsTime())
	if err != nil {
		return status.Error(common.FsApiError(err), "create message failed")
	}

	if err := server.Send(&ChatResponse{
		RequestID:       msg.ID,
		ResponseID:      0,
		ResponseMessage: "",
		SendAt:          nil,
		CreatedAt:       nil,
	}); err != nil {
		return status.Error(common.FsApiError(err), "send message error")
	}

	var (
		timeout       = time.Minute * 10
		ctx, timeoutF = context.WithTimeout(server.Context(), timeout)
		chatCh        = make(chan types.ReplyChannel)
		errCh         = make(chan error, 1)
	)
	defer timeoutF()

	go func() {
		defer close(errCh)
		errCh <- s.ctrl.ChatInRoom(ctx, request.RoomID, request.NewRequest, chatCh)
	}()
	for {
		select {
		case <-ctx.Done():
			err = errors.New("chat in room timeout")
			return status.Error(common.FsApiError(err), "context timeout")
		case err = <-errCh:
			if err != nil {
				return status.Error(common.FsApiError(err), "chat in room failed")
			}
		case reply, ok := <-chatCh:
			if !ok {
				return nil
			}
			if err = server.Send(&ChatResponse{
				ResponseID:      reply.ResponseId,
				ResponseMessage: reply.Line,
				Sender:          reply.Sender,
				SendAt:          timestamppb.New(reply.SendAt),
				CreatedAt:       timestamppb.New(reply.CreatedAt),
			}); err != nil {
				return status.Error(common.FsApiError(err), "send message error")
			}
		}
	}
}

func (s *services) DeleteRoom(ctx context.Context, request *DeleteRoomRequest) (*DeleteRoomResponse, error) {
	if request.RoomID == 0 {
		return nil, status.Error(codes.InvalidArgument, "entry id is empty")
	}
	err := s.ctrl.DeleteRoom(ctx, request.RoomID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "delete room failed")
	}
	return &DeleteRoomResponse{RoomID: request.RoomID}, nil
}

func (s *services) ClearRoom(ctx context.Context, request *ClearRoomRequest) (*ClearRoomResponse, error) {
	if request.RoomID == 0 {
		return nil, status.Error(codes.InvalidArgument, "entry id is empty")
	}
	err := s.ctrl.ClearRoom(ctx, request.RoomID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "clear room failed")
	}
	return &ClearRoomResponse{RoomID: request.RoomID}, nil
}

var _ Services = &services{}

func (s *services) AccessToken(ctx context.Context, request *AccessTokenRequest) (*AccessTokenResponse, error) {
	token, err := s.ctrl.AccessToken(ctx, request.AccessTokenKey, request.SecretToken)
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

func (s *services) ListDocuments(ctx context.Context, request *ListDocumentsRequest) (*ListDocumentsResponse, error) {
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
	docList, err := s.ctrl.ListDocuments(ctx, filter, &order)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "filter document failed")
	}
	resp := &ListDocumentsResponse{Documents: make([]*DocumentInfo, 0, len(docList))}
	for _, doc := range docList {
		properties, err := s.queryEntryProperties(ctx, doc.EntryId, doc.ParentEntryID)
		if err != nil {
			return nil, status.Error(common.FsApiError(err), "query entry properties failed")
		}
		docInfo := documentInfo(doc)
		docInfo.Properties = properties
		parentEn, err := s.ctrl.GetEntry(ctx, doc.ParentEntryID)
		if err != nil {
			return nil, status.Error(common.FsApiError(err), "query parent entry failed")
		}
		docInfo.Parent = entryInfo(parentEn)
		resp.Documents = append(resp.Documents, docInfo)
	}
	return resp, nil
}

func (s *services) GetDocumentParents(ctx context.Context, request *GetDocumentParentsRequest) (*GetDocumentParentsResponse, error) {
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
	entries, err := s.ctrl.ListDocumentGroups(ctx, request.ParentId, filter)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query groups of documents parent failed")
	}

	resp := &GetDocumentParentsResponse{}
	for _, en := range entries {
		resp.Entries = append(resp.Entries, entryInfo(en))
	}
	return resp, nil
}

func (s *services) GetDocumentDetail(ctx context.Context, request *GetDocumentDetailRequest) (*GetDocumentDetailResponse, error) {
	if request.DocumentID == 0 && request.EntryID == 0 {
		return nil, status.Error(codes.InvalidArgument, "entry id and document are all empty")
	}
	var (
		doc        *types.Document
		en         *types.Metadata
		properties []*Property
		err        error
	)
	if request.DocumentID != 0 {
		doc, err = s.ctrl.GetDocument(ctx, request.DocumentID)
		if err != nil {
			return nil, status.Error(common.FsApiError(err), "query document with document id failed")
		}
	}
	if request.EntryID != 0 {
		doc, err = s.ctrl.GetDocumentsByEntryId(ctx, request.EntryID)
		if err != nil {
			return nil, status.Error(common.FsApiError(err), "query document with entry id failed")
		}
		properties, err = s.queryEntryProperties(ctx, request.EntryID, doc.ParentEntryID)
		if err != nil {
			return nil, status.Error(common.FsApiError(err), "query entry properties failed")
		}
	}
	if doc != nil {
		en, err = s.ctrl.GetEntry(ctx, doc.EntryId)
		if err != nil {
			return nil, status.Error(common.FsApiError(err), "query entry of document failed")
		}
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

func (s *services) UpdateDocument(ctx context.Context, request *UpdateDocumentRequest) (*UpdateDocumentResponse, error) {
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
	err := s.ctrl.UpdateDocument(ctx, newDoc)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "update document failed")
	}
	return &UpdateDocumentResponse{Document: doc}, nil
}

func (s *services) SearchDocuments(ctx context.Context, request *SearchDocumentsRequest) (*SearchDocumentsResponse, error) {
	if len(request.Query) == 0 {
		return nil, status.Error(codes.InvalidArgument, "query is empty")
	}
	docList, err := s.ctrl.QueryDocuments(ctx, request.Query)
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

func (s *services) GroupTree(ctx context.Context, request *GetGroupTreeRequest) (*GetGroupTreeResponse, error) {
	root, err := s.ctrl.GetGroupTree(ctx)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query root entry failed")
	}
	resp := &GetGroupTreeResponse{}
	resp.Root = buildRootGroup(root)
	return resp, nil
}

func (s *services) FindEntryDetail(ctx context.Context, request *FindEntryDetailRequest) (*GetEntryDetailResponse, error) {
	var (
		en, par *types.Metadata
		err     error
	)

	if request.Root {
		en, err = s.ctrl.LoadRootEntry(ctx)
		if err != nil {
			return nil, status.Error(common.FsApiError(err), "query root entry failed")
		}
	} else {
		par, err = s.ctrl.GetEntry(ctx, request.ParentID)
		if err != nil {
			return nil, status.Error(common.FsApiError(err), fmt.Sprintf("query parent entry %d failed", request.ParentID))
		}

		en, err = s.ctrl.FindEntry(ctx, request.ParentID, request.Name)
		if err != nil {
			return nil, status.Error(common.FsApiError(err), fmt.Sprintf("find child entry %s failed", request.Name))
		}
	}

	properties, err := s.queryEntryProperties(ctx, en.ID, en.ParentID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry properties failed")
	}
	return &GetEntryDetailResponse{Entry: entryDetail(en, par), Properties: properties}, nil
}

func (s *services) GetEntryDetail(ctx context.Context, request *GetEntryDetailRequest) (*GetEntryDetailResponse, error) {
	caller := s.callerAuthFn(ctx)
	if !caller.Authenticated {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}
	en, err := s.ctrl.GetEntry(ctx, request.EntryID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}
	err = dentry.IsHasPermissions(en.Access, caller.UID, caller.GID, []types.Permission{types.PermOwnerRead, types.PermGroupRead, types.PermOthersRead})
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "has no permission")
	}

	var p *types.Metadata
	if en.ParentID != dentry.RootEntryID {
		p, err = s.ctrl.GetEntry(ctx, en.ParentID)
		if err != nil {
			return nil, status.Error(common.FsApiError(err), "query entry parent failed")
		}
	}

	properties, err := s.queryEntryProperties(ctx, en.ID, en.ParentID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry properties failed")
	}
	return &GetEntryDetailResponse{Entry: entryDetail(en, p), Properties: properties}, nil
}

func (s *services) CreateEntry(ctx context.Context, request *CreateEntryRequest) (*CreateEntryResponse, error) {
	caller := s.callerAuthFn(ctx)
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

	parent, err := s.ctrl.GetEntry(ctx, request.ParentID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry parent failed")
	}

	err = dentry.IsHasPermissions(parent.Access, caller.UID, caller.GID, []types.Permission{types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite})
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
	en, err := s.ctrl.CreateEntry(ctx, request.ParentID, attr)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "create entry failed")
	}
	return &CreateEntryResponse{Entry: entryInfo(en)}, nil
}

func (s *services) UpdateEntry(ctx context.Context, request *UpdateEntryRequest) (*UpdateEntryResponse, error) {
	caller := s.callerAuthFn(ctx)
	if !caller.Authenticated {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	if request.Entry.Id == 0 {
		return nil, status.Error(codes.InvalidArgument, "entry id is 0")
	}
	newEn := request.Entry

	en, err := s.ctrl.GetEntry(ctx, request.Entry.Id)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}

	err = dentry.IsHasPermissions(en.Access, caller.UID, caller.GID, []types.Permission{types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite})
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "has no permission")
	}

	parent, err := s.ctrl.GetEntry(ctx, en.ParentID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry parent failed")
	}

	// TODO: do update
	en.Aliases = newEn.Aliases

	err = s.ctrl.UpdateEntry(ctx, en)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "update entry failed")
	}
	return &UpdateEntryResponse{Entry: entryDetail(en, parent)}, nil
}

func (s *services) DeleteEntry(ctx context.Context, request *DeleteEntryRequest) (*DeleteEntryResponse, error) {
	caller := s.callerAuthFn(ctx)
	if !caller.Authenticated {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}
	en, parent, err := s.deleteEntry(ctx, caller.UID, request.EntryID)
	if err != nil {
		return nil, err
	}
	return &DeleteEntryResponse{Entry: entryDetail(en, parent)}, nil
}

func (s *services) deleteEntry(ctx context.Context, uid, entryId int64) (en, parent *types.Metadata, err error) {
	en, err = s.ctrl.GetEntry(ctx, entryId)
	if err != nil {
		err = status.Error(common.FsApiError(err), "query entry failed")
		return
	}
	err = dentry.IsHasPermissions(en.Access, uid, 0, []types.Permission{types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite})
	if err != nil {
		err = status.Error(common.FsApiError(err), "has no permission")
		return
	}

	parent, err = s.ctrl.GetEntry(ctx, en.ParentID)
	if err != nil {
		err = status.Error(common.FsApiError(err), "query entry parent failed")
		return
	}
	err = dentry.IsHasPermissions(parent.Access, uid, 0, []types.Permission{types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite})
	if err != nil {
		err = status.Error(common.FsApiError(err), "has no permission")
		return
	}

	err = s.ctrl.DestroyEntry(ctx, parent.ID, en.ID, types.DestroyObjectAttr{Uid: 0, Gid: 0})
	if err != nil {
		err = status.Error(common.FsApiError(err), "delete entry failed")
		return
	}
	return
}

func (s *services) DeleteEntries(ctx context.Context, request *DeleteEntriesRequest) (*DeleteEntriesResponse, error) {
	caller := s.callerAuthFn(ctx)
	if !caller.Authenticated {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	entryIds := make([]int64, 0, len(request.EntryIDs))
	for _, entryId := range request.EntryIDs {
		en, _, err := s.deleteEntry(ctx, caller.UID, entryId)
		if err != nil {
			return nil, err
		}
		entryIds = append(entryIds, en.ID)
	}
	return &DeleteEntriesResponse{EntryIDs: entryIds}, nil
}

func (s *services) ListGroupChildren(ctx context.Context, request *ListGroupChildrenRequest) (*ListGroupChildrenResponse, error) {
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
	children, err := s.ctrl.ListEntryChildren(ctx, request.ParentID, &order, filter)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "list children failed")
	}

	resp := &ListGroupChildrenResponse{}
	for _, en := range children {
		resp.Entries = append(resp.Entries, entryInfo(en))
	}
	return resp, nil
}

func (s *services) ChangeParent(ctx context.Context, request *ChangeParentRequest) (*ChangeParentResponse, error) {
	caller := s.callerAuthFn(ctx)
	if !caller.Authenticated {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	en, err := s.ctrl.GetEntry(ctx, request.EntryID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}
	err = dentry.IsHasPermissions(en.Access, caller.UID, caller.GID, []types.Permission{types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite})
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "has no permission")
	}

	newParent, err := s.ctrl.GetEntry(ctx, request.NewParentID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}
	err = dentry.IsHasPermissions(newParent.Access, caller.UID, caller.GID, []types.Permission{types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite})
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "has no permission")
	}

	if request.Option == nil {
		request.Option = &ChangeParentRequest_Option{}
	}
	err = s.ctrl.ChangeEntryParent(ctx, request.EntryID, en.ParentID, request.NewParentID, request.NewName, types.ChangeParentAttr{
		Uid:      caller.UID,
		Gid:      caller.GID,
		Replace:  request.Option.Replace,
		Exchange: request.Option.Exchange,
	})
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "change entry parent failed")
	}

	en, err = s.ctrl.GetEntry(ctx, request.EntryID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}
	return &ChangeParentResponse{Entry: entryInfo(en)}, nil
}

func (s *services) WriteFile(reader Entries_WriteFileServer) error {
	var (
		ctx       = reader.Context()
		recvTotal int64
		accessEn  int64
		file      dentry.File
	)

	caller := s.callerAuthFn(ctx)
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
			en, err := s.ctrl.GetEntry(ctx, writeRequest.EntryID)
			if err != nil {
				return status.Error(common.FsApiError(err), "query entry failed")
			}

			err = dentry.IsHasPermissions(en.Access, caller.UID, caller.GID, []types.Permission{types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite})
			if err != nil {
				return status.Error(common.FsApiError(err), "has no permission")
			}
			accessEn = writeRequest.EntryID
			s.logger.Infow("start to write data", "entry", accessEn)
		}

		if file == nil {
			file, err = s.ctrl.OpenFile(ctx, accessEn, types.OpenAttr{Write: true})
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

func (s *services) ReadFile(request *ReadFileRequest, writer Entries_ReadFileServer) error {
	var (
		ctx      = writer.Context()
		off      = request.Off
		readOnce int64
	)

	caller := s.callerAuthFn(ctx)
	if !caller.Authenticated {
		return status.Error(codes.Unauthenticated, "unauthenticated")
	}

	en, err := s.ctrl.GetEntry(ctx, request.EntryID)
	if err != nil {
		return status.Error(common.FsApiError(err), "query entry failed")
	}

	err = dentry.IsHasPermissions(en.Access, caller.UID, caller.GID, []types.Permission{types.PermOwnerRead, types.PermGroupRead, types.PermOthersRead})
	if err != nil {
		return status.Error(common.FsApiError(err), "has no permission")
	}

	file, err := s.ctrl.OpenFile(ctx, request.EntryID, types.OpenAttr{Read: true})
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

func (s *services) QuickInbox(ctx context.Context, request *QuickInboxRequest) (*QuickInboxResponse, error) {
	option := inbox.Option{
		Url:         request.Url,
		Data:        request.Data,
		ClutterFree: request.ClutterFree,
	}
	switch request.FileType {
	case WebFileType_BookmarkFile:
		option.FileType = "url"
	case WebFileType_WebArchiveFile:
		option.FileType = "webarchive"
	case WebFileType_HtmlFile:
		option.FileType = "html"
	}
	en, err := s.ctrl.QuickInbox(ctx, request.Filename, option)
	if err != nil {
		s.logger.Errorw("quick inbox failed", "err", err)
		return nil, status.Error(common.FsApiError(err), "quick inbox failed")
	}
	return &QuickInboxResponse{EntryID: en.ID}, nil
}

func (s *services) AddProperty(ctx context.Context, request *AddPropertyRequest) (*AddPropertyResponse, error) {
	en, err := s.ctrl.GetEntry(ctx, request.EntryID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}
	err = s.ctrl.SetEntryProperty(ctx, en.ID, request.Key, request.Value)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "add entry extend field failed")
	}
	properties, err := s.queryEntryProperties(ctx, en.ID, en.ParentID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry properties failed")
	}
	return &AddPropertyResponse{
		Entry:      entryInfo(en),
		Properties: properties,
	}, nil
}

func (s *services) UpdateProperty(ctx context.Context, request *UpdatePropertyRequest) (*UpdatePropertyResponse, error) {
	en, err := s.ctrl.GetEntry(ctx, request.EntryID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}

	_, err = s.ctrl.GetEntryProperty(ctx, en.ID, request.Key)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "fetch entry exist extend field failed")
	}

	err = s.ctrl.SetEntryProperty(ctx, en.ID, request.Key, request.Value)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "set entry extend field failed")
	}
	properties, err := s.queryEntryProperties(ctx, en.ID, en.ParentID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry properties failed")
	}
	return &UpdatePropertyResponse{
		Entry:      entryInfo(en),
		Properties: properties,
	}, nil
}

func (s *services) DeleteProperty(ctx context.Context, request *DeletePropertyRequest) (*DeletePropertyResponse, error) {
	en, err := s.ctrl.GetEntry(ctx, request.EntryID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}
	err = s.ctrl.RemoveEntryProperty(ctx, en.ID, request.Key)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "set entry extend field failed")
	}
	properties, err := s.queryEntryProperties(ctx, en.ID, en.ParentID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry properties failed")
	}
	return &DeletePropertyResponse{
		Entry:      entryInfo(en),
		Properties: properties,
	}, nil
}

func (s *services) GetLatestSequence(ctx context.Context, request *GetLatestSequenceRequest) (*GetLatestSequenceResponse, error) {
	caller := s.callerAuthFn(ctx)
	if !caller.Authenticated {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}
	seq, err := s.ctrl.GetLatestSequence(ctx)
	if err != nil {
		s.logger.Errorw("query latest sequence failed", "err", err)
		return nil, status.Error(common.FsApiError(err), "query latest sequence failed")
	}
	return &GetLatestSequenceResponse{
		Sequence:   seq,
		NeedRelist: request.StartSequence <= 0,
	}, nil
}

func (s *services) ListMessages(ctx context.Context, request *ListMessagesRequest) (*ListMessagesResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s *services) ReadMessages(ctx context.Context, request *ReadMessagesRequest) (*ReadMessagesResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s *services) ListUnSyncedEvent(ctx context.Context, request *ListUnSyncedEventRequest) (*ListUnSyncedEventResponse, error) {
	eventList, err := s.ctrl.ListUnSyncedEvent(ctx, request.StartSequence)
	if err != nil {
		s.logger.Errorw("list unsynced event failed", "err", err)
		return nil, status.Error(common.FsApiError(err), "list unsynced event failed")
	}

	resp := &ListUnSyncedEventResponse{}
	for _, evt := range eventList {
		resp.Events = append(resp.Events, eventInfo(&evt))
	}
	return resp, nil
}

func (s *services) CommitSyncedEvent(ctx context.Context, request *CommitSyncedEventRequest) (*CommitSyncedEventResponse, error) {
	s.logger.Infow("device commit sequence", "device", request.DeviceID, "sequence", request.Sequence)
	if request.DeviceID == "" {
		return nil, status.Error(codes.InvalidArgument, "device id is empty")
	}
	err := s.ctrl.CommitSyncedEvent(ctx, request.DeviceID, request.Sequence)
	if err != nil {
		s.logger.Errorw("device commit sequence failed", "device", request.DeviceID, "err", err)
		return nil, status.Error(common.FsApiError(err), "device commit sequence failed")
	}
	return &CommitSyncedEventResponse{}, nil
}

func (s *services) ListWorkflows(ctx context.Context, request *ListWorkflowsRequest) (*ListWorkflowsResponse, error) {
	workflowList, err := s.ctrl.ListWorkflows(ctx)
	if err != nil {
		s.logger.Errorw("list workflow failed", "err", err)
		return nil, status.Error(common.FsApiError(err), "list workflow failed")
	}

	buildInWorkflow := workflow.BuildInWorkflows()
	workflowList = append(workflowList, buildInWorkflow...)

	resp := &ListWorkflowsResponse{}
	for _, w := range workflowList {
		resp.Workflows = append(resp.Workflows, buildWorkflow(w))
	}
	return resp, nil
}

func (s *services) ListWorkflowJobs(ctx context.Context, request *ListWorkflowJobsRequest) (*ListWorkflowJobsResponse, error) {
	if request.WorkflowID == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid workflow id")
	}
	jobs, err := s.ctrl.ListJobs(ctx, request.WorkflowID)
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

func (s *services) TriggerWorkflow(ctx context.Context, request *TriggerWorkflowRequest) (*TriggerWorkflowResponse, error) {
	s.logger.Infow("trigger workflow", "workflow", request.WorkflowID)

	if request.WorkflowID == "" {
		return nil, status.Error(codes.InvalidArgument, "workflow id is empty")
	}
	if request.Target == nil {
		return nil, status.Error(codes.InvalidArgument, "workflow target is empty")
	}

	if _, err := s.ctrl.GetWorkflow(ctx, request.WorkflowID); err != nil {
		return nil, status.Error(common.FsApiError(err), "fetch workflow failed")
	}

	if request.Attr == nil {
		request.Attr = &TriggerWorkflowRequest_WorkflowJobAttr{
			Reason:  "fsapi trigger",
			Timeout: 60 * 10, // default 10min
		}
	}

	job, err := s.ctrl.TriggerWorkflow(ctx, request.WorkflowID,
		types.WorkflowTarget{EntryID: request.Target.EntryID, ParentEntryID: request.Target.ParentEntryID},
		workflow.JobAttr{Reason: request.Attr.Reason, Timeout: time.Second * time.Duration(request.Attr.Timeout)},
	)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "trigger workflow failed")
	}
	return &TriggerWorkflowResponse{JobID: job.Id}, nil
}

func (s *services) queryEntryProperties(ctx context.Context, entryID, parentID int64) ([]*Property, error) {
	var (
		properties = make(map[string]types.PropertyItem)
		err        error
	)
	if parentID != dentry.RootEntryID {
		properties, err = s.ctrl.ListEntryProperties(ctx, parentID)
		if err != nil {
			return nil, err
		}
		s.logger.Infow("list entry properties", "entry", entryID, "parentID", parentID, "got", len(properties))
	}
	entryProperties, err := s.ctrl.ListEntryProperties(ctx, entryID)
	if err != nil {
		return nil, err
	}
	for k, p := range entryProperties {
		properties[k] = p
	}
	result := make([]*Property, 0, len(properties))
	for key, p := range properties {
		result = append(result, &Property{
			Key:     key,
			Value:   p.Value,
			Encoded: p.Encoded,
		})
	}

	return result, nil
}

var (
	callerAuthGetter = common.CallerAuth
)
