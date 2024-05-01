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

package dentry

import (
	"context"
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/google/uuid"
	"time"
)

type entryEvent struct {
	entryID    int64
	topicNS    string
	actionType string
}

func (m *manager) publicEntryActionEvent(topicNS, actionType string, entryID int64) {
	m.eventQ <- &entryEvent{entryID: entryID, topicNS: topicNS, actionType: actionType}
}

func (m *manager) entryActionEventHandler() {
	m.logger.Debugw("start entryActionEventHandler")
	for evt := range m.eventQ {
		if evt.entryID == 0 {
			m.logger.Errorw("handle entry event error: entry id is empty", "entry", evt.entryID, "action", evt.actionType)
			continue
		}
		en, err := m.store.GetEntry(context.Background(), evt.entryID)
		if err != nil {
			m.logger.Errorw("encounter error when handle entry event", "entry", evt.entryID, "action", evt.actionType, "err", err)
			continue
		}
		// skip extgroup entries
		if en.Storage == externalStorage {
			continue
		}
		events.Publish(events.NamespacedTopic(evt.topicNS, evt.actionType), BuildEntryEvent(evt.actionType, en))
	}
}

func BuildEntryEvent(actionType string, entry *types.Metadata) *types.Event {
	return &types.Event{
		Id:              uuid.New().String(),
		Type:            actionType,
		Source:          "entryManager",
		SpecVersion:     "1.0",
		Time:            time.Now(),
		RefType:         "entry",
		RefID:           entry.ID,
		DataContentType: "application/json",
		Data:            types.NewEventDataFromEntry(entry),
	}
}
