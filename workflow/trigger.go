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
	"sync"
	"time"

	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/matcher"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/basenana/nanafs/workflow/jobrun"
	"github.com/hyponet/eventbus"
	"go.uber.org/zap"
)

type triggers struct {
	mgr    *manager
	jobQ   chan nextJob
	logger *zap.SugaredLogger

	circularQ [][]timedJobList
	cqIdx     int

	// todo: improve this
	matches map[string]*workflowMatches
	mux     sync.Mutex
}

func initTriggers(mgr *manager) *triggers {
	h := &triggers{
		mgr:    mgr,
		jobQ:   make(chan nextJob, 5),
		logger: logger.NewLogger("triggers"),

		circularQ: make([][]timedJobList, 288),
		matches:   make(map[string]*workflowMatches),
	}

	return h
}

func (h *triggers) start(ctx context.Context) {
	workflows, err := h.mgr.meta.ListAllNamespaceWorkflows(ctx)
	if err != nil {
		h.logger.Errorw("list all workflow failed", "err", err)
		return
	}

	for i := range workflows {
		h.handleWorkflowUpdate(workflows[i], false)
	}

	// matches hook
	eventbus.Subscribe(events.NamespacedTopic(events.TopicNamespaceEntry, events.ActionTypeCreate), h.handleEntryCreate)

	// interval
	go h.circularQueueRun(ctx)

	go func() {
		for job := range h.jobQ {
			h.triggerWorkflow(ctx, job.namespace, job.wfId, job.tgt, job.attr)
		}
	}()
}

// MARK: workflow cache hook

func (h *triggers) handleWorkflowUpdate(wf *types.Workflow, isDelete bool) {
	h.mux.Lock()
	nsHooks, ok := h.matches[wf.Namespace]
	if !ok {
		nsHooks = &workflowMatches{matches: make(map[string]*workflowMatch)}
		h.matches[wf.Namespace] = nsHooks
	}
	h.mux.Unlock()

	nsHooks.mux.Lock()
	defer nsHooks.mux.Unlock()
	if isDelete || !wf.Enable {
		delete(nsHooks.matches, wf.Id)
		return
	}

	trigger := wf.Trigger
	if trigger.OnCreate == nil || trigger.Interval == nil {
		delete(nsHooks.matches, wf.Id)
		return
	}

	hook, ok := nsHooks.matches[wf.Id]
	if !ok {
		hook = &workflowMatch{}
		nsHooks.matches[wf.Id] = hook
	}

	hook.onCreate = trigger.OnCreate
	hook.interval = trigger.Interval

	if hook.interval != nil {
		interval := *hook.interval
		idx := (h.cqIdx + interval/5) % len(h.circularQ)
		h.circularQ[idx] = append(h.circularQ[idx], timedJobList{namespace: wf.Namespace, workflow: wf.Id})
	}
}

// MARK: entry trigger hook

func (h *triggers) handleEntryCreate(evt *types.Event) {
	h.logger.Infow("[handleEntryCreate] handle entry create event", "type", evt.RefType, "entry", evt.RefID)
	h.mux.Lock()
	nsHooks, ok := h.matches[evt.Namespace]
	h.mux.Unlock()

	if !ok || len(nsHooks.matches) == 0 {
		return
	}

	if evt.RefType != "entry" || evt.Data.URI == "" {
		h.logger.Errorw("unknown event type", "type", evt.RefType, "refID", evt.RefID, "uri", evt.Data.URI)
		return
	}

	ctx := context.Background()
	entry, err := h.mgr.core.GetEntry(ctx, evt.Namespace, evt.RefID)
	if err != nil {
		h.logger.Errorw("[handleEntryCreate] get entry failed", "entry", evt.RefID, "err", err)
		return
	}

	nsHooks.mux.Lock()
	defer nsHooks.mux.Unlock()

	for wfID, hook := range nsHooks.matches {
		if hook.onCreate == nil {
			continue
		}

		matched, err := matcher.EntryMatch(ctx, entry, hook.onCreate)
		if err != nil {
			h.logger.Errorw("[handleEntryCreate] match entry failed", "workflow", wfID, "entry", entry.ID, "err", err)
			continue
		}
		if !matched {
			h.logger.Debugw("[handleEntryCreate] entry not match rule", "workflow", wfID, "entry", entry.ID)
			continue
		}

		if err = h.workflowJobDelay(ctx, evt.Namespace, wfID, evt.Data.URI, "entry created"); err != nil {
			h.logger.Errorw("[handleEntryCreate] trigger workflow failed", "namespace", evt.Namespace, "wfID", wfID, "entry", evt.RefID, "err", err)
			continue
		}
		h.logger.Infow("[handleEntryCreate] workflow triggered", "namespace", evt.Namespace, "wfID", wfID, "entry", entry.ID)
	}
	return
}

func (h *triggers) workflowJobDelay(ctx context.Context, namespace, wf, entryURI, reason string) error {
	h.jobQ <- nextJob{
		namespace: namespace,
		wfId:      wf,
		tgt:       types.WorkflowTarget{Entries: []string{entryURI}},
		attr:      JobAttr{Reason: reason},
	}
	return nil
}

func (h *triggers) filterAndRunCronWorkflow(ctx context.Context, wf *types.Workflow) error {
	if wf.Trigger.OnCreate == nil {
		return nil
	}

	celPattern := matcher.BuildCELFilterFromMatch(wf.Trigger.OnCreate)
	filter := types.Filter{CELPattern: celPattern}

	it, err := h.mgr.meta.FilterEntries(ctx, wf.Namespace, filter)
	if err != nil {
		h.logger.Errorw("[filterAndRunCronWorkflow] filter entries failed", "workflow", wf.Id, "err", err)
		return err
	}

	var entries []*types.Entry
	for it.HasNext() {
		en, err := it.Next()
		if err != nil {
			h.logger.Errorw("[filterAndRunCronWorkflow] get next entry failed", "workflow", wf.Id, "err", err)
			continue
		}
		entries = append(entries, en)
	}

	h.logger.Infow("[filterAndRunCronWorkflow] query entries with wf rule", "workflow", wf.Id, "entries", len(entries))

	for _, en := range entries {
		if en.Namespace != wf.Namespace {
			h.logger.Warnw("[filterAndRunCronWorkflow] match wrong namespace entry", "entry", en.ID, "entryNamespace", en.Namespace, "namespace", wf.Namespace)
			continue
		}

		if !en.IsGroup {
			h.logger.Warnw("[filterAndRunCronWorkflow] only group entry support cron run", "entry", en.ID, "namespace", en.Namespace)
			continue
		}

		matched, err := matcher.EntryMatch(ctx, en, wf.Trigger.OnCreate)
		if err != nil {
			h.logger.Errorw("[filterAndRunCronWorkflow] match entry failed", "workflow", wf.Id, "entry", en.ID, "err", err)
			continue
		}
		if !matched {
			h.logger.Debugw("[filterAndRunCronWorkflow] entry not match rule", "workflow", wf.Id, "entry", en.ID)
			continue
		}

		sameTargetJob, err := h.mgr.meta.ListWorkflowJobs(ctx, wf.Namespace, types.JobFilter{WorkFlowID: wf.Id, TargetEntry: en.ID})
		if err != nil {
			h.logger.Errorw("[filterAndRunCronWorkflow] query same target job failed", "entry", en.ID, "workflow", wf.Id, "err", err)
			continue
		}

		needSkip := false
		for _, j := range sameTargetJob {
			if j.Status == jobrun.InitializingStatus || j.Status == jobrun.RunningStatus {
				h.logger.Debugw("[filterAndRunCronWorkflow] found same target job, skip this event", "entry", en.ID, "job", j.Id)
				needSkip = true
				break
			}
		}
		if needSkip {
			continue
		}

		uri, err := core.ProbableEntryPath(ctx, h.mgr.core, en)
		if err != nil {
			h.logger.Errorw("[filterAndRunCronWorkflow] fetch entry uri failed", "workflow", wf.Id, "entry", en.ID, "err", err)
			continue
		}
		if err = h.workflowJobDelay(ctx, wf.Namespace, wf.Id, uri, "cron run"); err != nil {
			h.logger.Errorw("[filterAndRunCronWorkflow] trigger workflow failed", "namespace", wf.Namespace, "wfID", wf.Id, "entry", en.ID, "err", err)
			continue
		}
		h.logger.Infow("[filterAndRunCronWorkflow] workflow triggered", "namespace", wf.Namespace, "wfID", wf.Id, "entry", en.ID)
	}
	return nil
}

func (h *triggers) triggerWorkflow(ctx context.Context, namespace, wfId string, tgt types.WorkflowTarget, attr JobAttr) {
	job, err := h.mgr.TriggerWorkflow(ctx, namespace, wfId, tgt, attr)
	if err != nil {
		h.logger.Errorw("trigger workflow failed", "workflow", wfId, "err", err)
		return
	}
	h.logger.Infow("trigger workflow finish", "workflow", wfId, "job", job.Id)
}

func (h *triggers) circularQueueRun(ctx context.Context) {

	runWorkflow := func(namespace, workflow string) {
		h.mux.Lock()
		match := h.matches[namespace]
		h.mux.Unlock()

		match.mux.Lock()
		defer match.mux.Unlock()
		wf := match.matches[workflow]
		if wf == nil || wf.interval == nil {
			return
		}
		interval := *wf.interval
		h.usingSourcePlugin(ctx, namespace, workflow, interval)

		idx := (h.cqIdx + interval/5) % len(h.circularQ)
		h.circularQ[idx] = append(h.circularQ[idx], timedJobList{namespace: namespace, workflow: workflow})
	}

	t := time.NewTicker(5 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			todoList := h.circularQ[h.cqIdx]

			for _, td := range todoList {
				runWorkflow(td.namespace, td.workflow)
			}

			h.circularQ[h.cqIdx] = h.circularQ[h.cqIdx][:0]
			h.cqIdx += 1
			h.cqIdx %= len(h.circularQ)
		}
	}
}

func (h *triggers) usingSourcePlugin(ctx context.Context, namespace, workflow string, interval int) {
	wf, err := h.mgr.GetWorkflow(ctx, namespace, workflow)
	if err != nil {
		h.logger.Errorw("get workflow failed", "workflow", workflow, "err", err)
		return
	}

	var (
		trigger = wf.Trigger
		targets []types.WorkflowTarget
	)

	switch {
	case trigger.RSS != nil:
		it, err := h.mgr.meta.FilterEntries(ctx, namespace, types.Filter{CELPattern: `group.source == "rss"`})
		if err != nil {
			h.logger.Errorw("filter rss failed", "workflow", workflow, "err", err)
			return
		}

		for it.HasNext() {
			en, err := it.Next()
			if err != nil {
				h.logger.Errorw("get next rss group failed", "workflow", workflow, "err", err)
				continue
			}

			uri, err := core.ProbableEntryPath(ctx, h.mgr.core, en)
			if err != nil {
				h.logger.Errorw("fetch rss group uri failed", "workflow", workflow, "err", err)
				continue
			}
			targets = append(targets, types.WorkflowTarget{Entries: []string{uri}})
		}
	}

	for _, target := range targets {
		if len(target.Entries) > 0 {
			h.triggerWorkflow(ctx, namespace, workflow, target, JobAttr{
				Reason:     "interval run",
				Queue:      "",
				Parameters: nil,
				Timeout:    time.Duration(interval) * time.Minute,
			})
		}
	}

}

type workflowMatches struct {
	matches map[string]*workflowMatch
	mux     sync.Mutex
}

type workflowMatch struct {
	onCreate *types.WorkflowEntryMatch
	interval *int
}

type nextJob struct {
	namespace string
	wfId      string
	tgt       types.WorkflowTarget
	attr      JobAttr
}

type timedJobList struct {
	namespace string
	workflow  string
}
