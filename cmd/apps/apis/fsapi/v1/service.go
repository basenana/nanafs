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
	"github.com/basenana/nanafs/cmd/apps/apis/pathmgr"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"google.golang.org/grpc"
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
	//TODO implement me
	panic("implement me")
}

func (s *services) GetDocumentDetail(ctx context.Context, request *GetDocumentDetailRequest) (*GetDocumentDetailResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s *services) GetEntryDetail(ctx context.Context, request *GetEntryDetailRequest) (*GetEntryDetailResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s *services) CreateEntry(ctx context.Context, request *CreateEntryRequest) (*CreateEntryResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s *services) UpdateEntry(ctx context.Context, request *UpdateEntryRequest) (*UpdateEntryResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s *services) DeleteEntry(ctx context.Context, request *DeleteEntryRequest) (*DeleteEntryResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s *services) GetGroupTree(ctx context.Context, request *GetGroupTreeRequest) (*GetGroupTreeResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s *services) ListGroupChildren(ctx context.Context, request *ListGroupChildrenRequest) (*ListGroupChildrenResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s *services) WriteFile(ctx context.Context, request *WriteFileRequest) (*WriteFileResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s *services) ReadFile(ctx context.Context, request *ReadFileRequest) (*ReadFileResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s *services) QuickInbox(ctx context.Context, request *QuickInboxRequest) (*QuickInboxResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s *services) AddProperty(ctx context.Context, request *AddPropertyRequest) (*AddPropertyResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s *services) UpdateProperty(ctx context.Context, request *UpdatePropertyRequest) (*UpdatePropertyResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s *services) DeleteProperty(ctx context.Context, request *DeletePropertyRequest) (*DeletePropertyResponse, error) {
	//TODO implement me
	panic("implement me")
}
