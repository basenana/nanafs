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
	"google.golang.org/protobuf/types/known/timestamppb"
	"io"
	"path"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/basenana/nanafs/cmd/apps/apis/fsapi/common"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/notify"
	"github.com/basenana/nanafs/pkg/token"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/basenana/nanafs/workflow"
)

type ServicesV1 interface {
	EntriesServer
	PropertiesServer
	NotifyServer
	WorkflowServer
}

func InitServicesV1(server *grpc.Server, depends *common.Depends) (ServicesV1, error) {
	s := &servicesV1{
		meta:      depends.Meta,
		core:      depends.Core,
		workflow:  depends.Workflow,
		notify:    depends.Notify,
		cfgLoader: depends.ConfigLoader,
		logger:    logger.NewLogger("fsapi"),
	}

	RegisterEntriesServer(server, s)
	RegisterPropertiesServer(server, s)
	RegisterNotifyServer(server, s)
	RegisterWorkflowServer(server, s)
	return s, nil
}

type servicesV1 struct {
	meta      metastore.Meta
	core      core.Core
	workflow  workflow.Workflow
	notify    *notify.Notify
	cfgLoader config.Config
	logger    *zap.SugaredLogger
}

var _ ServicesV1 = &servicesV1{}

func (s *servicesV1) GroupTree(ctx context.Context, request *GetGroupTreeRequest) (*GetGroupTreeResponse, error) {
	caller, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	root, err := s.getGroupTree(ctx, caller.Namespace)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query root entry failed")
	}
	resp := &GetGroupTreeResponse{Root: root}
	return resp, nil
}

func (s *servicesV1) GetEntryDetail(ctx context.Context, request *GetEntryDetailRequest) (*GetEntryDetailResponse, error) {
	caller, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}

	s.logger.Infow("request", "uri", request.Uri)
	parentID, entryID, err := s.getEntryByPath(ctx, caller.Namespace, request.Uri)
	if err != nil {
		return nil, err
	}
	child, err := s.meta.GetChild(ctx, caller.Namespace, parentID, entryID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "get child failed")
	}

	en, err := s.core.GetEntry(ctx, caller.Namespace, child.ChildID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}
	err = core.HasAllPermissions(en.Access, caller.UID, caller.GID, types.PermOwnerRead, types.PermGroupRead, types.PermOthersRead)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "has no permission")
	}

	parentURI, name := path.Split(request.Uri)
	detail, properties, err := s.getEntryDetails(ctx, caller.Namespace, parentURI, name, child.ChildID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry details failed")
	}
	return &GetEntryDetailResponse{Entry: detail, Properties: properties}, nil
}

func (s *servicesV1) FilterEntry(ctx context.Context, request *FilterEntryRequest) (*ListEntriesResponse, error) {
	caller, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}

	it, err := s.meta.FilterEntries(ctx, caller.Namespace, types.Filter{CELPattern: request.CelPattern})
	if err != nil {
		s.logger.Errorw("list static children failed", "err", err)
		return nil, err
	}

	var (
		resp = &ListEntriesResponse{}
		doc  *types.DocumentProperties
	)

	for it.HasNext() {
		en, err := it.Next()
		if err != nil {
			return nil, err
		}

		if !en.IsGroup {
			doc = &types.DocumentProperties{}
			if err = s.meta.GetEntryProperties(ctx, caller.Namespace, types.PropertyTypeDocument, en.ID, doc); err != nil {
				s.logger.Errorw("get entry document properties failed", "entry", en.ID, "err", err)
				doc = nil
			}
		}

		uri, err := core.ProbableEntryPath(ctx, s.core, en)
		if err != nil {
			s.logger.Errorw("guess entry uri error, hide this", "entry", en.ID, "err", err)
			continue
		}
		resp.Entries = append(resp.Entries, toEntryInfo(uri, path.Base(uri), en, doc))
	}

	return resp, nil
}

func (s *servicesV1) CreateEntry(ctx context.Context, request *CreateEntryRequest) (*CreateEntryResponse, error) {
	caller, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}

	parentURI, name := path.Split(request.Uri)

	_, parentID, err := s.getEntryByPath(ctx, caller.Namespace, parentURI)
	if err != nil {
		return nil, err
	}

	if request.Kind == "" {
		return nil, status.Error(codes.InvalidArgument, "entry has unknown kind")
	}

	parent, err := s.core.GetEntry(ctx, caller.Namespace, parentID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry parent failed")
	}

	err = core.HasAllPermissions(parent.Access, caller.UID, caller.GID, types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "has no permission")
	}

	attr := types.EntryAttr{
		Name:   name,
		Kind:   pdKind2EntryKind(request.Kind),
		Access: &parent.Access,
	}

	switch cfg := request.GroupConfig.(type) {
	case *CreateEntryRequest_Rss:
		s.logger.Infow("setup rss feed to dir", "feed", cfg.Rss.Feed, "siteName", cfg.Rss.SiteName)
		setupRssConfig(cfg.Rss, &attr)
	case *CreateEntryRequest_Filter:
		s.logger.Infow("setup group filter", "cel", cfg.Filter.CelPattern)
		setupGroupFilterConfig(cfg.Filter, &attr)
	}

	en, err := s.core.CreateEntry(ctx, caller.Namespace, parentID, attr)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "create entry failed")
	}

	if attr.Properties != nil {
		if err = s.meta.UpdateEntryProperties(ctx, caller.Namespace, types.PropertyTypeProperty, en.ID, attr.Properties); err != nil {
			return nil, err
		}
	}

	return &CreateEntryResponse{Entry: toEntryInfo(parentURI, name, en, nil)}, nil
}

func (s *servicesV1) UpdateEntry(ctx context.Context, request *UpdateEntryRequest) (*UpdateEntryResponse, error) {
	caller, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}

	parentID, entryID, err := s.getEntryByPath(ctx, caller.Namespace, request.Uri)
	if err != nil {
		return nil, err
	}

	en, err := s.core.GetEntry(ctx, caller.Namespace, entryID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}

	err = core.HasAllPermissions(en.Access, caller.UID, caller.GID, types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "has no permission")
	}

	_, err = s.core.GetEntry(ctx, caller.Namespace, parentID)
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

	parentURI, name := path.Split(request.Uri)
	detail, _, err := s.getEntryDetails(ctx, caller.Namespace, parentURI, name, en.ID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry detail failed")
	}
	return &UpdateEntryResponse{Entry: detail}, nil
}

func (s *servicesV1) DeleteEntry(ctx context.Context, request *DeleteEntryRequest) (*DeleteEntryResponse, error) {
	caller, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}

	parentID, entryID, err := s.getEntryByPath(ctx, caller.Namespace, request.Uri)
	if err != nil {
		return nil, err
	}
	en, err := s.deleteEntry(ctx, caller.Namespace, caller.UID, parentID, entryID, path.Base(request.Uri))
	if err != nil {
		return nil, err
	}

	parentURI, name := path.Split(request.Uri)
	return &DeleteEntryResponse{Entry: toEntryInfo(parentURI, name, en, nil)}, nil
}

func (s *servicesV1) deleteEntry(ctx context.Context, namespace string, uid, parentID, entryId int64, name string) (en *types.Entry, err error) {
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

	parent, err := s.core.GetEntry(ctx, namespace, parentID)
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

	s.logger.Debugw("destroy entry", "parent", parentID, "entry", entryId)
	return en, s.core.RemoveEntry(ctx, namespace, parentID, entryId, name, types.DeleteEntry{})
}

func (s *servicesV1) DeleteEntries(ctx context.Context, request *DeleteEntriesRequest) (*DeleteEntriesResponse, error) {
	caller, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}

	deleted := make([]string, 0, len(request.UriList))
	for _, entryURI := range request.UriList {
		parentID, entryID, err := s.getEntryByPath(ctx, caller.Namespace, entryURI)
		if err != nil {
			return &DeleteEntriesResponse{Deleted: deleted}, err
		}
		_, err = s.deleteEntry(ctx, caller.Namespace, caller.UID, parentID, entryID, path.Base(entryURI))
		if err != nil {
			return &DeleteEntriesResponse{Deleted: deleted}, err
		}
		deleted = append(deleted, entryURI)
	}
	return &DeleteEntriesResponse{Deleted: deleted}, nil
}

func (s *servicesV1) ListGroupChildren(ctx context.Context, request *ListGroupChildrenRequest) (*ListEntriesResponse, error) {
	caller, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}

	_, parentID, err := s.getEntryByPath(ctx, caller.Namespace, request.ParentURI)
	if err != nil {
		return nil, err
	}

	if request.Pagination != nil {
		ctx = types.WithPagination(ctx, types.NewPagination(request.Pagination.Page, request.Pagination.PageSize))
	}
	//order := EntryOrder{
	//	Order: EnOrder(request.Order),
	//	Desc:  request.OrderDesc,
	//}
	children, err := s.listEntryChildren(ctx, caller.Namespace, parentID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "list children failed")
	}

	var (
		resp = &ListEntriesResponse{}
		doc  *types.DocumentProperties
	)
	for _, en := range children {
		if !en.IsGroup {
			doc = &types.DocumentProperties{}
			if err = s.meta.GetEntryProperties(ctx, caller.Namespace, types.PropertyTypeDocument, en.ID, doc); err != nil {
				s.logger.Errorw("get entry document properties failed", "entry", en.ID, "err", err)
				doc = nil
			}
		}
		resp.Entries = append(resp.Entries, toEntryInfo(request.ParentURI, en.Name, en, doc))
	}
	return resp, nil
}

func (s *servicesV1) ChangeParent(ctx context.Context, request *ChangeParentRequest) (*ChangeParentResponse, error) {
	caller, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}

	oldName := path.Base(request.EntryURI)
	oldParentID, entryID, err := s.getEntryByPath(ctx, caller.Namespace, request.EntryURI)
	if err != nil {
		return nil, err
	}

	newParentURI, newName := path.Split(request.NewEntryURI)
	_, newParentID, err := s.getEntryByPath(ctx, caller.Namespace, newParentURI)
	if err != nil {
		return nil, err
	}

	en, err := s.core.GetEntry(ctx, caller.Namespace, entryID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}
	err = core.HasAllPermissions(en.Access, caller.UID, caller.GID, types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "has no permission")
	}

	newParent, err := s.core.GetEntry(ctx, caller.Namespace, newParentID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}
	err = core.HasAllPermissions(newParent.Access, caller.UID, caller.GID, types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "has no permission")
	}

	var (
		existObjId *int64
	)
	existObj, err := s.core.FindEntry(ctx, caller.Namespace, newParent.ID, newName)
	if err != nil {
		if !errors.Is(err, types.ErrNotFound) {
			return nil, status.Error(common.FsApiError(err), "check existing entry failed")
		}
	}
	if existObj != nil {
		existObjId = &existObj.ChildID
	}

	if request.Option == nil {
		request.Option = &ChangeParentRequest_Option{}
	}
	err = s.core.ChangeEntryParent(ctx, caller.Namespace, entryID, existObjId, oldParentID, newParentID, oldName, newName, types.ChangeParentAttr{
		Uid:      caller.UID,
		Gid:      caller.GID,
		Replace:  request.Option.Replace,
		Exchange: request.Option.Exchange,
	})
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "change entry parent failed")
	}

	en, err = s.core.GetEntry(ctx, caller.Namespace, entryID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}
	return &ChangeParentResponse{Entry: toEntryInfo(newParentURI, newName, en, nil)}, nil
}

func (s *servicesV1) WriteFile(reader Entries_WriteFileServer) error {
	var (
		ctx       = reader.Context()
		recvTotal int64
		accessEn  int64
		file      core.RawFile
	)

	caller, err := s.caller(ctx)
	if err != nil {
		return err
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
			s.logger.Errorw("read entry failed", "error", err)
			return err
		}

		if len(writeRequest.Data) == 0 || writeRequest.Len == 0 {
			break
		}

		if writeRequest.Entry != accessEn {
			s.logger.Debugw("handle write data to file", "entry", writeRequest.Entry)
			en, err := s.core.GetEntry(ctx, caller.Namespace, writeRequest.Entry)
			if err != nil {
				return status.Error(common.FsApiError(err), "query entry failed")
			}

			err = core.HasAllPermissions(en.Access, caller.UID, caller.GID, types.PermOwnerWrite, types.PermGroupWrite, types.PermOthersWrite)
			if err != nil {
				return status.Error(common.FsApiError(err), "has no permission")
			}
			accessEn = writeRequest.Entry
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

	caller, err := s.caller(ctx)
	if err != nil {
		return err
	}
	en, err := s.core.GetEntry(ctx, caller.Namespace, request.Entry)
	if err != nil {
		return status.Error(common.FsApiError(err), "query entry failed")
	}

	err = core.HasAllPermissions(en.Access, caller.UID, caller.GID, types.PermOwnerRead, types.PermGroupRead, types.PermOthersRead)
	if err != nil {
		return status.Error(common.FsApiError(err), "has no permission")
	}

	file, err := s.core.Open(ctx, caller.Namespace, request.Entry, types.OpenAttr{Read: true})
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

func (s *servicesV1) UpdateDocumentProperty(ctx context.Context, request *UpdateDocumentPropertyRequest) (*GetDocumentPropertiesResponse, error) {
	caller, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	en, err := s.core.GetEntry(ctx, caller.Namespace, request.Entry)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}

	if en.IsGroup {
		return nil, status.Error(common.FsApiError(types.ErrIsGroup), "group has no document properties")
	}

	properties := &types.DocumentProperties{}
	err = s.meta.GetEntryProperties(ctx, caller.Namespace, types.PropertyTypeDocument, en.ID, &properties)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "fetch entry properties failed")
	}

	switch mark := request.Mark.(type) {
	case *UpdateDocumentPropertyRequest_Unread:
		properties.Unread = mark.Unread
	case *UpdateDocumentPropertyRequest_Marked:
		properties.Marked = mark.Marked
	}

	err = s.meta.UpdateEntryProperties(ctx, caller.Namespace, types.PropertyTypeDocument, en.ID, &properties)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "update entry properties failed")
	}

	return &GetDocumentPropertiesResponse{
		Properties: &DocumentProperty{
			Title:       properties.Title,
			Author:      properties.Author,
			Year:        properties.Year,
			Source:      properties.Source,
			Abstract:    properties.Abstract,
			Keywords:    properties.Keywords,
			Notes:       properties.Notes,
			Unread:      properties.Unread,
			Marked:      properties.Marked,
			PublishAt:   timestamppb.New(time.Unix(properties.PublishAt, 0)),
			Url:         properties.URL,
			HeaderImage: properties.HeaderImage,
		},
	}, nil
}

func (s *servicesV1) UpdateProperty(ctx context.Context, request *UpdatePropertyRequest) (*GetPropertiesResponse, error) {
	caller, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	en, err := s.core.GetEntry(ctx, caller.Namespace, request.Entry)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}

	properties := &types.Properties{}
	err = s.meta.GetEntryProperties(ctx, caller.Namespace, types.PropertyTypeProperty, en.ID, &properties)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "fetch entry properties failed")
	}

	update := request.Properties
	if update != nil {
		properties.Tags = update.Tags
		properties.Properties = update.Properties
	}

	err = s.meta.UpdateEntryProperties(ctx, caller.Namespace, types.PropertyTypeProperty, en.ID, &properties)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "update entry properties failed")
	}

	resp := &GetPropertiesResponse{Properties: &Property{
		Tags:       properties.Tags,
		Properties: properties.Properties,
	}}
	return resp, nil
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
	caller, err := s.caller(ctx)
	if err != nil {
		return nil, err
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
	caller, err := s.caller(ctx)
	if err != nil {
		return nil, err
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
	caller, err := s.caller(ctx)
	if err != nil {
		return nil, err
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
		types.WorkflowTarget{Entries: []string{request.Target.Uri}},
		workflow.JobAttr{Reason: request.Attr.Reason, Timeout: time.Second * time.Duration(request.Attr.Timeout)},
	)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "trigger workflow failed")
	}
	return &TriggerWorkflowResponse{JobID: job.Id}, nil
}

func (s *servicesV1) caller(ctx context.Context) (*token.AuthInfo, error) {
	ai := common.Auth(ctx)
	if ai == nil {
		return nil, status.Errorf(codes.Unauthenticated, "unauthenticated")
	}
	return ai, nil
}

func (s *servicesV1) getEntryByPath(ctx context.Context, namespace, path string) (int64, int64, error) {
	p, e, err := s.core.GetEntryByPath(ctx, namespace, path)
	if err != nil {
		return 0, 0, status.Error(common.FsApiError(err), fmt.Sprintf("get entry failed: %s", err))
	}
	var pid, eid int64
	if p != nil {
		pid = p.ID
	}
	if e != nil {
		eid = e.ID
	}

	return pid, eid, nil
}
