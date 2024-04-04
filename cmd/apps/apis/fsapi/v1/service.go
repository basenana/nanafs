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
	"github.com/basenana/nanafs/cmd/apps/apis/fsapi/common"
	"github.com/basenana/nanafs/cmd/apps/apis/pathmgr"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/inbox"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"io"
)

type Services interface {
	DocumentServer
	EntriesServer
	InboxServer
	PropertiesServer
	NotifyServer
}

func InitServices(server *grpc.Server, ctrl controller.Controller, pathEntryMgr *pathmgr.PathManager) (Services, error) {
	s := &services{
		ctrl:         ctrl,
		pathEntryMgr: pathEntryMgr,
		callerAuthFn: callerAuthGetter,
		logger:       logger.NewLogger("fsapi"),
	}

	RegisterDocumentServer(server, s)
	RegisterEntriesServer(server, s)
	RegisterInboxServer(server, s)
	RegisterPropertiesServer(server, s)
	RegisterNotifyServer(server, s)
	return s, nil
}

type services struct {
	ctrl         controller.Controller
	pathEntryMgr *pathmgr.PathManager
	callerAuthFn common.CallerAuthGetter
	logger       *zap.SugaredLogger
}

var _ Services = &services{}

func (s *services) ListDocuments(ctx context.Context, request *ListDocumentsRequest) (*ListDocumentsResponse, error) {
	filter := types.DocFilter{ParentID: request.ParentID}
	if request.Marked {
		filter.Marked = &request.Marked
	}
	if request.Unread {
		filter.Unread = &request.Unread
	}
	docList, err := s.ctrl.ListDocuments(ctx, filter)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "filter document failed")
	}
	resp := &ListDocumentsResponse{Documents: make([]*DocumentInfo, 0, len(docList))}
	for _, doc := range docList {
		resp.Documents = append(resp.Documents, &DocumentInfo{
			Id:            doc.ID,
			Name:          doc.Name,
			EntryID:       doc.OID,
			ParentEntryID: doc.ParentEntryID,
			Source:        doc.Source,
			Marked:        doc.Marked,
			Unread:        doc.Unread,
			CreatedAt:     timestamppb.New(doc.CreatedAt),
			ChangedAt:     timestamppb.New(doc.ChangedAt),
		})
	}
	return resp, nil
}

func (s *services) GetDocumentDetail(ctx context.Context, request *GetDocumentDetailRequest) (*GetDocumentDetailResponse, error) {
	if request.EntryID == 0 {
		return nil, status.Error(codes.InvalidArgument, "entry id is empty")
	}
	doc, err := s.ctrl.GetDocumentsByEntryId(ctx, request.EntryID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query document with entry id failed")
	}
	return &GetDocumentDetailResponse{
		Document: &DocumentDescribe{
			Id:            doc.ID,
			Name:          doc.Name,
			EntryID:       doc.OID,
			ParentEntryID: doc.ParentEntryID,
			Source:        doc.Source,
			Marked:        doc.Marked,
			Unread:        doc.Unread,
			HtmlContent:   doc.Content,
			KeyWords:      doc.KeyWords,
			Summary:       doc.Summary,
			CreatedAt:     timestamppb.New(doc.CreatedAt),
			ChangedAt:     timestamppb.New(doc.ChangedAt),
		},
	}, nil
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
	err = dentry.IsHasPermissions(en.Access, caller.UID, 0, []types.Permission{types.PermOwnerRead, types.PermGroupRead, types.PermOthersRead})
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "has no permission")
	}

	p, err := s.ctrl.GetEntry(ctx, en.ParentID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry parent failed")
	}

	properties, err := s.queryEntryProperties(ctx, request.EntryID)
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

	err = dentry.IsHasPermissions(parent.Access, caller.UID, 0, []types.Permission{types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite})
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "has no permission")
	}

	en, err := s.ctrl.CreateEntry(ctx, request.ParentID, types.EntryAttr{
		Name:   request.Name,
		Kind:   pdKind2EntryKind(request.Kind),
		Access: &parent.Access,
		Dev:    parent.Dev,
	})
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

	err = dentry.IsHasPermissions(en.Access, caller.UID, 0, []types.Permission{types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite})
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

	en, err := s.ctrl.GetEntry(ctx, request.EntryID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}
	err = dentry.IsHasPermissions(en.Access, caller.UID, 0, []types.Permission{types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite})
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "has no permission")
	}

	parent, err := s.ctrl.GetEntry(ctx, en.ParentID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry parent failed")
	}
	err = dentry.IsHasPermissions(parent.Access, caller.UID, 0, []types.Permission{types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite})
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "has no permission")
	}

	err = s.ctrl.DestroyEntry(ctx, parent.ID, en.ID, types.DestroyObjectAttr{Uid: 0, Gid: 0})
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "delete entry failed")
	}
	return &DeleteEntryResponse{Entry: entryDetail(en, parent)}, nil
}

func (s *services) ListGroupChildren(ctx context.Context, request *ListGroupChildrenRequest) (*ListGroupChildrenResponse, error) {
	children, err := s.ctrl.ListEntryChildren(ctx, request.ParentID)
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
	err = dentry.IsHasPermissions(en.Access, caller.UID, 0, []types.Permission{types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite})
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "has no permission")
	}

	newParent, err := s.ctrl.GetEntry(ctx, request.NewParentID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}
	err = dentry.IsHasPermissions(newParent.Access, caller.UID, 0, []types.Permission{types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite})
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

		if writeRequest.EntryID != accessEn {
			en, err := s.ctrl.GetEntry(ctx, writeRequest.EntryID)
			if err != nil {
				return status.Error(common.FsApiError(err), "query entry failed")
			}

			err = dentry.IsHasPermissions(en.Access, caller.UID, 0, []types.Permission{types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite})
			if err != nil {
				return status.Error(common.FsApiError(err), "has no permission")
			}
			accessEn = writeRequest.EntryID
		}

		if file == nil {
			file, err = s.ctrl.OpenFile(ctx, writeRequest.EntryID, types.OpenAttr{Write: true})
			if err != nil {
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

	err = dentry.IsHasPermissions(en.Access, caller.UID, 0, []types.Permission{types.PermOwnerRead, types.PermGroupRead, types.PermOthersRead})
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
		ClutterFree: request.ClutterFree,
	}
	switch request.FileType {
	case QuickInboxRequest_BookmarkFile:
		option.FileType = "url"
	case QuickInboxRequest_WebArchiveFile:
		option.FileType = "webarchive"
	case QuickInboxRequest_HtmlFile:
		option.FileType = "html"
	}
	en, err := s.ctrl.QuickInbox(ctx, request.Filename, option)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "quick inbox failed")
	}
	return &QuickInboxResponse{EntryID: en.ID}, nil
}

func (s *services) AddProperty(ctx context.Context, request *AddPropertyRequest) (*AddPropertyResponse, error) {
	en, err := s.ctrl.GetEntry(ctx, request.EntryID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}
	err = s.ctrl.SetEntryExtendField(ctx, en.ID, request.Key, request.Value)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "add entry extend field failed")
	}
	properties, err := s.queryEntryProperties(ctx, request.EntryID)
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

	_, err = s.ctrl.GetEntryExtendField(ctx, en.ID, request.Key)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "fetch entry exist extend field failed")
	}

	err = s.ctrl.SetEntryExtendField(ctx, en.ID, request.Key, request.Value)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "set entry extend field failed")
	}
	properties, err := s.queryEntryProperties(ctx, request.EntryID)
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
	err = s.ctrl.RemoveEntryExtendField(ctx, en.ID, request.Key)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "set entry extend field failed")
	}
	properties, err := s.queryEntryProperties(ctx, request.EntryID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry properties failed")
	}
	return &DeletePropertyResponse{
		Entry:      entryInfo(en),
		Properties: properties,
	}, nil
}

func (s *services) GetLatestSequence(ctx context.Context, request *GetLatestSequenceRequest) (*GetLatestSequenceResponse, error) {
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
	err := s.ctrl.CommitSyncedEvent(ctx, request.DeviceID, request.Sequence)
	if err != nil {
		s.logger.Errorw("device commit sequence failed", "device", request.DeviceID, "err", err)
		return nil, status.Error(common.FsApiError(err), "device commit sequence failed")
	}
	return &CommitSyncedEventResponse{}, nil
}

func (s *services) queryEntryProperties(ctx context.Context, entryID int64) ([]*Property, error) {
	properties, err := s.ctrl.ListEntryExtendField(ctx, entryID)
	if err != nil {
		return nil, err
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
