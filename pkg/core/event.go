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

package core

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/types"
)

type entryEvent struct {
	entryID    int64
	topicNS    string
	actionType string
}

var eventQ = make(chan *entryEvent, 8)

func publicEntryActionEvent(topicNS, actionType string, entryID int64) {
	eventQ <- &entryEvent{entryID: entryID, topicNS: topicNS, actionType: actionType}
}

func (c *core) entryActionEventHandler() {
	c.logger.Debugw("start entryActionEventHandler")
	for evt := range eventQ {
		if evt.entryID == 0 {
			c.logger.Errorw("handle entry event error: entry id is empty", "entry", evt.entryID, "action", evt.actionType)
			continue
		}
		en, err := c.store.GetEntry(context.Background(), evt.entryID)
		if err != nil {
			c.logger.Errorw("encounter error when handle entry event", "entry", evt.entryID, "action", evt.actionType, "err", err)
			continue
		}
		events.Publish(events.NamespacedTopic(evt.topicNS, evt.actionType), BuildEntryEvent(evt.actionType, en))
	}
}

func BuildEntryEvent(actionType string, entry *types.Entry) *types.Event {
	return &types.Event{
		Id:              uuid.New().String(),
		Namespace:       entry.Namespace,
		Type:            actionType,
		Source:          "fsCore",
		SpecVersion:     "1.0",
		Time:            time.Now(),
		RefType:         "entry",
		RefID:           entry.ID,
		DataContentType: "application/json",
		Data:            types.NewEventDataFromEntry(entry),
	}
}
