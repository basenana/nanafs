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

package notify

import (
	"context"
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/types"
	"sync/atomic"
)

var (
	globalSequence uint64
)

func registerEventHandle(n *Notify) {
	_, _ = events.Subscribe(events.NamespacedTopic(events.TopicNamespaceEntry, "*"), n.handleEvent)
	_, _ = events.Subscribe(events.NamespacedTopic(events.TopicNamespaceDocument, "*"), n.handleEvent)
}

func (n *Notify) GetLatestSequence(ctx context.Context) (int64, error) {
	evtList, err := n.store.ListEvents(ctx, types.EventFilter{Limit: 1})
	if err != nil {
		return 0, err
	}
	if len(evtList) == 0 {
		return 0, nil
	}
	return evtList[0].Sequence, nil
}

func (n *Notify) ListUnSyncedEvent(ctx context.Context, sequence int64) ([]types.Event, error) {
	evtList, err := n.store.ListEvents(ctx, types.EventFilter{StartSequence: sequence})
	if err != nil {
		return nil, err
	}
	return evtList, nil
}

func (n *Notify) CommitSyncedEvent(ctx context.Context, deviceID string, sequence int64) error {
	return n.store.DeviceSync(ctx, deviceID, sequence)
}

func (n *Notify) handleEvent(evt *types.Event) error {
	if evt == nil {
		return nil
	}
	evt.Sequence = evt.Time.UnixNano()*10 + int64(atomic.AddUint64(&globalSequence, 1)%10)
	return n.store.RecordEvents(context.Background(), []types.Event{*evt})
}
