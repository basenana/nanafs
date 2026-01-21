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
	"path"
	"time"

	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
)

type EntryInfo struct {
	URI        string            `json:"uri"`
	Entry      int64             `json:"entry"`
	Name       string            `json:"name"`
	Kind       string            `json:"kind"`
	IsGroup    bool              `json:"is_group"`
	Size       int64             `json:"size"`
	Document   *DocumentProperty `json:"document,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	ChangedAt  time.Time         `json:"changed_at"`
	ModifiedAt time.Time         `json:"modified_at"`
	AccessAt   time.Time         `json:"access_at"`
}

type EntryDetail struct {
	URI        string            `json:"uri"`
	Entry      int64             `json:"entry"`
	Name       string            `json:"name"`
	Aliases    string            `json:"aliases"`
	Kind       string            `json:"kind"`
	IsGroup    bool              `json:"is_group"`
	Size       int64             `json:"size"`
	Version    int64             `json:"version"`
	Namespace  string            `json:"namespace"`
	Storage    string            `json:"storage"`
	Access     *Access           `json:"access"`
	Property   *Property         `json:"property"`
	Document   *DocumentProperty `json:"document,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	ChangedAt  time.Time         `json:"changed_at"`
	ModifiedAt time.Time         `json:"modified_at"`
	AccessAt   time.Time         `json:"access_at"`
}

type Access struct {
	UID         int64    `json:"uid"`
	GID         int64    `json:"gid"`
	Permissions []string `json:"permissions"`
}

type DocumentProperty struct {
	Title       string     `json:"title,omitempty"`
	Author      string     `json:"author,omitempty"`
	Year        string     `json:"year,omitempty"`
	Source      string     `json:"source,omitempty"`
	Abstract    string     `json:"abstract,omitempty"`
	Keywords    []string   `json:"keywords,omitempty"`
	Notes       string     `json:"notes,omitempty"`
	Unread      bool       `json:"unread,omitempty"`
	Marked      bool       `json:"marked,omitempty"`
	PublishAt   *time.Time `json:"publish_at,omitempty"`
	URL         string     `json:"url,omitempty"`
	SiteName    string     `json:"site_name,omitempty"`
	SiteURL     string     `json:"site_url,omitempty"`
	HeaderImage string     `json:"header_image,omitempty"`
}

type Property struct {
	Tags         []string          `json:"tags,omitempty"`
	IndexVersion string            `json:"index_version,omitempty"`
	Summarize    string            `json:"summarize,omitempty"`
	URL          string            `json:"url,omitempty"`
	SiteName     string            `json:"site_name,omitempty"`
	Properties   map[string]string `json:"properties,omitempty"`
}

type FridayProperty struct {
	Summary string `json:"summary,omitempty"`
}

type FridayPropertyResponse struct {
	Property *FridayProperty `json:"property"`
}

type GroupEntry struct {
	Name     string        `json:"name"`
	URI      string        `json:"uri"`
	Children []*GroupEntry `json:"children"`
}

type GroupTreeResponse struct {
	Root *GroupEntry `json:"root"`
}

type Message struct {
	ID      string    `json:"id"`
	Title   string    `json:"title"`
	Message string    `json:"message"`
	Type    string    `json:"type"`
	Source  string    `json:"source"`
	Action  string    `json:"action"`
	Status  string    `json:"status"`
	Time    time.Time `json:"time"`
}

type WorkflowInfo struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	QueueName       string    `json:"queue_name"`
	Enable          bool      `json:"enable"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	LastTriggeredAt time.Time `json:"last_triggered_at"`
}

type WorkflowDetail struct {
	ID              string                `json:"id"`
	Namespace       string                `json:"namespace"`
	Name            string                `json:"name"`
	Trigger         types.WorkflowTrigger `json:"trigger"`
	Nodes           []types.WorkflowNode  `json:"nodes"`
	Enable          bool                  `json:"enable"`
	QueueName       string                `json:"queue_name"`
	CreatedAt       time.Time             `json:"created_at"`
	UpdatedAt       time.Time             `json:"updated_at"`
	LastTriggeredAt time.Time             `json:"last_triggered_at"`
}

type WorkflowJobStep struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type WorkflowJobTarget struct {
	Entries []string `json:"entries"`
}

type WorkflowJobDetail struct {
	ID            string             `json:"id"`
	Workflow      string             `json:"workflow"`
	TriggerReason string             `json:"trigger_reason"`
	Status        string             `json:"status"`
	Message       string             `json:"message"`
	QueueName     string             `json:"queue_name"`
	Target        *WorkflowJobTarget `json:"target"`
	Steps         []*WorkflowJobStep `json:"steps"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
	StartAt       time.Time          `json:"start_at"`
	FinishAt      time.Time          `json:"finish_at"`
}

type DeleteEntriesResponse struct {
	Deleted []string `json:"deleted"`
	Message string   `json:"message"`
}

type WriteFileResponse struct {
	Len int64 `json:"len"`
}

type PaginationInfo struct {
	Page     int64 `json:"page,omitempty"`
	PageSize int64 `json:"page_size,omitempty"`
}

type ListEntriesResponse struct {
	Entries    []*EntryInfo    `json:"entries"`
	Pagination *PaginationInfo `json:"pagination,omitempty"`
}

type DocumentInfo struct {
	ID        int64     `json:"id"`
	URI       string    `json:"uri"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreateAt  time.Time `json:"create_at"`
	ChangedAt time.Time `json:"changed_at"`
}

type SearchDocumentsResponse struct {
	Documents  []*DocumentInfo `json:"documents"`
	Pagination *PaginationInfo `json:"pagination,omitempty"`
}

type ListMessagesResponse struct {
	Messages []*Message `json:"messages"`
}

type ReadMessagesResponse struct {
	Success bool `json:"success"`
}

type ListWorkflowsResponse struct {
	Workflows  []*WorkflowInfo `json:"workflows"`
	Pagination *PaginationInfo `json:"pagination,omitempty"`
}

type ListWorkflowJobsResponse struct {
	Jobs       []*WorkflowJobDetail `json:"jobs"`
	Pagination *PaginationInfo      `json:"pagination,omitempty"`
}

type TriggerWorkflowResponse struct {
	JobID string `json:"job_id"`
}

func toEntryInfo(parentURI, name string, en *types.Entry, doc *types.DocumentProperties) *EntryInfo {
	info := &EntryInfo{
		URI:        path.Join(parentURI, name),
		Entry:      en.ID,
		Name:       name,
		Kind:       string(en.Kind),
		IsGroup:    en.IsGroup,
		Size:       en.Size,
		CreatedAt:  en.CreatedAt,
		ChangedAt:  en.ChangedAt,
		ModifiedAt: en.ModifiedAt,
		AccessAt:   en.AccessAt,
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
			PublishAt:   timestampTime(doc.PublishAt),
			URL:         doc.URL,
			SiteName:    doc.SiteName,
			SiteURL:     doc.SiteURL,
			HeaderImage: doc.HeaderImage,
		}
	}
	return info
}

func toEntryDetail(parentURI, name string, en *types.Entry, doc types.DocumentProperties, prop *Property) *EntryDetail {
	access := &Access{
		UID:         en.Access.UID,
		GID:         en.Access.GID,
		Permissions: make([]string, len(en.Access.Permissions)),
	}
	for i, perm := range en.Access.Permissions {
		access.Permissions[i] = string(perm)
	}

	ed := &EntryDetail{
		URI:       path.Join(parentURI, name),
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
		Property:  prop,
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
			PublishAt:   timestampTime(doc.PublishAt),
			URL:         doc.URL,
			SiteName:    doc.SiteName,
			SiteURL:     doc.SiteURL,
			HeaderImage: doc.HeaderImage,
		},
		CreatedAt:  en.CreatedAt,
		ChangedAt:  en.ChangedAt,
		ModifiedAt: en.ModifiedAt,
		AccessAt:   en.AccessAt,
	}
	return ed
}

func toWorkflowInfo(w *types.Workflow) *WorkflowInfo {
	return &WorkflowInfo{
		ID:              w.Id,
		Name:            w.Name,
		QueueName:       w.QueueName,
		Enable:          w.Enable,
		CreatedAt:       w.CreatedAt,
		UpdatedAt:       w.UpdatedAt,
		LastTriggeredAt: w.LastTriggeredAt,
	}
}

func toWorkflowDetail(w *types.Workflow) *WorkflowDetail {
	return &WorkflowDetail{
		ID:              w.Id,
		Namespace:       w.Namespace,
		Name:            w.Name,
		Trigger:         w.Trigger,
		Nodes:           w.Nodes,
		Enable:          w.Enable,
		QueueName:       w.QueueName,
		CreatedAt:       w.CreatedAt,
		UpdatedAt:       w.UpdatedAt,
		LastTriggeredAt: w.LastTriggeredAt,
	}
}

type ConfigResponse struct {
	Group string `json:"group"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ListConfigResponse struct {
	Items []types.ConfigItem `json:"items"`
}

type DeleteConfigResponse struct {
	Group   string `json:"group"`
	Name    string `json:"name"`
	Deleted bool   `json:"deleted"`
}

type EntryResponse struct {
	Entry *EntryInfo `json:"entry"`
}

type EntryDetailResponse struct {
	Entry *EntryDetail `json:"entry"`
}

type AppendFileResponse struct {
	Entry   *EntryInfo   `json:"entry"`
	Entries []*EntryInfo `json:"entries"`
}

type UpdateDocumentPropertyResponse struct {
	Deleted []string     `json:"deleted"`
	Entries []*EntryInfo `json:"entries"`
}

type WorkflowResponse struct {
	Workflow *WorkflowDetail `json:"workflow"`
}

type CreateWorkflowResponse struct {
	Workflow *WorkflowInfo `json:"workflow"`
}

type WorkflowJobResponse struct {
	Job *WorkflowJobDetail `json:"job"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

type SetConfigResponse struct {
	Group string `json:"group"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

type PropertyResponse struct {
	Property *Property `json:"properties"`
}

type DocumentPropertyResponse struct {
	Property *DocumentProperty `json:"properties"`
}

func toWorkflowJobDetail(j *types.WorkflowJob) *WorkflowJobDetail {
	jd := &WorkflowJobDetail{
		ID:            j.Id,
		Workflow:      j.Workflow,
		TriggerReason: j.TriggerReason,
		Status:        j.Status,
		Message:       j.Message,
		QueueName:     j.QueueName,
		Target: &WorkflowJobTarget{
			Entries: j.Targets.Entries,
		},
		Steps:     make([]*WorkflowJobStep, 0, len(j.Nodes)),
		CreatedAt: j.CreatedAt,
		UpdatedAt: j.UpdatedAt,
		StartAt:   j.StartAt,
		FinishAt:  j.FinishAt,
	}

	for _, s := range j.Nodes {
		jd.Steps = append(jd.Steps, &WorkflowJobStep{
			Name:    s.Name,
			Status:  s.Status,
			Message: s.Message,
		})
	}
	return jd
}

func timestampTime(ts int64) *time.Time {
	if ts == 0 {
		return nil
	}
	return utils.ToPtr(time.Unix(ts, 0))
}

func nanoTimestampToTime(ts int64) time.Time {
	if ts == 0 {
		return time.Time{}
	}
	return time.Unix(0, ts)
}

func timeToTimestamp(ts string) int64 {
	if ts == "" {
		return 0
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return 0
	}
	return t.Unix()
}

func toProperty(props *types.Properties) *Property {
	if props == nil {
		return nil
	}
	return &Property{
		Tags:         props.Tags,
		IndexVersion: props.IndexVersion,
		Summarize:    props.Summarize,
		URL:          props.URL,
		SiteName:     props.SiteName,
		Properties:   props.Properties,
	}
}

func toFridayProperty(props *types.FridayProcessProperties) *FridayProperty {
	if props == nil {
		return nil
	}
	return &FridayProperty{
		Summary: props.Summary,
	}
}

type PluginParameter struct {
	Name        string   `json:"name"`
	Required    bool     `json:"required"`
	Default     string   `json:"default,omitempty"`
	Description string   `json:"description,omitempty"`
	Options     []string `json:"options,omitempty"`
}

type PluginInfo struct {
	Name           string            `json:"name"`
	Version        string            `json:"version"`
	Type           string            `json:"type"`
	InitParameters []PluginParameter `json:"init_parameters"`
	Parameters     []PluginParameter `json:"parameters"`
}

type ListPluginsResponse struct {
	Plugins []*PluginInfo `json:"plugins"`
}
