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

package events

import (
	"fmt"

	"github.com/hyponet/eventbus"
)

func PublishFridayEvent(namespace, session string, event any) {
	topic := fmt.Sprintf("friday.sessions.%s.events", session)
	eventbus.Publish(topic, event)
}

func SubscribeFridayEvents(namespace, session string) (chan any, func()) {
	result := make(chan any, 10)

	topic := fmt.Sprintf("friday.sessions.%s.events", session)
	sid := eventbus.Subscribe(topic, func(event any) {
		result <- event
	})

	return result, func() {
		eventbus.Unsubscribe(sid)
		close(result)
	}
}
