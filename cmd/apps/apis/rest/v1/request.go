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

// CreateEntryRequest 创建条目请求
type CreateEntryRequest struct {
	URI  string         `json:"uri" binding:"required"`
	Kind string         `json:"kind"`
	Rss  *RssConfig     `json:"rss"`
	Filter *FilterConfig `json:"filter"`
}

type RssConfig struct {
	Feed     string `json:"feed"`
	SiteName string `json:"siteName"`
	SiteURL  string `json:"siteUrl"`
	FileType string `json:"fileType"`
}

type FilterConfig struct {
	CELPattern string `json:"celPattern"`
}

// UpdateEntryRequest 更新条目请求
type UpdateEntryRequest struct {
	Name    string `json:"name"`
	Aliases string `json:"aliases"`
}

// ChangeParentRequest 更改父级请求
type ChangeParentRequest struct {
	EntryURI   string `json:"entryUri" binding:"required"`
	NewEntryURI string `json:"newEntryUri" binding:"required"`
	Replace    bool   `json:"replace"`
	Exchange   bool   `json:"exchange"`
}

// DeleteEntriesRequest 批量删除请求
type DeleteEntriesRequest struct {
	URIList []string `json:"uriList" binding:"required"`
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
	CELPattern string `json:"celPattern" binding:"required"`
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
	MessageIDList []string `json:"messageIdList" binding:"required"`
}

// ListWorkflowJobsRequest 列出工作流作业请求
type ListWorkflowJobsRequest struct {
	WorkflowID string `form:"workflowId"`
}

// TriggerWorkflowRequest 触发工作流请求
type TriggerWorkflowRequest struct {
	WorkflowID string            `json:"workflowId" binding:"required"`
	URI        string            `json:"uri"`
	Reason     string            `json:"reason"`
	Timeout    int64             `json:"timeout"`
}
