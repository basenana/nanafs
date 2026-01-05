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
	"time"

	"github.com/basenana/nanafs/pkg/types"
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
	URI       string            `json:"uri"`
	Entry     int64             `json:"entry"`
	Name      string            `json:"name"`
	Aliases   string            `json:"aliases"`
	Kind      string            `json:"kind"`
	IsGroup   bool              `json:"is_group"`
	Size      int64             `json:"size"`
	Version   int64             `json:"version"`
	Namespace string            `json:"namespace"`
	Storage   string            `json:"storage"`
	Access    *Access           `json:"access"`
	Property  *Property         `json:"property"`
	Document  *DocumentProperty `json:"document,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	ChangedAt time.Time         `json:"changed_at"`
	ModifiedAt time.Time        `json:"modified_at"`
	AccessAt  time.Time         `json:"access_at"`
}

// Access 访问权限
type Access struct {
	UID         int64    `json:"uid"`
	GID         int64    `json:"gid"`
	Permissions []string `json:"permissions"`
}

// DocumentProperty 文档属性
type DocumentProperty struct {
	Title       string    `json:"title"`
	Author      string    `json:"author"`
	Year        string    `json:"year"`
	Source      string    `json:"source"`
	Abstract    string    `json:"abstract"`
	Keywords    []string  `json:"keywords"`
	Notes       string    `json:"notes"`
	Unread      bool      `json:"unread"`
	Marked      bool      `json:"marked"`
	PublishAt   time.Time `json:"publish_at"`
	URL         string    `json:"url"`
	HeaderImage string    `json:"header_image"`
}

// Property 属性
type Property struct {
	Tags       []string          `json:"tags"`
	Properties map[string]string `json:"properties"`
}

// GroupEntry 组条目
type GroupEntry struct {
	Name     string       `json:"name"`
	URI      string       `json:"uri"`
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

// WorkflowInfo 工作流信息
type WorkflowInfo struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	QueueName       string    `json:"queue_name"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	LastTriggeredAt time.Time `json:"last_triggered_at"`
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

// ListEntriesResponse 列出条目响应
type ListEntriesResponse struct {
	Entries []*EntryInfo `json:"entries"`
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
	Workflows []*WorkflowInfo `json:"workflows"`
}

// ListWorkflowJobsResponse 列出工作流作业响应
type ListWorkflowJobsResponse struct {
	Jobs []*WorkflowJobDetail `json:"jobs"`
}

// TriggerWorkflowResponse 触发工作流响应
type TriggerWorkflowResponse struct {
	JobID string `json:"job_id"`
}

// Helpers for converting from types package

func toEntryInfo(parentURI, name string, en *types.Entry, doc *types.DocumentProperties) *EntryInfo {
	info := &EntryInfo{
		URI:        parentURI + name,
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
			PublishAt:   time.Unix(doc.PublishAt, 0),
			URL:         doc.URL,
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
		URI:       parentURI + name,
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
			PublishAt:   time.Unix(doc.PublishAt, 0),
			URL:         doc.URL,
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
		CreatedAt:       w.CreatedAt,
		UpdatedAt:       w.UpdatedAt,
		LastTriggeredAt: w.LastTriggeredAt,
	}
}

// ConfigResponse 配置响应
type ConfigResponse struct {
	Group     string    `json:"group"`
	Name      string    `json:"name"`
	Value     string    `json:"value"`
	ChangedAt time.Time `json:"changed_at"`
}

// ListConfigResponse 列出配置响应
type ListConfigResponse struct {
	Items []types.ConfigItem `json:"items"`
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
