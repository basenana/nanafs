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
	"errors"
	"fmt"
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/rule"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/basenana/nanafs/workflow/jobrun"
	"github.com/hyponet/eventbus"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"sync"
	"time"
)

var entryHandleDelay = time.Second * 30

type hooks struct {
	mgr      *manager
	cron     *cron.Cron
	allHooks map[string]*workflowHooks
	mux      sync.Mutex

	delayQ []*pendingEntry
	qMux   sync.Mutex

	logger *zap.SugaredLogger
}

func initHooks(mgr *manager) *hooks {
	h := &hooks{
		mgr: mgr,
		cron: cron.New([]cron.Option{
			cron.WithLocation(time.Local),
			cron.WithLogger(logger.NewCronLogger()),
		}...),
		allHooks: make(map[string]*workflowHooks),
		logger:   logger.NewLogger("workflowHooks"),
	}
	h.setupHooks()
	return h
}

func (h *hooks) setupHooks() {
	eventbus.Subscribe(events.NamespacedTopic(events.TopicNamespaceEntry, events.ActionTypeCreate), h.handleEntryCreate)
}

func (h *hooks) start(ctx context.Context) {
	go func() {
		// delay start
		time.Sleep(time.Minute)
		h.cron.Start()
	}()
	go func() {
		<-ctx.Done()
		<-h.cron.Stop().Done()
	}()

	workflows, err := h.mgr.meta.ListAllNamespaceWorkflows(ctx)
	if err != nil {
		h.logger.Errorw("list all workflow failed", "err", err)
		return
	}

	for i := range workflows {
		h.handleWorkflowUpdate(workflows[i])
	}

	go func() {
		rechecker := time.NewTicker(entryHandleDelay)
		defer rechecker.Stop()
		for {
			select {
			case <-rechecker.C:
				h.triggerDelayedWorkflowJob(false)
			case <-ctx.Done():
				h.triggerDelayedWorkflowJob(true)
				return
			}
		}
	}()
}

// MARK: workflow cache hook

func (h *hooks) handleWorkflowUpdate(wf *types.Workflow) {
	h.mux.Lock()
	nsHooks, ok := h.allHooks[wf.Namespace]
	if !ok {
		nsHooks = &workflowHooks{workflowOnBoard: make(map[string]*workflowHook)}
		h.allHooks[wf.Namespace] = nsHooks
	}
	h.mux.Unlock()

	nsHooks.mux.Lock()
	defer nsHooks.mux.Unlock()
	if !wf.Enable {
		delete(nsHooks.workflowOnBoard, wf.Id)
		return
	}
	if wf.Rule == nil {
		delete(nsHooks.workflowOnBoard, wf.Id)
		return
	}

	hook, ok := nsHooks.workflowOnBoard[wf.Id]
	if !ok {
		hook = &workflowHook{}
		nsHooks.workflowOnBoard[wf.Id] = hook
	}

	hook.rule = *wf.Rule

	if wf.Cron == "" {
		if hook.cronID != nil {
			h.cron.Remove(*hook.cronID)
			hook.cronID = nil
			hook.cron = nil
		}
		return
	}

	if hook.cron == nil || *(hook.cron) != wf.Cron {
		if hook.cronID != nil {
			h.cron.Remove(*hook.cronID)
		}
		hook.cron = utils.ToPtr(wf.Cron)
		cronID, err := h.cron.AddFunc(wf.Cron, h.newJobFunc(wf.Namespace, wf.Id))
		if err != nil {
			h.logger.Errorw("add cron job failed", "err", err)
			return
		}
		hook.cronID = &cronID
	}

}

func (h *hooks) handleWorkflowDelete(namespace, wfID string) {
	h.mux.Lock()
	nsHooks, ok := h.allHooks[namespace]
	h.mux.Unlock()

	if !ok {
		return
	}

	nsHooks.mux.Lock()
	hook := nsHooks.workflowOnBoard[wfID]
	delete(nsHooks.workflowOnBoard, wfID)
	nsHooks.mux.Unlock()
	if hook == nil {
		return
	}

	if hook.cronID != nil {
		h.cron.Remove(*hook.cronID)
	}
}

// MARK: entry trigger hook

func (h *hooks) handleEntryCreate(evt *types.Event) {
	h.logger.Infow("[handleEntryCreate] handle entry create event", "type", evt.RefType, "entry", evt.RefID)
	h.mux.Lock()
	nsHooks, ok := h.allHooks[evt.Namespace]
	h.mux.Unlock()

	ctx := context.Background()
	if !ok {
		if err := h.initSystemWorkflow(ctx, evt.Namespace); err != nil {
			h.logger.Errorw("[handleEntryCreate] init system workflow failed", "namespace", evt.Namespace, "err", err)
			return
		}
		h.mux.Lock()
		nsHooks = h.allHooks[evt.Namespace]
		h.mux.Unlock()
	}

	if nsHooks == nil {
		h.logger.Errorw("[handleEntryCreate] namespaced workflow hook not found", "namespace", evt.Namespace)
		return
	}

	en, err := h.mgr.core.GetEntry(ctx, evt.Namespace, evt.RefID)
	if err != nil {
		h.logger.Errorw("[handleEntryCreate] get entry failed", "entry", evt.RefID, "err", err)
		return
	}
	properties, err := h.mgr.meta.ListEntryProperties(ctx, en.Namespace, en.ID)
	if err != nil {
		h.logger.Errorw("[handleEntryCreate] get entry properties failed", "entry", evt.RefID, "err", err)
		return
	}
	labels, err := h.mgr.meta.GetEntryLabels(ctx, en.Namespace, en.ID)
	if err != nil {
		h.logger.Errorw("[handleEntryCreate] get entry labels failed", "entry", evt.RefID, "err", err)
		return
	}

	for wfID, hook := range nsHooks.workflowOnBoard {
		if !rule.Filter(&hook.rule, en, &properties, &labels) {
			continue
		}

		if en.IsGroup {
			if err = h.triggerGroupWorkflow(ctx, evt.Namespace, wfID, en.ID, "group created"); err != nil {
				h.logger.Errorw("[handleEntryCreate] trigger group workflow failed", "entry", evt.RefID, "err", err)
			}
			continue
		}

		h.workflowJobDelay(ctx, evt.Namespace, wfID, en, "entry created")
	}
	return
}

func (h *hooks) newJobFunc(namespace, wfID string) func() {
	// trigger new workflow job
	return func() {
		wf, err := h.mgr.GetWorkflow(context.Background(), namespace, wfID)
		if err != nil {
			h.logger.Errorw("query workflow failed", "workflow", wfID, "err", err)
			return
		}

		if err = h.filterAndRunCronWorkflow(context.Background(), wf); err != nil {
			h.logger.Errorw("filter and trigger workflow failed", "workflow", wfID, "err", err)
			return
		}
	}
}

func (h *hooks) filterAndRunCronWorkflow(ctx context.Context, wf *types.Workflow) error {
	var (
		entries []*types.Entry
		err     error
	)
	entries, err = rule.Q().Rule(*wf.Rule).Results(ctx)
	if err != nil {
		h.logger.Errorw("[filterAndRunCronWorkflow] query entries with wf rule failed", "workflow", wf.Id, "rule", wf.Rule, "err", err)
		return err
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

		_ = h.triggerGroupWorkflow(ctx, wf.Namespace, wf.Id, en.ID, "cronjob")
	}
	return nil
}

func (h *hooks) workflowJobDelay(ctx context.Context, namespace, wfId string, en *types.Entry, reason string) {
	h.qMux.Lock()
	h.delayQ = append(h.delayQ, &pendingEntry{
		namespace: namespace,
		workflow:  wfId,
		entryID:   en.ID,
		parentID:  en.ParentID,
		isGroup:   en.IsGroup,
		reason:    reason,
		addAt:     time.Now(),
	})
	h.qMux.Unlock()
	h.logger.Infow("delay handle entry", "namespace", namespace, "workflow", wfId, "entry", en.ID, "reason", reason)
}

func (h *hooks) triggerDelayedWorkflowJob(isAll bool) {
	if len(h.delayQ) == 0 {
		return
	}

	workflows := make(map[string][]*pendingEntry)
	h.qMux.Lock()
	for len(h.delayQ) > 0 {
		pe := h.delayQ[0]
		if !isAll && time.Since(pe.addAt) < entryHandleDelay {
			break
		}

		workflows[pe.workflow] = append(workflows[pe.workflow], pe)
		h.delayQ = h.delayQ[1:]
	}
	h.qMux.Unlock()

	if len(workflows) == 0 {
		return
	}

	ctx := context.Background()
	for wf, pendingEntries := range workflows {
		parents := make(map[int64][]*pendingEntry)

		for _, pe := range pendingEntries {
			parents[pe.parentID] = append(parents[pe.parentID], pe)
		}

		for parentID, entries := range parents {
			if len(entries) == 0 {
				continue
			}

			firstEn := entries[0]
			var entryIDs []int64
			for _, en := range entries {
				entryIDs = append(entryIDs, en.entryID)
			}
			if err := h.triggerEntriesWorkflow(ctx, firstEn.namespace, wf, entryIDs, parentID, firstEn.reason); err != nil {
				h.logger.Errorw("trigger entries workflow failed", "parentID", parentID, "workflow", wf, "err", err)
			}
		}
	}

}

func (h *hooks) triggerGroupWorkflow(ctx context.Context, namespace, wfId string, parentID int64, reason string) error {
	tgt := types.WorkflowTarget{ParentEntryID: parentID}
	return h.triggerWorkflow(ctx, namespace, wfId, tgt, reason)
}

func (h *hooks) triggerEntriesWorkflow(ctx context.Context, namespace, wfId string, entries []int64, parentID int64, reason string) error {
	if len(entries) == 0 {
		return nil
	}
	tgt := types.WorkflowTarget{
		Entries:       entries,
		ParentEntryID: parentID,
	}
	return h.triggerWorkflow(ctx, namespace, wfId, tgt, reason)
}

func (h *hooks) triggerWorkflow(ctx context.Context, namespace, wfId string, tgt types.WorkflowTarget, reason string) error {
	job, err := h.mgr.TriggerWorkflow(ctx, namespace, wfId, tgt, JobAttr{Reason: reason})
	if err != nil {
		h.logger.Errorw("trigger workflow failed", "workflow", wfId, "err", err)
		return err
	}
	h.logger.Infow("trigger workflow finish", "workflow", wfId, "job", job.Id)
	return nil
}

func (h *hooks) initSystemWorkflow(ctx context.Context, namespace string) error {
	var err error
	workflows := buildInNsWorkflows(namespace)
	for i := range workflows {
		wf := workflows[i]
		err = h.mgr.meta.SaveWorkflow(ctx, namespace, wf)
		if err != nil {
			if !errors.Is(err, types.ErrIsExist) {
				return err
			}
		}
		h.handleWorkflowUpdate(wf)
	}
	return nil
}

type workflowHooks struct {
	workflowOnBoard map[string]*workflowHook
	mux             sync.Mutex
}

type workflowHook struct {
	cronID *cron.EntryID
	cron   *string
	rule   types.Rule
}

func buildInNsWorkflows(namespace string) []*types.Workflow {
	return []*types.Workflow{
		{

			Id:        fmt.Sprintf("%s.dacload", namespace),
			Name:      "Document load",
			Namespace: namespace,
			Rule: &types.Rule{
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
			},
			Enable:    true,
			System:    true,
			QueueName: types.WorkflowQueueFile,
		},
		{
			Id:        fmt.Sprintf("%s.rss", namespace),
			Name:      "RSS Collect",
			Namespace: namespace,
			Rule: &types.Rule{
				Labels: &types.LabelMatch{
					Include: []types.Label{
						{Key: types.LabelKeyPluginKind, Value: string(types.TypeSource)},
						{Key: types.LabelKeyPluginName, Value: "rss"},
					},
				},
			},
			Cron: "*/30 * * * *",
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
			Enable:    true,
			System:    true,
			QueueName: types.WorkflowQueueFile,
		},
	}
}

type pendingEntry struct {
	namespace string
	workflow  string
	entryID   int64
	parentID  int64
	isGroup   bool
	reason    string
	addAt     time.Time
}
