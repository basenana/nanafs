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

package exec

import (
	"errors"
	"fmt"
	"github.com/basenana/go-flow/flow"
	"os"
	"path"
)

var (
	OperatorNotFound  = errors.New("operator not found")
	OperatorIsExisted = errors.New("operator is existed")
)

func initFlowWorkDir(base, flowID string) error {
	dir := flowWorkdir(base, flowID)
	s, err := os.Stat(dir)

	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		if s.IsDir() {
			return nil
		}
		return fmt.Errorf("init flow workdir failed: %s not dir", dir)
	}
	return os.MkdirAll(dir, 0755)
}

func cleanUpFlowWorkDir(base, flowID string) error {
	dir := flowWorkdir(base, flowID)
	s, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !s.IsDir() {
		return fmt.Errorf("flow workdir cleanup failed: %s not dir", dir)
	}

	return os.RemoveAll(dir)
}

func flowWorkdir(base, flowID string) string {
	return path.Join(base, "flows", flowID)
}

func setTaskResult(data *flow.ResultData, taskName, result string) {
	data.Result.Store(taskName, result)
}

func getTaskResult(data *flow.ResultData, taskName string) string {
	rawResult, ok := data.Result.Load(taskName)
	if ok {
		if result, isStr := rawResult.(string); isStr {
			return result
		}
	}
	return ""
}
