package types

import (
	"encoding/json"
	"time"
)

type Event struct {
	Id              string            `json:"id"`
	Type            string            `json:"type"`
	Source          string            `json:"source"`
	SpecVersion     string            `json:"specversion"`
	DataContentType string            `json:"datacontenttype"`
	Data            string            `json:"data"`
	ExtraValue      map[string]string `json:"extra_value,omitempty"`
	Time            time.Time         `json:"time"`
}

func NewEvent(source, msg string) *Event {
	return &Event{
		Id:              NewID(),
		Type:            "event",
		Source:          source,
		SpecVersion:     "1.0",
		DataContentType: "text/plain",
		Data:            msg,
		Time:            time.Now(),
	}
}

func NewEventData(evtType, source string, obj any) *Event {
	data, _ := json.Marshal(obj)
	evt := &Event{
		Id:              NewID(),
		Type:            evtType,
		Source:          source,
		SpecVersion:     "1.0",
		DataContentType: "application/json",
		Data:            string(data),
		Time:            time.Now(),
	}
	return evt
}

type Delta struct {
	Content   string `json:"content,omitempty"`
	Reasoning string `json:"reasoning,omitempty"`
}
