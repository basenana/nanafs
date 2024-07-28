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
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/robfig/cron/v3"
	"time"
)

func init() {
	getCronOptions = func() []cron.Option {
		return []cron.Option{
			cron.WithSeconds(),
			cron.WithLocation(time.Local),
			cron.WithLogger(logger.NewCronLogger()),
		}
	}
}

var _ = Describe("testConfigEntrySourcePlugin", func() {
	var (
		entryID int64
		ctx     = context.TODO()
	)
	Context("create group entry", func() {
		It("create should be succeed", func() {
			root, err := entryMgr.Root(ctx)
			Expect(err).Should(BeNil())
			en, err := entryMgr.CreateEntry(ctx, root.ID, types.EntryAttr{
				Name: "test-rss-group",
				Kind: types.GroupKind,
			})
			Expect(err).Should(BeNil())
			entryID = en.ID
		})
		It("config source group should be succeed", func() {
			labels, err := entryMgr.GetEntryLabels(ctx, entryID)
			Expect(err).Should(BeNil())

			needAddLabels := map[string]string{
				types.LabelKeyPluginKind: string(types.TypeSource),
				types.LabelKeyPluginName: "rss",
			}
			for i, l := range labels.Labels {
				if val, ok := needAddLabels[l.Key]; ok {
					labels.Labels[i].Value = val
					delete(needAddLabels, l.Key)
				}
			}
			for k, v := range needAddLabels {
				labels.Labels = append(labels.Labels, types.Label{Key: k, Value: v})
			}

			err = entryMgr.UpdateEntryLabels(ctx, entryID, labels)
			Expect(err).Should(BeNil())
		})
	})

	Context("config source cron workflow", func() {
		var workflowID = "test_source_" + utils.RandStringRunes(5)
		It("create source workflow should be succeed", func() {
			_, err := mgr.CreateWorkflow(ctx, &types.Workflow{
				Id:   workflowID,
				Name: workflowID,
				Rule: types.Rule{Labels: &types.LabelMatch{
					Include: []types.Label{{Key: types.LabelKeyPluginKind, Value: string(types.TypeSource)}},
				}},
				Cron: "* * * * * *",
				Steps: []types.WorkflowStepSpec{
					{
						Name: "step-1",
						Plugin: &types.PlugScope{
							PluginName: "delay",
							Version:    "1.0",
							Action:     "delay",
							PluginType: types.TypeProcess,
							Parameters: map[string]string{"delay": "1s"},
						},
					},
				},
				QueueName: "default",
				Executor:  "local",
				Enable:    true,
			})
			Expect(err).Should(BeNil())
		})
		It("wait 3s", func() {
			time.Sleep(time.Second * 3)
		})
		It("check source entry has triggered", func() {
			jobs, err := mgr.ListJobs(ctx, workflowID)
			Expect(err).Should(BeNil())

			gotTargetJob := 0
			for _, j := range jobs {
				if j.Target.ParentEntryID == entryID {
					gotTargetJob += 1
				}
			}
			Expect(gotTargetJob > 0).Should(Equal(true))
		})
	})
})
