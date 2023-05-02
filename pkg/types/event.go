package types

import (
	"time"
)

type Event struct {
	Id              string      `json:"id"`
	Type            string      `json:"type"`
	Source          string      `json:"source"`
	SpecVersion     string      `json:"specversion"`
	Time            time.Time   `json:"time"`
	RefID           int64       `json:"nanafsrefid"`
	RefType         string      `json:"nanafsreftype"`
	DataContentType string      `json:"datacontenttype"`
	Data            interface{} `json:"data"`
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
}
