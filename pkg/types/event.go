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

package types

import (
	"encoding/json"
	"time"
)

type Event struct {
	Id              string    `json:"id"`
	Type            string    `json:"type"`
	Source          string    `json:"source"`
	SpecVersion     string    `json:"specversion"`
	RefID           int64     `json:"nanafsrefid"`
	RefType         string    `json:"nanafsreftype"`
	Sequence        int64     `json:"nanafssequence"`
	Namespace       string    `json:"namespace"`
	DataContentType string    `json:"datacontenttype"`
	Data            EventData `json:"data"`
	Time            time.Time `json:"time"`
}

type EventData struct {
	ID        int64  `json:"id"`
	ParentID  int64  `json:"parent_id,omitempty"`
	Kind      Kind   `json:"kind,omitempty"`
	KindMap   int64  `json:"kind_map,omitempty"`
	IsGroup   bool   `json:"is_group,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

func (e EventData) String() string {
	raw, err := json.Marshal(e)
	if err == nil {
		return string(raw)
	}
	return "{}"
}

func NewEventDataFromEntry(entry *Metadata) EventData {
	return EventData{
		ID:        entry.ID,
		ParentID:  entry.ParentID,
		Kind:      entry.Kind,
		IsGroup:   entry.IsGroup,
		Namespace: entry.Namespace,
	}
}

func NewEventDataFromDocument(doc *Document) EventData {
	return EventData{
		ID:        doc.ID,
		ParentID:  doc.ParentEntryID,
		Namespace: doc.Namespace,
	}
}

const (
	ScheduledTaskInitial   = "initial"
	ScheduledTaskWait      = "wait"
	ScheduledTaskExecuting = "executing"
	ScheduledTaskFinish    = "finish"
	ScheduledTaskSucceed   = "succeed"
	ScheduledTaskFailed    = "failed"
)

type ScheduledTask struct {
	ID        int64
	Namespace string
	TaskID    string
	Status    string
	RefType   string
	RefID     int64
	Result    string

	CreatedTime    time.Time
	ExecutionTime  time.Time
	ExpirationTime time.Time
	Event          Event
}

type ScheduledTaskFilter struct {
	RefType string
	RefID   int64
	Status  []string
}

const (
	NotificationInfo   = "info"
	NotificationWarn   = "warn"
	NotificationUnread = "unread"
	NotificationRead   = "read"
)

type Notification struct {
	ID        string
	Namespace string
	Title     string
	Message   string
	Type      string
	Source    string
	Action    string
	Status    string
	Time      time.Time
}
