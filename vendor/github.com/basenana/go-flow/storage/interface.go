/*
   Copyright 2022 Go-Flow Authors

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

package storage

import (
	"github.com/basenana/go-flow/flow"
)

type Interface interface {
	GetFlow(flowId flow.FID) (flow.Flow, error)
	GetFlowMeta(flowId flow.FID) (*FlowMeta, error)
	SaveFlow(flow flow.Flow) error
	DeleteFlow(flowId flow.FID) error
	SaveTask(flowId flow.FID, task flow.Task) error
	DeleteTask(flowId flow.FID, taskName flow.TName) error
}
