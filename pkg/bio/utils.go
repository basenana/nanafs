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

func buildCompactEvent(entry *types.Metadata) *types.Event {
	return &types.Event{
		Id:              uuid.New().String(),
		Type:            events.ActionTypeCompact,
		Source:          fmt.Sprintf("/object/%d", entry.ID),
		SpecVersion:     "1.0",
		Time:            time.Now(),
		RefType:         "object",
		RefID:           entry.ID,
		DataContentType: "application/json",
		Data:            types.EventData{Metadata: entry},
	}
}

func noneInvalidCache(eid int64) {}
