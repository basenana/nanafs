package bio

import (
	"fmt"
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/google/uuid"
	"time"
)

func maxOff(off1, off2 int64) int64 {
	if off1 > off2 {
		return off1
	}
	return off2
}

func minOff(off1, off2 int64) int64 {
	if off1 < off2 {
		return off1
	}
	return off2
}

func buildCompactEvent(md *types.Metadata) *types.Event {
	return &types.Event{
		Id:              uuid.New().String(),
		Type:            events.ActionTypeCompact,
		Source:          fmt.Sprintf("/object/%d", md.ID),
		SpecVersion:     "1.0",
		Time:            time.Now(),
		RefType:         "object",
		RefID:           md.ID,
		DataContentType: "application/json",
		Data:            types.EventData{Metadata: md},
	}
}
