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
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/types"
	"reflect"
)

func registerBuildInWorkflow(ctx context.Context, mgr Manager) error {
	for i, bWf := range buildInWorflows {
		old, err := mgr.GetWorkflow(ctx, bWf.Id)
		if err != nil && err != types.ErrNotFound {
			return fmt.Errorf("query workflow %s failed: %s", bWf.Id, err)
		}

		if err = createOrUpdateBuildInWorkflow(ctx, mgr, buildInWorflows[i], old); err != nil {
			return fmt.Errorf("create or update workflow %s failed: %s", bWf.Id, err)
		}
	}
	return nil
}

func createOrUpdateBuildInWorkflow(ctx context.Context, mgr Manager, expect, old *types.WorkflowSpec) error {
	if old == nil {
		_, err := mgr.CreateWorkflow(ctx, expect)
		return err
	}

	if expect.Cron == old.Cron &&
		reflect.DeepEqual(expect.Rule, old.Rule) &&
		reflect.DeepEqual(expect.Steps, old.Steps) {
		return nil
	}

	old.Cron = expect.Cron
	old.Rule = expect.Rule
	old.Steps = expect.Steps
	_, err := mgr.UpdateWorkflow(ctx, old)
	return err
}

var (
	buildInWorflows = []*types.WorkflowSpec{
		{

			Id:   "buildin.rss",
			Name: "RSS Collect",
			Rule: types.Rule{
				Labels: &types.LabelMatch{
					Include: []types.Label{
						{Key: types.LabelKeyPluginKind, Value: string(types.TypeSource)},
						{Key: types.LabelKeyPluginName, Value: "rss"},
					},
				},
			},
			Cron: "*/15 * * * *",
			Steps: []types.WorkflowStepSpec{
				{
					Name: "collect",
					Plugin: &types.PlugScope{
						PluginName: "rss",
						Version:    "1.0",
						PluginType: types.TypeSource,
						Parameters: map[string]string{},
					},
				},
			},
			Enable: true,
		},
		{
			Id:   "buildin.docload",
			Name: "Document Load",
			Rule: types.Rule{
				Operation: types.RuleOpEndWith,
				Column:    "name",
				Value:     "html,htm,webarchive,pdf",
			},
			Steps: []types.WorkflowStepSpec{
				{
					Name: "docload",
					Plugin: &types.PlugScope{
						PluginName: "docloader",
						Version:    "1.0",
						PluginType: types.TypeProcess,
						Parameters: map[string]string{},
					},
				},
				{
					Name: "summary",
					Plugin: &types.PlugScope{
						PluginName: "summary",
						Version:    "1.0",
						PluginType: types.TypeProcess,
						Parameters: map[string]string{},
					},
				},
			},
			Enable: true,
		},
	}
)