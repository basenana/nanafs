package events

import (
	"time"

	"github.com/basenana/nanafs/pkg/types"
	"github.com/google/uuid"
)

func BuildEntryEvent(actionType string, source, uri string, entry *types.Entry) *types.Event {
	return &types.Event{
		Id:              uuid.New().String(),
		Namespace:       entry.Namespace,
		Type:            actionType,
		Source:          source,
		SpecVersion:     "1.0",
		Time:            time.Now(),
		RefType:         "entry",
		RefID:           entry.ID,
		DataContentType: "application/event-data",
		Data: types.EventData{
			ID:      entry.ID,
			Kind:    entry.Kind,
			IsGroup: entry.IsGroup,
			URI:     uri,
		},
	}
}

func BuildFileEvent(actionType string, source string, entry *types.Entry) *types.Event {
	return &types.Event{
		Id:              uuid.New().String(),
		Namespace:       entry.Namespace,
		Type:            actionType,
		Source:          source,
		SpecVersion:     "1.0",
		Time:            time.Now(),
		RefType:         "file",
		RefID:           entry.ID,
		DataContentType: "application/event-data",
		Data: types.EventData{
			ID:      entry.ID,
			Kind:    entry.Kind,
			IsGroup: entry.IsGroup,
		},
	}
}
