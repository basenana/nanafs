package dentry

import (
	"fmt"
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/google/uuid"
	"time"
)

func PublicEntryActionEvent(actionType string, en Entry) {
	events.Publish(events.EntryActionTopic(events.TopicEntryActionFmt, actionType), BuildEntryEvent(actionType, en))
}

func PublicFileActionEvent(actionType string, en Entry) {
	events.Publish(events.EntryActionTopic(events.TopicFileActionFmt, actionType), BuildEntryEvent(actionType, en))
}

func BuildEntryEvent(actionType string, en Entry) *types.Event {
	md := en.Metadata()
	return &types.Event{
		Id:              uuid.New().String(),
		Type:            actionType,
		Source:          fmt.Sprintf("/object/%d", md.ID),
		SpecVersion:     "1.0",
		Time:            time.Now(),
		RefType:         "object",
		RefID:           md.ID,
		DataContentType: "application/json",
		Data:            types.EventData{Metadata: md},
	}
}
