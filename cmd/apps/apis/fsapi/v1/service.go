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
}

func InitServices(server *grpc.Server, ctrl controller.Controller, pathEntryMgr *pathmgr.PathManager) (Services, error) {
	s := &services{
		ctrl:         ctrl,
		pathEntryMgr: pathEntryMgr,
		logger:       logger.NewLogger("fsapi"),
	}

	RegisterDocumentServer(server, s)
	RegisterEntriesServer(server, s)
	RegisterInboxServer(server, s)
	RegisterPropertiesServer(server, s)
	return s, nil
}

type services struct {
	ctrl         controller.Controller
	pathEntryMgr *pathmgr.PathManager
	logger       *zap.SugaredLogger
}

var _ Services = &services{}

func (s *services) ListDocuments(ctx context.Context, request *ListDocumentsRequest) (*ListDocumentsResponse, error) {
	filter := types.DocFilter{ParentID: request.ParentID}
	if !request.ListAll && filter.ParentID == 0 {
		return nil, status.Error(codes.InvalidArgument, "parent id is empty")
	}
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
	en, err := s.ctrl.GetEntry(ctx, request.EntryID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}

	p, err := s.ctrl.GetEntry(ctx, en.ParentID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry parent failed")
	}

	return &GetEntryDetailResponse{Entry: entryDetail(en, p)}, nil
}

func (s *services) CreateEntry(ctx context.Context, request *CreateEntryRequest) (*CreateEntryResponse, error) {
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

	en, err := s.ctrl.CreateEntry(ctx, request.ParentID, types.EntryAttr{
		Name:   request.Name,
		Kind:   pdKind2EntryKind(request.Kind),
		Access: &parent.Access,
		Dev:    parent.Dev,
	})
	return &CreateEntryResponse{Entry: entryInfo(en)}, nil
}

func (s *services) UpdateEntry(ctx context.Context, request *UpdateEntryRequest) (*UpdateEntryResponse, error) {
	if request.Entry.Id == 0 {
		return nil, status.Error(codes.InvalidArgument, "entry id is 0")
	}
	en, err := s.ctrl.GetEntry(ctx, request.Entry.Id)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}

	parent, err := s.ctrl.GetEntry(ctx, en.ParentID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry parent failed")
	}

	// TODO: do update

	err = s.ctrl.UpdateEntry(ctx, en)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "update entry failed")
	}
	return &UpdateEntryResponse{Entry: entryDetail(en, parent)}, nil
}

func (s *services) DeleteEntry(ctx context.Context, request *DeleteEntryRequest) (*DeleteEntryResponse, error) {
	en, err := s.ctrl.GetEntry(ctx, request.EntryID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}

	parent, err := s.ctrl.GetEntry(ctx, en.ParentID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry parent failed")
	}

	err = s.ctrl.DestroyEntry(ctx, parent.ParentID, en.ID, types.DestroyObjectAttr{Uid: 0, Gid: 0})
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "delete entry failed")
	}
	return &DeleteEntryResponse{Entry: entryDetail(en, parent)}, nil
}

func (s *services) GetGroupTree(ctx context.Context, request *GetGroupTreeRequest) (*GetGroupTreeResponse, error) {
	//TODO implement me
	panic("implement me")
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

func (s *services) WriteFile(reader Entries_WriteFileServer) error {
	var (
		ctx       = reader.Context()
		recvTotal int64
		file      dentry.File
	)

	defer func() {
		if file != nil {
			_ = file.Close(context.TODO())
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
	return reader.SendAndClose(&WriteFileResponse{Len: recvTotal})
}

func (s *services) ReadFile(request *ReadFileRequest, writer Entries_ReadFileServer) error {
	var (
		ctx           = writer.Context()
		readOnce, off int64
	)

	file, err := s.ctrl.OpenFile(ctx, request.EntryID, types.OpenAttr{Read: true})
	if err != nil {
		return status.Error(common.FsApiError(err), "open file failed")
	}

	chunk := make([]byte, 1024)
	for {
		readOnce, err = file.ReadAt(ctx, chunk, off)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		err = writer.Send(&ReadFileResponse{
			Off:  off,
			Len:  readOnce,
			Data: chunk[:readOnce],
		})
		if err != nil {
			return err
		}
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
	en, err := s.ctrl.QuickInbox(ctx, option)
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
	return &AddPropertyResponse{
		Entry:      entryInfo(en),
		Properties: nil, // TODO
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
	return &UpdatePropertyResponse{
		Entry:      entryInfo(en),
		Properties: nil, // TODO
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
	return &DeletePropertyResponse{
		Entry:      entryInfo(en),
		Properties: nil, // TODO
	}, nil
}
