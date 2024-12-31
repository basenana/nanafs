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
)

func registerEventHandle(n *Notify) {
	_, _ = events.Subscribe(events.NamespacedTopic(events.TopicNamespaceEntry, "*"), n.handleEvent)
}

func (n *Notify) handleEvent(evt *types.Event) error {
	if evt == nil {
		return nil
	}
	evt.Sequence = evt.Time.UnixNano()
	return n.store.RecordEvents(context.Background(), []types.Event{*evt})
}
