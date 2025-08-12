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
	"google.golang.org/protobuf/types/known/timestamppb"
	"path"
	"time"

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
		Entry:      en.ID,
		Name:       en.Name,
		Kind:       string(en.Kind),
		IsGroup:    en.IsGroup,
		Size:       en.Size,
		CreatedAt:  timestamppb.New(en.CreatedAt),
		ChangedAt:  timestamppb.New(en.ChangedAt),
		ModifiedAt: timestamppb.New(en.ModifiedAt),
		AccessAt:   timestamppb.New(en.AccessAt),
	}
}

func coreEntryInfo(parentID int64, name string, en *types.Entry) *EntryInfo {
	return &EntryInfo{
		Entry:      en.ID,
		Name:       name,
		Kind:       string(en.Kind),
		IsGroup:    en.IsGroup,
		Size:       en.Size,
		CreatedAt:  timestamppb.New(en.CreatedAt),
		ChangedAt:  timestamppb.New(en.ChangedAt),
		ModifiedAt: timestamppb.New(en.ModifiedAt),
		AccessAt:   timestamppb.New(en.AccessAt),
	}
}

func toEntryInfo(parentURI, name string, en *types.Entry, doc *types.DocumentProperties) *EntryInfo {
	info := &EntryInfo{
		Uri:        path.Join(parentURI, name),
		Entry:      en.ID,
		Name:       name,
		Kind:       string(en.Kind),
		IsGroup:    en.IsGroup,
		Size:       en.Size,
		CreatedAt:  timestamppb.New(en.CreatedAt),
		ChangedAt:  timestamppb.New(en.ChangedAt),
		ModifiedAt: timestamppb.New(en.ModifiedAt),
		AccessAt:   timestamppb.New(en.AccessAt),
	}

	if doc != nil {
		info.Document = &DocumentProperty{
			Title:       doc.Title,
			Author:      doc.Author,
			Year:        doc.Year,
			Source:      doc.Source,
			Abstract:    doc.Abstract,
			Keywords:    doc.Keywords,
			Notes:       doc.Notes,
			Unread:      doc.Unread,
			Marked:      doc.Marked,
			PublishAt:   timestamppb.New(time.Unix(doc.PublishAt, 0)),
			Url:         doc.URL,
			HeaderImage: doc.HeaderImage,
		}
	}
	return info
}

func toEntryDetail(parentURI, name string, en *types.Entry, doc types.DocumentProperties) *EntryDetail {
	access := &EntryDetail_Access{Uid: en.Access.UID, Gid: en.Access.GID}
	for _, perm := range en.Access.Permissions {
		access.Permissions = append(access.Permissions, string(perm))
	}

	ed := &EntryDetail{
		Uri:       path.Join(parentURI, name),
		Entry:     en.ID,
		Name:      name,
		Aliases:   en.Aliases,
		Kind:      string(en.Kind),
		IsGroup:   en.IsGroup,
		Size:      en.Size,
		Version:   en.Version,
		Namespace: en.Namespace,
		Storage:   en.Storage,
		Access:    access,
		Document: &DocumentProperty{
			Title:       doc.Title,
			Author:      doc.Author,
			Year:        doc.Year,
			Source:      doc.Source,
			Abstract:    doc.Abstract,
			Keywords:    doc.Keywords,
			Notes:       doc.Notes,
			Unread:      doc.Unread,
			Marked:      doc.Marked,
			PublishAt:   timestamppb.New(time.Unix(doc.PublishAt, 0)),
			Url:         doc.URL,
			HeaderImage: doc.HeaderImage,
		},
		CreatedAt:  timestamppb.New(en.CreatedAt),
		ChangedAt:  timestamppb.New(en.ChangedAt),
		ModifiedAt: timestamppb.New(en.ModifiedAt),
		AccessAt:   timestamppb.New(en.AccessAt),
	}
	return ed
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
			Entries: j.Targets.Entries,
		},
		Steps:     nil,
		CreatedAt: timestamppb.New(j.CreatedAt),
		UpdatedAt: timestamppb.New(j.UpdatedAt),
		StartAt:   timestamppb.New(j.StartAt),
		FinishAt:  timestamppb.New(j.FinishAt),
	}

	for _, s := range j.Nodes {
		jd.Steps = append(jd.Steps, &WorkflowJobDetail_JobStep{
			Name:    s.Name,
			Status:  s.Status,
			Message: s.Message,
		})
	}
	return jd
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

func setupGroupFilterConfig(config *CreateEntryRequest_FilterConfig, attr *types.EntryAttr) {
	attr.GroupProperties = &types.GroupProperties{
		Filter: &types.Filter{
			CELPattern: config.CelPattern,
		},
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

type GroupTree struct {
	ID       int64
	Name     string
	Children []*GroupTree
}

type EnOrder int

const (
	EntryName EnOrder = iota
	EntryKind
	EntryIsGroup
	EntrySize
	EntryCreatedAt
	EntryModifiedAt
)

func (d EnOrder) String() string {
	names := []string{
		"name",
		"kind",
		"is_group",
		"size",
		"created_at",
		"modified_at",
	}
	if d < EntryName || d > EntryModifiedAt {
		return "Unknown"
	}
	return names[d]
}

type EntryOrder struct {
	Order EnOrder
	Desc  bool
}
