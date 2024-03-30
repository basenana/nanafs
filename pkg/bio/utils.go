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
		Source:          "bio",
		SpecVersion:     "1.0",
		Time:            time.Now(),
		RefType:         "entry",
		RefID:           entry.ID,
		DataContentType: "application/json",
		Data:            types.NewEventDataFromEntry(entry),
	}
}

func expectPreRead(readCnt int64) int32 {
	if readCnt < 2 {
		return 2
	}
	if readCnt > 5 {
		return 5
	}
	return int32(readCnt)
}
