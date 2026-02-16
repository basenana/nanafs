package session

import (
	"fmt"

	"github.com/basenana/friday/core/types"
	"github.com/hyponet/eventbus"
)

func SendEvent(sessionID string, eventList ...*types.Event) {
	for _, event := range eventList {
		eventbus.Publish(fmt.Sprintf("friday.sessions.%s.events", sessionID), event)
	}
}

func (s *Session) SubjectEvents() (chan *types.Event, func()) {
	result := make(chan *types.Event, 5)

	sid := eventbus.Subscribe(fmt.Sprintf("friday.sessions.%s.events", s.ID), func(msg *types.Event) {
		result <- msg
	})

	return result, func() {
		eventbus.Unsubscribe(sid)
		for {
			select {
			case _ = <-result:
				// discard
			default:
				close(result)
				return
			}
		}
	}
}
