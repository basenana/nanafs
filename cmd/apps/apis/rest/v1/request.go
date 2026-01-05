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

import "github.com/basenana/nanafs/pkg/types"

// CreateEntryRequest 创建条目请求
type CreateEntryRequest struct {
	URI    string         `json:"uri" binding:"required"`
	Kind   string         `json:"kind"`
	Rss    *RssConfig     `json:"rss"`
	Filter *FilterConfig  `json:"filter"`
}

type RssConfig struct {
	Feed     string `json:"feed"`
	SiteName string `json:"site_name"`
	SiteURL  string `json:"site_url"`
	FileType string `json:"file_type"`
}

type FilterConfig struct {
	CELPattern string `json:"cel_pattern"`
}

// UpdateEntryRequest 更新条目请求
type UpdateEntryRequest struct {
	Name    string `json:"name"`
	Aliases string `json:"aliases"`
}

// ChangeParentRequest 更改父级请求
type ChangeParentRequest struct {
	EntryURI    string `json:"entry_uri" binding:"required"`
	NewEntryURI string `json:"new_entry_uri" binding:"required"`
	Replace     bool   `json:"replace"`
	Exchange    bool   `json:"exchange"`
}

// DeleteEntriesRequest 批量删除请求
type DeleteEntriesRequest struct {
	URIList []string `json:"uri_list" binding:"required"`
}

// ListGroupChildrenRequest 列出组子项请求
type ListGroupChildrenRequest struct {
	Offset int64  `form:"offset"`
	Limit  int64  `form:"limit"`
	Order  string `form:"order"`
	Desc   bool   `form:"desc"`
}

// FilterEntryRequest 过滤条目请求
type FilterEntryRequest struct {
	CELPattern string `json:"cel_pattern" binding:"required"`
}

// UpdatePropertyRequest 更新属性请求
type UpdatePropertyRequest struct {
	Tags       []string          `json:"tags"`
	Properties map[string]string `json:"properties"`
}

// UpdateDocumentPropertyRequest 更新文档属性请求
type UpdateDocumentPropertyRequest struct {
	Unread bool `json:"unread"`
	Marked bool `json:"marked"`
}

// ListMessagesRequest 列出消息请求
type ListMessagesRequest struct {
	All bool `form:"all"`
}

// ReadMessagesRequest 读取消息请求
type ReadMessagesRequest struct {
	MessageIDList []string `json:"message_id_list" binding:"required"`
}

// ListWorkflowJobsRequest 列出工作流作业请求
type ListWorkflowJobsRequest struct {
	WorkflowID string `form:"workflow_id"`
}

// TriggerWorkflowRequest 触发工作流请求
type TriggerWorkflowRequest struct {
	URI     string `json:"uri"`
	Reason  string `json:"reason"`
	Timeout int64  `json:"timeout"`
}

// SetConfigRequest 设置配置请求
type SetConfigRequest struct {
	Value string `json:"value" binding:"required"`
}

// CreateWorkflowRequest 创建工作流请求
type CreateWorkflowRequest struct {
	Name      string            `json:"name" binding:"required"`
	Trigger   types.WorkflowTrigger `json:"trigger"`
	Nodes     []types.WorkflowNode `json:"nodes"`
	Enable    bool              `json:"enable"`
	QueueName string            `json:"queue_name"`
}

// UpdateWorkflowRequest 更新工作流请求
type UpdateWorkflowRequest struct {
	Name      string            `json:"name"`
	Trigger   types.WorkflowTrigger `json:"trigger"`
	Nodes     []types.WorkflowNode `json:"nodes"`
	Enable    *bool             `json:"enable"`
	QueueName string            `json:"queue_name"`
}
