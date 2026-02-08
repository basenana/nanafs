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

type EntrySelector struct {
	URI string `json:"uri"`
	ID  int64  `json:"id"`
}

type EntryDetailRequest struct {
	EntrySelector
}

type FileContentRequest struct {
	EntrySelector
}

type ListGroupChildrenRequest struct {
	EntrySelector
	Page     int64  `json:"page"`
	PageSize int64  `json:"page_size"`
	Sort     string `json:"sort"`
	Order    string `json:"order"`
}

type CreateEntryRequest struct {
	URI    string        `json:"uri" binding:"required"`
	Kind   string        `json:"kind"`
	Rss    *RssConfig    `json:"rss"`
	Filter *FilterConfig `json:"filter"`

	Properties *types.Properties         `json:"properties,omitempty"`
	Document   *types.DocumentProperties `json:"document,omitempty"`
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

type UpdateEntryRequest struct {
	EntrySelector
	Aliases string `json:"aliases"`
}

type ChangeParentRequest struct {
	EntryURI    string `json:"entry_uri" binding:"required"`
	NewEntryURI string `json:"new_entry_uri" binding:"required"`
	Replace     bool   `json:"replace"`
	Exchange    bool   `json:"exchange"`
}

type DeleteEntriesRequest struct {
	URIList []string `json:"uri_list" binding:"required"`
}

type PaginationRequest struct {
	Page     int64  `form:"page"`
	PageSize int64  `form:"page_size"`
	Sort     string `form:"sort"`
	Order    string `form:"order"`
}

type FilterEntryRequest struct {
	CELPattern string `json:"cel_pattern" binding:"required"` // CEL pattern for filtering
	Page       int64  `json:"page"`                           // Page number
	PageSize   int64  `json:"page_size"`                      // Page size
	Sort       string `json:"sort"`                           // Sort field
	Order      string `json:"order"`                          // Order direction (asc/desc)
}

type SearchDocumentsRequest struct {
	Query    string `json:"query" binding:"required"` // Search keywords
	Page     int64  `json:"page"`                     // Page number
	PageSize int64  `json:"page_size"`                // Page size
}

type UpdatePropertyRequest struct {
	EntrySelector
	Tags       []string          `json:"tags"`
	Properties map[string]string `json:"properties"`
}

type UpdateDocumentPropertyRequest struct {
	EntrySelector
	Title       *string  `json:"title,omitempty"`
	Author      *string  `json:"author,omitempty"`
	Year        *string  `json:"year,omitempty"`
	Source      *string  `json:"source,omitempty"`
	Abstract    *string  `json:"abstract,omitempty"`
	Notes       *string  `json:"notes,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
	URL         *string  `json:"url,omitempty"`
	SiteName    *string  `json:"site_name,omitempty"`
	SiteURL     *string  `json:"site_url,omitempty"`
	HeaderImage *string  `json:"headerImage,omitempty"`
	Unread      *bool    `json:"unread,omitempty"`
	Marked      *bool    `json:"marked,omitempty"`
	PublishAt   *string  `json:"publish_at,omitempty"`
}

type ReadMessagesRequest struct {
	MessageIDList []string `json:"message_id_list" binding:"required"`
}

type TriggerWorkflowRequest struct {
	URI        string            `json:"uri"`
	Parameters map[string]string `json:"parameters"`
	Reason     string            `json:"reason"`
	Timeout    int64             `json:"timeout"`
}

type SetConfigRequest struct {
	Value string `json:"value" binding:"required"`
}

type CreateWorkflowRequest struct {
	Name      string                `json:"name" binding:"required"`
	Trigger   types.WorkflowTrigger `json:"trigger"`
	Nodes     []types.WorkflowNode  `json:"nodes"`
	Enable    bool                  `json:"enable"`
	QueueName string                `json:"queue_name"`
}

type UpdateWorkflowRequest struct {
	Name      string                `json:"name"`
	Trigger   types.WorkflowTrigger `json:"trigger"`
	Nodes     []types.WorkflowNode  `json:"nodes"`
	Enable    *bool                 `json:"enable"`
	QueueName string                `json:"queue_name"`
}

type GetGroupConfigRequest struct {
	EntrySelector
}

type SetGroupConfigRequest struct {
	EntrySelector
	Rss    *RssConfig    `json:"rss,omitempty"`
	Filter *FilterConfig `json:"filter,omitempty"`
}
