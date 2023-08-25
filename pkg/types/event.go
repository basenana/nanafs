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
	"time"
)

type EntryEvent struct {
	Id              string    `json:"id"`
	Type            string    `json:"type"`
	Source          string    `json:"source"`
	SpecVersion     string    `json:"specversion"`
	Time            time.Time `json:"time"`
	RefID           int64     `json:"nanafsrefid"`
	RefType         string    `json:"nanafsreftype"`
	DataContentType string    `json:"datacontenttype"`
	Data            EventData `json:"data"`
}

type EventData struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	ParentID   int64     `json:"parent_id"`
	RefID      int64     `json:"ref_id,omitempty"`
	RefCount   int       `json:"ref_count,omitempty"`
	Kind       Kind      `json:"kind"`
	KindMap    int64     `json:"kind_map"`
	Version    int64     `json:"version"`
	Dev        int64     `json:"dev"`
	Namespace  string    `json:"namespace,omitempty"`
	Storage    string    `json:"storage"`
	CreatedAt  time.Time `json:"created_at"`
	ChangedAt  time.Time `json:"changed_at"`
	ModifiedAt time.Time `json:"modified_at"`
	AccessAt   time.Time `json:"access_at"`
}

func NewEventData(entry *Metadata) EventData {
	return EventData{
		ID:         entry.ID,
		Name:       entry.Name,
		ParentID:   entry.ParentID,
		RefID:      entry.RefID,
		RefCount:   entry.RefCount,
		Kind:       entry.Kind,
		KindMap:    entry.KindMap,
		Version:    entry.Version,
		Dev:        entry.Dev,
		Namespace:  entry.Namespace,
		Storage:    entry.Storage,
		CreatedAt:  entry.CreatedAt,
		ChangedAt:  entry.ChangedAt,
		ModifiedAt: entry.ModifiedAt,
		AccessAt:   entry.AccessAt,
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
	ID      int64
	TaskID  string
	Status  string
	RefType string
	RefID   int64
	Result  string

	CreatedTime    time.Time
	ExecutionTime  time.Time
	ExpirationTime time.Time
	Event          EntryEvent
}

type ScheduledTaskFilter struct {
	RefType string
	RefID   int64
	Status  []string
}

type Notification struct {
	ID      string
	Title   string
	Message string
	Type    string
	Source  string
	Action  string
	Status  string
	Time    time.Time
}
