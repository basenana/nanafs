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
	"strconv"
)

var (
	TopicEntryCreateFmt       = "action.entry.%s.create"
	TopicEntryUpdateFmt       = "action.entry.%s.update"
	TopicEntryDestroyFmt      = "action.entry.%s.destroy"
	TopicEntryMirrorFmt       = "action.entry.%s.mirror"
	TopicEntryChangeParentFmt = "action.entry.%s.change_parent"
	TopicFileTruncFmt         = "action.file.%s.trunc"
	TopicFileOpenFmt          = "action.file.%s.open"
	TopicFileCloseFmt         = "action.file.%s.close"
)

func EntryActionTopic(topicFmt string, enID int64) string {
	if enID == 0 {
		return fmt.Sprintf(topicFmt, "*")
	}
	return fmt.Sprintf(topicFmt, strconv.FormatInt(enID, 10))
}
