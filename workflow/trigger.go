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
	"sync"
	"time"

	"github.com/basenana/nanafs/pkg/cel"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/hyponet/eventbus"
	"go.uber.org/zap"
)

type triggerConfig struct {
	localFileWatch *types.WorkflowLocalFileWatch
	rss            *types.WorkflowRssTrigger
	interval       *int
	hash           string
}

type triggerUpdate struct {
	namespace string
	workflow  string
	interval  int
}

type triggers struct {
	mgr    Workflow
	core   core.Core
	meta   metastore.Meta
	logger *zap.SugaredLogger

	// for unit test
	timerTool timerTool

	// workflows: namespace -> workflowID -> trigger
	workflows map[string]map[string]*triggerConfig
	waiters   map[string]chan triggerUpdate
	mux       sync.Mutex
}

func initTriggers(mgr Workflow, c core.Core, m metastore.Meta) *triggers {
	return &triggers{
		mgr:       mgr,
		core:      c,
		meta:      m,
		logger:    logger.NewLogger("triggers"),
		timerTool: &realTimerTool{},
		workflows: make(map[string]map[string]*triggerConfig),
		waiters:   make(map[string]chan triggerUpdate),
	}
}

func (h *triggers) start(ctx context.Context) {
	workflows, err := h.meta.ListAllNamespaceWorkflows(ctx)
	if err != nil {
		h.logger.Errorw("list all workflow failed", "err", err)
		return
	}

	for i := range workflows {
		h.handleWorkflowUpdate(workflows[i], false)
	}

	eventbus.Subscribe(events.NamespacedTopic(events.TopicNamespaceEntry, events.ActionTypeCreate), h.handleEntryCreate)
}

// MARK: workflow trigger management

func (h *triggers) handleWorkflowUpdate(wf *types.Workflow, isDelete bool) {
	h.mux.Lock()
	defer h.mux.Unlock()

	nsTriggers, ok := h.workflows[wf.Namespace]
	if !ok {
		nsTriggers = make(map[string]*triggerConfig)
		h.workflows[wf.Namespace] = nsTriggers
	}

	if isDelete || !wf.Enable {
		delete(nsTriggers, wf.Id)
		return
	}

	triggerCfg := wf.Trigger
	if triggerCfg.LocalFileWatch == nil && triggerCfg.Interval == nil {
		delete(nsTriggers, wf.Id)
		return
	}

	newCfg := &triggerConfig{
		localFileWatch: triggerCfg.LocalFileWatch,
		rss:            triggerCfg.RSS,
		interval:       triggerCfg.Interval,
	}
	newCfgHash := utils.ComputeStructHash(newCfg, nil)

	if old, ok := nsTriggers[wf.Id]; ok && old.hash == newCfgHash {
		return
	}

	newCfg.hash = newCfgHash
	nsTriggers[wf.Id] = newCfg
	h.sendWorkflowUpdateInLock(triggerUpdate{namespace: wf.Namespace, workflow: wf.Id, interval: utils.Deref(triggerCfg.Interval, 0)})
}

func (h *triggers) sendWorkflowUpdateInLock(update triggerUpdate) {
	wk := fmt.Sprintf("%s/%s", update.namespace, update.workflow)
	updater, ok := h.waiters[wk]
	if !ok {
		if update.interval > 0 { // new waiter
			updater = make(chan triggerUpdate, 5)
			h.waiters[wk] = updater
			go h.runWorkflowTimer(update.namespace, update.workflow, updater)
		} else {
			return // ignore
		}
	}
	updater <- update
}

func (h *triggers) runWorkflowTimer(namespace, workflow string, updater chan triggerUpdate) {
	var (
		waitKey = fmt.Sprintf("%s/%s", namespace, workflow)
		timer   = h.timerTool.NewTimer(time.Hour)
	)

	defer func() {
		h.mux.Lock()
		defer h.mux.Unlock()
		delete(h.waiters, waitKey)
		close(updater)
		timer.Stop()
	}()

	h.logger.Infow("start workflow timer", "namespace", namespace, "workflow", workflow)

	for {
		select {
		case <-timer.C:
			tc := h.getTriggerConfig(namespace, workflow)
			if tc == nil || utils.Deref(tc.interval, 0) == 0 {
				// interval task has been deleted, exit
				return
			}
			switch {
			case tc.rss != nil:
				h.runRSSWorkflow(context.Background(), namespace, workflow, tc.rss)
			default:
				// no interval task, exit
				return
			}
		case update := <-updater:
			if update.interval == 0 {
				// delete interval task, exit
				return
			}
			h.timerTool.ResetTimer(update.interval, timer)
		}
	}
}

func (h *triggers) getTriggerConfig(namespace, workflow string) *triggerConfig {
	h.mux.Lock()
	defer h.mux.Unlock()

	nsTriggers, ok := h.workflows[namespace]
	if !ok {
		return nil
	}
	triggerCfg, ok := nsTriggers[workflow]
	if !ok {
		return nil
	}

	return triggerCfg
}

// MARK: entry trigger hook

func (h *triggers) handleEntryCreate(evt *types.Event) {
	h.logger.Infow("[handleEntryCreate] handle entry create event", "type", evt.RefType, "entry", evt.RefID)

	if evt.RefType != "entry" {
		h.logger.Errorw("unknown event type", "type", evt.RefType, "refID", evt.RefID)
		return
	}

	// only hold lock when accessing h.workflows
	h.mux.Lock()
	nsTriggers := h.workflows[evt.Namespace]
	h.mux.Unlock()

	if len(nsTriggers) == 0 {
		return
	}

	entryURI := evt.Data.URI
	if entryURI == "" {
		h.logger.Errorw("[handleEntryCreate] guss entry uri failed", "entry", evt.RefID, "err", "uri in event is empty")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	entry, err := h.core.GetEntry(ctx, evt.Namespace, evt.RefID)
	if err != nil {
		h.logger.Errorw("[handleEntryCreate] get entry failed", "entry", evt.RefID, "err", err)
		return
	}

	// if entry is a Group, skip localFileWatch matching
	if types.IsGroup(entry.Kind) {
		gp := &types.GroupProperties{}
		err = h.meta.GetEntryProperties(ctx, evt.Namespace, types.PropertyTypeGroupAttr, entry.ID, gp)
		if err != nil {
			h.logger.Errorw("[handleEntryCreate] get group attr failed", "entry", entry.ID, "err", err)
			return
		}

		// if it's an RSS Group, trigger RSS workflows immediately
		if gp.Source == "rss" {
			for wfID, trg := range nsTriggers {
				if trg.rss != nil {
					h.triggerRSSWorkflowForGroup(ctx, evt.Namespace, entryURI, entry, gp, wfID)
				}
			}
		}
		return
	}

	for wfID, trg := range nsTriggers {
		if trg.localFileWatch == nil {
			continue
		}
		if trg.localFileWatch.Event != evt.Type {
			continue
		}

		matched, err := cel.EntryMatch(ctx, entry, trg.localFileWatch)
		if err != nil {
			h.logger.Errorw("[handleEntryCreate] match entry failed", "workflow", wfID, "entry", entry.ID, "err", err)
			continue
		}
		if !matched {
			h.logger.Debugw("[handleEntryCreate] entry not match rule", "workflow", wfID, "entry", entry.ID)
			continue
		}

		h.triggerWorkflow(ctx, evt.Namespace, wfID, types.WorkflowTarget{Entries: []string{entryURI}}, JobAttr{Reason: "entry created"})
	}
}

// MARK: interval trigger

func (h *triggers) triggerRSSWorkflowForGroup(ctx context.Context, namespace string, enURI string, en *types.Entry, gp *types.GroupProperties, wfID string) {
	if gp.RSS == nil || gp.RSS.Feed == "" {
		h.logger.Warnw("empty rss group attr", "workflow", wfID, "entry", en.ID)
		return
	}

	params := map[string]string{
		"feed": gp.RSS.Feed,
	}
	h.triggerWorkflow(ctx, namespace, wfID, types.WorkflowTarget{Entries: []string{enURI}}, JobAttr{Parameters: params, Reason: "rss group created"})
}

func (h *triggers) runRSSWorkflow(ctx context.Context, namespace, workflowID string, rss *types.WorkflowRssTrigger) {
	it, err := h.meta.FilterEntries(ctx, namespace, types.Filter{CELPattern: `group.source == "rss"`})
	if err != nil {
		h.logger.Errorw("filter rss failed", "workflow", workflowID, "err", err)
		return
	}

	for it.HasNext() {
		en, err := it.Next()
		if err != nil {
			h.logger.Errorw("get next rss group failed", "workflow", workflowID, "entry", en.ID, "err", err)
			continue
		}

		gp := &types.GroupProperties{}
		err = h.meta.GetEntryProperties(ctx, namespace, types.PropertyTypeGroupAttr, en.ID, gp)
		if err != nil {
			h.logger.Errorw("get group attr failed", "workflow", workflowID, "entry", en.ID, "err", err)
			continue
		}

		uri, err := core.ProbableEntryPath(ctx, h.core, en)
		if err != nil {
			h.logger.Errorw("fetch rss group uri failed", "workflow", workflowID, "entry", en.ID, "err", err)
			continue
		}
		h.triggerRSSWorkflowForGroup(ctx, namespace, uri, en, gp, workflowID)
	}
}

// MARK: workflow trigger

func (h *triggers) triggerWorkflow(ctx context.Context, namespace, wfId string, tgt types.WorkflowTarget, attr JobAttr) {
	job, err := h.mgr.TriggerWorkflow(ctx, namespace, wfId, tgt, attr)
	if err != nil {
		h.logger.Errorw("trigger workflow failed", "workflow", wfId, "err", err)
		return
	}
	h.logger.Infow("trigger workflow finish", "workflow", wfId, "job", job.Id, "reason", attr.Reason)
}

type timerTool interface {
	NewTimer(d time.Duration) *time.Ticker
	ResetTimer(min int, t *time.Ticker)
}

type realTimerTool struct{}

func (r *realTimerTool) NewTimer(d time.Duration) *time.Ticker {
	return time.NewTicker(d)
}

func (r *realTimerTool) ResetTimer(min int, t *time.Ticker) {
	t.Reset(time.Duration(min) * time.Minute)
}
