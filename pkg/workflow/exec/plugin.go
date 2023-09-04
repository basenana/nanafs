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

package exec

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/plugin/stub"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/pkg/workflow/jobrun"
)

type pluginCallOperator struct {
	plugin    types.PlugScope
	entryID   int64
	entryPath string
}

var _ jobrun.Operator = &pluginCallOperator{}

func (e *pluginCallOperator) Do(ctx context.Context, param *jobrun.Parameter) error {
	req := stub.NewRequest()
	req.WorkPath = param.Workdir
	req.EntryId = e.entryID
	req.EntryPath = e.entryPath
	req.Action = e.plugin.Action
	req.Parameter = e.plugin.Parameters
	resp, err := plugin.Call(ctx, e.plugin, req)
	if err != nil {
		return fmt.Errorf("plugin action error: %s", err)
	}
	if !resp.IsSucceed {
		return fmt.Errorf("plugin action failed: %s", resp.Message)
	}
	return nil
}
