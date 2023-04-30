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
)

var (
	TopicAllActions     = "action.*.*"
	TopicEntryActionFmt = "action.entry.%s"
	TopicFileActionFmt  = "action.file.%s"

	ActionTypeCreate       = "create"
	ActionTypeUpdate       = "update"
	ActionTypeDestroy      = "destroy"
	ActionTypeMirror       = "mirror"
	ActionTypeChangeParent = "change_parent"
	ActionTypeTrunc        = "trunc"
	ActionTypeOpen         = "open"
	ActionTypeClose        = "close"
)

func EntryActionTopic(topicFmt string, actionType string) string {
	return fmt.Sprintf(topicFmt, actionType)
}
