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

package controller

import (
	"bytes"
	"github.com/basenana/go-flow/flow"
	"github.com/basenana/go-flow/fsm"
)

type Errors []error

func (e Errors) Error() string {
	buf := bytes.Buffer{}
	for _, oneE := range e {
		buf.WriteString(oneE.Error())
		buf.WriteString(" ")
	}

	return buf.String()
}

func (e Errors) IsError() bool {
	return len(e) > 0
}

func NewErrors() Errors {
	return []error{}
}

func IsFinishedStatus(sts fsm.Status) bool {
	switch sts {
	case flow.SucceedStatus, flow.FailedStatus, flow.CanceledStatus:
		return true
	default:
		return false
	}
}
