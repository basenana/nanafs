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
	"github.com/basenana/nanafs/fs"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/basenana/nanafs/pkg/types"
)

var (
	kind2PdMapping = map[types.Kind]struct{}{
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
)

func pdKind2EntryKind(k string) types.Kind {
	_, ok := kind2PdMapping[types.Kind(k)]
	if !ok {
		return types.RawKind
	}
	return types.Kind(k)
}

func entryInfo(en *types.Entry) *EntryInfo {
	return &EntryInfo{
		Id:         en.ID,
		Name:       en.Name,
		Kind:       string(en.Kind),
		ParentID:   en.ParentID,
		IsGroup:    en.IsGroup,
		Size:       en.Size,
		CreatedAt:  timestamppb.New(en.CreatedAt),
		ChangedAt:  timestamppb.New(en.ChangedAt),
		ModifiedAt: timestamppb.New(en.ModifiedAt),
		AccessAt:   timestamppb.New(en.AccessAt),
	}
}

func toEntryInfo(en *fs.Entry) *EntryInfo {
	return &EntryInfo{
		Id:         en.ID,
		Name:       en.Name,
		Kind:       string(en.Kind),
		ParentID:   en.ParentID,
		IsGroup:    en.IsGroup,
		Size:       en.Size,
		CreatedAt:  timestamppb.New(en.CreatedAt),
		ChangedAt:  timestamppb.New(en.ChangedAt),
		ModifiedAt: timestamppb.New(en.ModifiedAt),
		AccessAt:   timestamppb.New(en.AccessAt),
	}
}

func toEntryDetail(en, parent *fs.Entry) (*EntryDetail, []*Property) {
	access := &EntryDetail_Access{Uid: en.Access.UID, Gid: en.Access.GID}
	for _, perm := range en.Access.Permissions {
		access.Permissions = append(access.Permissions, string(perm))
	}

	properties := make(map[string]types.PropertyItem)
	ed := &EntryDetail{
		Id:         en.ID,
		Name:       en.Name,
		Aliases:    en.Aliases,
		Kind:       string(en.Kind),
		IsGroup:    en.IsGroup,
		Size:       en.Size,
		Version:    en.Version,
		Namespace:  en.Namespace,
		Storage:    en.Storage,
		Access:     access,
		CreatedAt:  timestamppb.New(en.CreatedAt),
		ChangedAt:  timestamppb.New(en.ChangedAt),
		ModifiedAt: timestamppb.New(en.ModifiedAt),
		AccessAt:   timestamppb.New(en.AccessAt),
	}

	if parent != nil {
		ed.Parent = toEntryInfo(parent)

		if parent.Properties != nil {
			for k, v := range en.Properties.Fields {
				properties[k] = v
			}
		}
	}

	if en.Properties != nil {
		for k, v := range en.Properties.Fields {
			properties[k] = v
		}
	}

	pl := make([]*Property, 0)
	for k, item := range properties {
		pl = append(pl, &Property{
			Key:     k,
			Value:   item.Value,
			Encoded: item.Encoded,
		})
	}
	return ed, pl
}

func entryDetail(en, parent *types.Entry) *EntryDetail {
	access := &EntryDetail_Access{Uid: en.Access.UID, Gid: en.Access.GID}
	for _, perm := range en.Access.Permissions {
		access.Permissions = append(access.Permissions, string(perm))
	}

	ed := &EntryDetail{
		Id:         en.ID,
		Name:       en.Name,
		Aliases:    en.Aliases,
		Kind:       string(en.Kind),
		IsGroup:    en.IsGroup,
		Size:       en.Size,
		Version:    en.Version,
		Namespace:  en.Namespace,
		Storage:    en.Storage,
		Access:     access,
		CreatedAt:  timestamppb.New(en.CreatedAt),
		ChangedAt:  timestamppb.New(en.ChangedAt),
		ModifiedAt: timestamppb.New(en.ModifiedAt),
		AccessAt:   timestamppb.New(en.AccessAt),
	}
	if parent != nil {
		ed.Parent = entryInfo(parent)
	}
	return ed
}

func eventInfo(evt *types.Event) *Event {
	return &Event{
		Id:              evt.Id,
		Type:            evt.Type,
		Source:          evt.Source,
		SpecVersion:     evt.SpecVersion,
		DataContentType: evt.DataContentType,
		Data: &Event_EventData{
			Id:        evt.Data.ID,
			ParentID:  evt.Data.ParentID,
			Kind:      string(evt.Data.Kind),
			IsGroup:   evt.Data.IsGroup,
			Namespace: evt.Data.Namespace,
		},
		Time:     timestamppb.New(evt.Time),
		RefID:    evt.RefID,
		RefType:  evt.RefType,
		Sequence: evt.Sequence,
	}
}

func jobDetail(j *types.WorkflowJob) *WorkflowJobDetail {
	jd := &WorkflowJobDetail{
		Id:            j.Id,
		Workflow:      j.Workflow,
		TriggerReason: j.TriggerReason,
		Status:        j.Status,
		Message:       j.Message,
		Executor:      j.Executor,
		QueueName:     j.QueueName,
		Target: &WorkflowJobDetail_JobTarget{
			Entries:       j.Target.Entries,
			ParentEntryID: j.Target.ParentEntryID,
		},
		Steps:     nil,
		CreatedAt: timestamppb.New(j.CreatedAt),
		UpdatedAt: timestamppb.New(j.UpdatedAt),
		StartAt:   timestamppb.New(j.StartAt),
		FinishAt:  timestamppb.New(j.FinishAt),
	}

	for _, s := range j.Steps {
		jd.Steps = append(jd.Steps, &WorkflowJobDetail_JobStep{
			Name:    s.StepName,
			Status:  s.Status,
			Message: s.Message,
		})
	}
	return jd
}

func documentInfo(doc *types.Document) *DocumentInfo {
	return &DocumentInfo{
		Id:            doc.EntryId,
		Name:          doc.Name,
		EntryID:       doc.EntryId,
		ParentEntryID: doc.ParentEntryID,
		Source:        doc.Source,
		Marked:        *doc.Marked,
		Unread:        *doc.Unread,
		Namespace:     doc.Namespace,
		SubContent:    doc.SubContent,
		SearchContent: doc.SearchContext,
		HeaderImage:   doc.HeaderImage,
		CreatedAt:     timestamppb.New(doc.CreatedAt),
		ChangedAt:     timestamppb.New(doc.ChangedAt),
	}
}

func buildRootGroup(tree *fs.GroupTree) *GetGroupTreeResponse_GroupEntry {
	result := &GetGroupTreeResponse_GroupEntry{
		Name:     tree.Name,
		Children: make([]*GetGroupTreeResponse_GroupEntry, 0),
	}

	for _, ch := range tree.Children {
		result.Children = append(result.Children, buildRootGroup(ch))
	}
	return result
}

func setupRssConfig(config *CreateEntryRequest_RssConfig, attr *types.EntryAttr) {
	if config == nil || config.Feed == "" {
		return
	}

	const (
		archiveFileTypeUrl        = "url"
		archiveFileTypeHtml       = "html"
		archiveFileTypeRawHtml    = "rawhtml"
		archiveFileTypeWebArchive = "webarchive"
	)

	var fileType string
	switch config.FileType {
	case WebFileType_BookmarkFile:
		fileType = archiveFileTypeUrl
	case WebFileType_HtmlFile:
		fileType = archiveFileTypeHtml
	case WebFileType_RawHtmlFile:
		fileType = archiveFileTypeRawHtml
	case WebFileType_WebArchiveFile:
		fileType = archiveFileTypeWebArchive
	default:
		fileType = archiveFileTypeHtml
	}

	attr.ExtendData = &types.ExtendData{
		PlugScope: &types.PlugScope{
			PluginName: "rss",
			Version:    "1.0",
			PluginType: types.TypeSource,
			Parameters: map[string]string{
				"feed":         config.Feed,
				"file_type":    fileType,
				"clutter_free": "true",
			},
		},
	}

	attr.Labels = types.Labels{Labels: []types.Label{
		{
			Key:   types.LabelKeyPluginKind,
			Value: "source",
		},
		{
			Key:   types.LabelKeyPluginName,
			Value: "rss",
		},
	}}

	if attr.Properties.Fields == nil {
		attr.Properties.Fields = map[string]types.PropertyItem{}
	}
	if config.SiteURL != "" {
		attr.Properties.Fields[types.PropertyWebSiteURL] = types.PropertyItem{Value: config.SiteURL}
	}
	if config.SiteName != "" {
		attr.Properties.Fields[types.PropertyWebSiteName] = types.PropertyItem{Value: config.SiteName}
	}
}

func buildWorkflow(w *types.Workflow) *WorkflowInfo {
	return &WorkflowInfo{
		Id:              w.Id,
		Name:            w.Name,
		QueueName:       w.QueueName,
		CreatedAt:       timestamppb.New(w.CreatedAt),
		UpdatedAt:       timestamppb.New(w.UpdatedAt),
		LastTriggeredAt: timestamppb.New(w.LastTriggeredAt),
	}
}
