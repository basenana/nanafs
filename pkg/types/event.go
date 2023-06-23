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

type Event struct {
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
	*Metadata
}

const (
	ScheduledTaskInitial   = "initial"
	ScheduledTaskWait      = "wait"
	ScheduledTaskExecuting = "executing"
	ScheduledTaskFinish    = "finish"
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
	Event          Event
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
