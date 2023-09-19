/*
   Copyright 2023 Go-Flow Authors

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

package jobrun

import (
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/pkg/workflow/fsm"
)

const (
	InitializingStatus = "initializing"
	RunningStatus      = "running"
	SucceedStatus      = "succeed"
	FailedStatus       = "failed"
	ErrorStatus        = "error"
	PausedStatus       = "paused"
	CanceledStatus     = "canceled"

	TriggerEvent       = "flow.execute.trigger"
	ExecuteFinishEvent = "flow.execute.finish"
	ExecuteFailedEvent = "flow.execute.failed"
	ExecuteErrorEvent  = "flow.execute.error"
	ExecutePauseEvent  = "flow.execute.pause"
	ExecuteResumeEvent = "flow.execute.resume"
	ExecuteCancelEvent = "flow.execute.cancel"
)

var _ fsm.Stateful = &types.WorkflowJob{}

func IsFinishedStatus(sts string) bool {
	switch sts {
	case SucceedStatus, FailedStatus, CanceledStatus, ErrorStatus:
		return true
	default:
		return false
	}
}
