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
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/document"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/notify"
	"github.com/basenana/nanafs/pkg/token"
	"io"
	"path"
	"strings"
	"time"

	"github.com/basenana/nanafs/workflow"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/basenana/nanafs/cmd/apps/apis/fsapi/common"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
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
		doc:       depends.Document,
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
	doc       document.Manager
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

	if child.ParentID != core.RootEntryID {
		_, err = s.core.GetEntry(ctx, caller.Namespace, child.ParentID)
		if err != nil {
			return nil, status.Error(common.FsApiError(err), "query entry parent failed")
		}
	}

	detail, properties, err := s.getEntryDetails(ctx, caller.Namespace, request.Uri, child.ParentID, child.ChildID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry details failed")
	}
	return &GetEntryDetailResponse{Entry: detail, Properties: properties}, nil
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
	}

	en, err := s.core.CreateEntry(ctx, caller.Namespace, parentID, attr)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "create entry failed")
	}
	return &CreateEntryResponse{Entry: coreEntryInfo(parent.ID, name, en)}, nil
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

	detail, _, err := s.getEntryDetails(ctx, caller.Namespace, request.Uri, parentID, en.ID)
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
	return &DeleteEntryResponse{Entry: toEntryInfo(parentURI, name, en)}, nil
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

func (s *servicesV1) ListGroupChildren(ctx context.Context, request *ListGroupChildrenRequest) (*ListGroupChildrenResponse, error) {
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
	//order := EntryOrder{
	//	Order: EnOrder(request.Order),
	//	Desc:  request.OrderDesc,
	//}
	children, err := s.listEntryChildren(ctx, caller.Namespace, parentID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "list children failed")
	}

	resp := &ListGroupChildrenResponse{}
	for _, en := range children {
		resp.Entries = append(resp.Entries, toEntryInfo(request.ParentURI, en.Name, en))
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
	return &ChangeParentResponse{Entry: toEntryInfo(newParentURI, newName, en)}, nil
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

func (s *servicesV1) AddProperty(ctx context.Context, request *AddPropertyRequest) (*AddPropertyResponse, error) {
	caller, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	en, err := s.core.GetEntry(ctx, caller.Namespace, request.Entry)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}
	err = s.meta.AddEntryProperty(ctx, caller.Namespace, en.ID, request.Key, types.PropertyItem{Value: request.Value, Encoded: false})
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "add entry extend field failed")
	}

	en, err = s.core.GetEntry(ctx, caller.Namespace, request.Entry)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}

	properties, err := s.queryEntryProperties(ctx, caller.Namespace, en.ID, 0)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry properties failed")
	}
	return &AddPropertyResponse{
		Properties: properties,
	}, nil
}

func (s *servicesV1) UpdateProperty(ctx context.Context, request *UpdatePropertyRequest) (*UpdatePropertyResponse, error) {
	caller, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	en, err := s.core.GetEntry(ctx, caller.Namespace, request.Entry)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}

	err = s.meta.AddEntryProperty(ctx, caller.Namespace, en.ID, request.Key, types.PropertyItem{Value: request.Value, Encoded: false})
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "set entry extend field failed")
	}

	en, err = s.core.GetEntry(ctx, caller.Namespace, request.Entry)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}
	properties, err := s.queryEntryProperties(ctx, caller.Namespace, en.ID, en.ParentID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry properties failed")
	}

	return &UpdatePropertyResponse{
		Properties: properties,
	}, nil
}

func (s *servicesV1) DeleteProperty(ctx context.Context, request *DeletePropertyRequest) (*DeletePropertyResponse, error) {
	caller, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}

	en, err := s.core.GetEntry(ctx, caller.Namespace, request.Entry)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}
	err = s.meta.RemoveEntryProperty(ctx, caller.Namespace, en.ID, request.Key)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "set entry extend field failed")
	}

	en, err = s.core.GetEntry(ctx, caller.Namespace, request.Entry)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry failed")
	}
	properties, err := s.queryEntryProperties(ctx, caller.Namespace, en.ID, en.ParentID)
	if err != nil {
		return nil, status.Error(common.FsApiError(err), "query entry properties failed")
	}

	return &DeletePropertyResponse{
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
	if ai != nil {
		return nil, status.Errorf(codes.Unauthenticated, "unauthenticated")
	}
	return ai, nil
}

func (s *servicesV1) getEntryByPath(ctx context.Context, namespace, path string) (int64, int64, error) {
	var (
		root        *types.Entry
		next        *types.Child
		crt, parent int64
		err         error
	)
	root, err = s.core.NamespaceRoot(ctx, namespace)
	if err != nil {
		return 0, 0, status.Error(common.FsApiError(err), fmt.Sprintf("get root failed: %s", err))
	}

	parent = root.ID
	if path == "/" {
		return parent, parent, nil
	}

	entries := strings.Split(path, "/")
	for _, entryName := range entries {
		if entryName == "" {
			continue
		}

		if crt != 0 {
			parent = crt
		}

		next, err = s.core.FindEntry(ctx, namespace, parent, entryName)
		if err != nil {
			return 0, 0, status.Error(common.FsApiError(err), fmt.Sprintf("get entry failed: %s", err))
		}
		crt = next.ChildID
	}

	return parent, crt, nil
}
