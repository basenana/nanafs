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

// EntryInfo 条目信息
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

// EntryDetail 条目详情
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

// Access 访问权限
type Access struct {
	UID         int64    `json:"uid"`
	GID         int64    `json:"gid"`
	Permissions []string `json:"permissions"`
}

// DocumentProperty 文档属性
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

// Property 属性
type Property struct {
	Tags       []string          `json:"tags,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
}

// GroupEntry 组条目
type GroupEntry struct {
	Name     string        `json:"name"`
	URI      string        `json:"uri"`
	Children []*GroupEntry `json:"children"`
}

// GroupTreeResponse 组树响应
type GroupTreeResponse struct {
	Root *GroupEntry `json:"root"`
}

// Message 消息
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

// WorkflowInfo 工作流信息 (列表用简化版)
type WorkflowInfo struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	QueueName       string    `json:"queue_name"`
	Enable          bool      `json:"enable"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	LastTriggeredAt time.Time `json:"last_triggered_at"`
}

// WorkflowDetail 工作流详情 (完整信息)
type WorkflowDetail struct {
	ID              string                   `json:"id"`
	Namespace       string                   `json:"namespace"`
	Name            string                   `json:"name"`
	Trigger         types.WorkflowTrigger    `json:"trigger"`
	Nodes           []types.WorkflowNode     `json:"nodes"`
	Enable          bool                     `json:"enable"`
	QueueName       string                   `json:"queue_name"`
	CreatedAt       time.Time                `json:"created_at"`
	UpdatedAt       time.Time                `json:"updated_at"`
	LastTriggeredAt time.Time                `json:"last_triggered_at"`
}

// WorkflowJobStep 工作流作业步骤
type WorkflowJobStep struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// WorkflowJobTarget 工作流作业目标
type WorkflowJobTarget struct {
	Entries []string `json:"entries"`
}

// WorkflowJobDetail 工作流作业详情
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

// DeleteEntriesResponse 批量删除响应
type DeleteEntriesResponse struct {
	Deleted []string `json:"deleted"`
	Message string   `json:"message"`
}

// WriteFileResponse 写文件响应
type WriteFileResponse struct {
	Len int64 `json:"len"`
}

// PaginationInfo 分页信息
type PaginationInfo struct {
	Page     int64 `json:"page"`
	PageSize int64 `json:"page_size"`
}

// ListEntriesResponse 列出条目响应
type ListEntriesResponse struct {
	Entries    []*EntryInfo    `json:"entries"`
	Pagination *PaginationInfo `json:"pagination,omitempty"`
}

// ListMessagesResponse 列出消息响应
type ListMessagesResponse struct {
	Messages []*Message `json:"messages"`
}

// ReadMessagesResponse 读取消息响应
type ReadMessagesResponse struct {
	Success bool `json:"success"`
}

// ListWorkflowsResponse 列出工作流响应
type ListWorkflowsResponse struct {
	Workflows  []*WorkflowInfo `json:"workflows"`
	Pagination *PaginationInfo `json:"pagination,omitempty"`
}

// ListWorkflowJobsResponse 列出工作流作业响应
type ListWorkflowJobsResponse struct {
	Jobs       []*WorkflowJobDetail `json:"jobs"`
	Pagination *PaginationInfo      `json:"pagination,omitempty"`
}

// TriggerWorkflowResponse 触发工作流响应
type TriggerWorkflowResponse struct {
	JobID string `json:"job_id"`
}

// Helpers for converting from types package

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

// ConfigResponse 配置响应
type ConfigResponse struct {
	Group string `json:"group"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ListConfigResponse 列出配置响应
type ListConfigResponse struct {
	Items []types.ConfigItem `json:"items"`
}

// DeleteConfigResponse 删除配置响应
type DeleteConfigResponse struct {
	Group   string `json:"group"`
	Name    string `json:"name"`
	Deleted bool   `json:"deleted"`
}

// EntryResponse 条目响应
type EntryResponse struct {
	Entry *EntryInfo `json:"entry"`
}

// EntryDetailResponse 条目详情响应
type EntryDetailResponse struct {
	Entry *EntryDetail `json:"entry"`
}

// AppendFileResponse 追加文件响应
type AppendFileResponse struct {
	Entry   *EntryInfo   `json:"entry"`
	Entries []*EntryInfo `json:"entries"`
}

// UpdateDocumentPropertyResponse 更新文档属性响应
type UpdateDocumentPropertyResponse struct {
	Deleted []string     `json:"deleted"`
	Entries []*EntryInfo `json:"entries"`
}

// WorkflowResponse 工作流响应
type WorkflowResponse struct {
	Workflow *WorkflowDetail `json:"workflow"`
}

// CreateWorkflowResponse 创建工作流响应
type CreateWorkflowResponse struct {
	Workflow *WorkflowInfo `json:"workflow"`
}

// WorkflowJobResponse 工作流作业响应
type WorkflowJobResponse struct {
	Job *WorkflowJobDetail `json:"job"`
}

// MessageResponse 消息响应
type MessageResponse struct {
	Message string `json:"message"`
}

// SetConfigResponse 设置配置响应
type SetConfigResponse struct {
	Group string `json:"group"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

// PropertyResponse 属性响应
type PropertyResponse struct {
	Property *Property `json:"properties"`
}

// DocumentPropertyResponse 文档属性响应
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
