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
	"github.com/basenana/nanafs/pkg/rule"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"sync"
	"time"
)

type cronRecord struct {
	workflowID string
	cronEntry  cron.EntryID
	cronDef    string
}

type CronHandler struct {
	*cron.Cron

	mgr      Manager
	registry map[string]cronRecord
	mux      sync.Mutex
	logger   *zap.SugaredLogger
}

func (c *CronHandler) Start(ctx context.Context) {
	c.Cron.Start()
	c.logger.Info("started")

	go func() {
		<-ctx.Done()
		c.logger.Info("stopping")
		sdCtx := c.Cron.Stop()
		<-sdCtx.Done()
		c.logger.Info("stopped")
	}()
}

func (c *CronHandler) Register(wf *types.WorkflowSpec) error {
	c.mux.Lock()
	defer c.mux.Unlock()
	oldRecord, ok := c.registry[wf.Id]
	if ok {
		if oldRecord.cronDef == wf.Cron {
			return nil
		}
		c.Remove(oldRecord.cronEntry)
	}

	if wf.Cron == "" {
		return nil
	}

	cronEnID, err := c.AddFunc(wf.Cron, c.newJobFunc(wf.Id))
	if err != nil {
		c.logger.Errorw("reconcile workflow cron registry failed", "workflow", wf.Id, "err", err)
		return err
	}

	c.registry[wf.Id] = cronRecord{workflowID: wf.Id, cronEntry: cronEnID, cronDef: wf.Cron}
	return nil
}

func (c *CronHandler) Unregister(wfID string) {
	c.mux.Lock()
	defer c.mux.Unlock()
	oldRecord, ok := c.registry[wfID]
	if !ok {
		return
	}
	c.Remove(oldRecord.cronEntry)
}

func (c *CronHandler) newJobFunc(wfID string) func() {
	// trigger new workflow job
	return func() {
		wf, err := c.mgr.GetWorkflow(context.Background(), wfID)
		if err != nil {
			c.logger.Errorw("query workflow failed", "workflow", wfID, "err", err)
			return
		}

		if err = c.filterAndTrigger(context.Background(), wf); err != nil {
			c.logger.Errorw("filter and trigger workflow failed", "workflow", wfID, "err", err)
			return
		}
	}
}

func (c *CronHandler) filterAndTrigger(ctx context.Context, wf *types.WorkflowSpec) error {
	var (
		entries []*types.Metadata
		err     error
	)
	entries, err = rule.Q().Rule(wf.Rule).Results(ctx)
	if err != nil {
		c.logger.Errorw("query entries with wf rule failed", "workflow", wf.Id, "rule", wf.Rule, "err", err)
		return err
	}
	c.logger.Infow("query entries with wf rule", "workflow", wf.Id, "entries", len(entries))

	for _, en := range entries {
		tgt := types.WorkflowTarget{}
		if types.IsGroup(en.Kind) {
			tgt.ParentEntryID = en.ID
		} else {
			tgt.EntryID = en.ID
		}
		job, err := c.mgr.TriggerWorkflow(ctx, wf.Id, tgt, JobAttr{Reason: "cronjob"})
		if err != nil {
			c.logger.Errorw("trigger workflow failed", "workflow", wf.Id, "entry", en.ID, "err", err)
			return err
		}
		c.logger.Infow("trigger workflow finish", "workflow", wf.Id, "job", job.Id, "entry", en.ID)
	}
	return nil
}

func newCronHandler(mgr Manager) *CronHandler {
	c := cron.New(
		cron.WithLocation(time.Local),
		cron.WithLogger(logger.NewCronLogger()),
	)

	return &CronHandler{
		Cron:   c,
		mgr:    mgr,
		logger: logger.NewLogger("cronHandler"),
	}
}
