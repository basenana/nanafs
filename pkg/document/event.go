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

package document

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/types"
)

func (m *manager) publicDocActionEvent(actionType string, doc *types.Document) {
	events.Publish(events.NamespacedTopic(events.TopicNamespaceDocument, actionType), buildDocumentEvent(actionType, doc))
}

func (m *manager) handleDocumentEvent(event *types.Event) error {
	if event.RefType != "document" {
		return nil
	}

	if m.indexer == nil {
		return nil
	}

	ctx, canF := context.WithTimeout(context.Background(), time.Minute*10)
	defer canF()

	nsCtx := types.WithNamespace(ctx, types.NewNamespace(event.Namespace))
	switch event.Type {
	case events.ActionTypeCreate, events.ActionTypeUpdate:
		doc, err := m.GetDocument(nsCtx, event.RefID)
		if err != nil {
			m.logger.Errorw("handle document event and query document failed", "document", event.RefID, "err", err)
			return err
		}

		if err = m.indexer.Index(nsCtx, doc); err != nil {
			m.logger.Errorw("handle update event and try index document failed", "document", event.RefID, "err", err)
			return err
		}
	case events.ActionTypeDestroy:
		if err := m.indexer.Delete(nsCtx, event.RefID); err != nil {
			m.logger.Errorw("handle destroy event and try cleanup index failed", "document", event.RefID, "err", err)
			return err
		}
	default:
		m.logger.Warnw("handle unknown action type document event", "action", event.Type)
		return nil
	}
	return nil
}

func buildDocumentEvent(actionType string, doc *types.Document) *types.Event {
	return &types.Event{
		Id:              uuid.New().String(),
		Namespace:       doc.Namespace,
		Type:            actionType,
		Source:          "documentManager",
		SpecVersion:     "1.0",
		Time:            time.Now(),
		RefType:         "document",
		RefID:           doc.ID,
		DataContentType: "application/json",
		Data:            types.NewEventDataFromDocument(doc),
	}
}
