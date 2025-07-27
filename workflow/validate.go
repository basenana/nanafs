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

package workflow

import (
	"fmt"
	"github.com/basenana/nanafs/pkg/types"
	"regexp"
)

var (
	workflowIDPattern = "^[A-zA-Z][a-zA-Z0-9-_.]{5,31}$"
	wfIDRegexp        = regexp.MustCompile(workflowIDPattern)
)

func isValidID(idStr string) error {
	if wfIDRegexp.MatchString(idStr) {
		return nil
	}
	return fmt.Errorf("invalid ID [%s], pattern: %s", idStr, workflowIDPattern)
}

func validateWorkflowSpec(spec *types.Workflow) error {
	if spec.Id == "" {
		return fmt.Errorf("workflow id is empty")
	}
	if spec.Name == "" {
		return fmt.Errorf("workflow name is empty")
	}
	if spec.Namespace == "" {
		return fmt.Errorf("workflow namespace is empty")
	}
	return nil
}
