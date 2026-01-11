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

package jobrun

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/basenana/go-flow"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	pluginapi "github.com/basenana/plugin/api"
	plugintypes "github.com/basenana/plugin/types"
)

var (
	InitializingStatus = flow.InitializingStatus
	RunningStatus      = flow.RunningStatus
	PausingStatus      = flow.PausedStatus
	SucceedStatus      = flow.SucceedStatus
	FailedStatus       = flow.FailedStatus
	ErrorStatus        = flow.ErrorStatus
	PausedStatus       = flow.PausedStatus
	CanceledStatus     = flow.CanceledStatus
)

type Task struct {
	job  *types.WorkflowJob
	step *types.WorkflowJobNode
}

func (t *Task) GetName() string {
	return t.step.Name
}

func (t *Task) GetStatus() string {
	return t.step.Status
}

func (t *Task) SetStatus(s string) {
	t.step.Status = s
}

func (t *Task) GetMessage() string {
	return t.step.Message
}

func (t *Task) SetMessage(s string) {
	t.step.Message = s
}

func (t *Task) GetBranchNext() string {
	return t.step.BranchNext
}

func (t *Task) SetBranchNext(s string) {
	t.step.BranchNext = s
}

func newTask(job *types.WorkflowJob, step *types.WorkflowJobNode) flow.Task {
	return &Task{job: job, step: step}
}

var _ flow.Task = &Task{}

type JobID struct {
	namespace string
	id        string
}

func (j JobID) FlowID() string {
	return j.namespace + "." + j.id
}

func NewJobID(flowID string) (jid JobID) {
	parts := strings.Split(flowID, ".")
	jid.namespace = parts[0]
	if len(parts) > 1 {
		jid.id = parts[1]
	}
	return
}

type coordinator struct {
	next       map[string]string
	mux        sync.Mutex
	crt        string
	isFinished bool
}

func (c *coordinator) NewTask(task flow.Task) {
	t, ok := task.(*Task)
	if !ok {
		return
	}

	c.mux.Lock()
	defer c.mux.Unlock()

	if c.crt == "" {
		c.crt = t.GetName()
	}

	c.next[t.GetName()] = t.step.Next
}

func (c *coordinator) UpdateTask(task flow.Task) {
	t, ok := task.(*Task)
	if !ok {
		return
	}

	tname := t.GetName()
	c.mux.Lock()
	defer c.mux.Unlock()

	if task.GetStatus() != SucceedStatus {
		return
	}

	if t.GetBranchNext() != "" {
		c.crt = t.GetBranchNext()
		return
	}

	c.crt = c.next[tname]
}

func (c *coordinator) NextBatch(context.Context) ([]string, error) {
	c.mux.Lock()
	defer c.mux.Unlock()

	if c.crt == "" {
		c.isFinished = true
		return nil, nil
	}

	return []string{c.crt}, nil
}

func (c *coordinator) Finished() bool {
	return c.isFinished
}

func (c *coordinator) HandleFail(flow.Task, error) flow.FailOperation {
	c.isFinished = true
	return flow.FailAndInterrupt
}

var _ flow.Coordinator = &coordinator{}

func workflowJob2Flow(ctrl *Controller, job *types.WorkflowJob) *flow.Flow {
	fb := flow.NewFlowBuilder(JobID{namespace: job.Namespace, id: job.Id}.FlowID()).
		Coordinator(&coordinator{next: make(map[string]string)}).
		Executor(newExecutor(ctrl, job)).
		Observer(ctrl)

	for i := range job.Nodes {
		fb.Task(newTask(job, &job.Nodes[i]))
	}

	return fb.Finish()
}

type namespacedStore struct {
	store     metastore.Meta
	namespace string
}

func newPersistentStore(store metastore.Meta, namespace string) pluginapi.PersistentStore {
	return &namespacedStore{store: store, namespace: namespace}
}

func (p *namespacedStore) Load(ctx context.Context, source, group, key string, data any) error {
	source = strings.TrimSpace(source)
	group = strings.TrimSpace(group)
	return p.store.LoadJobData(ctx, p.namespace, source, group, p.keyHash(key), data)
}

func (p *namespacedStore) Save(ctx context.Context, source, group, key string, data any) error {
	source = strings.TrimSpace(source)
	group = strings.TrimSpace(group)
	return p.store.SaveJobData(ctx, p.namespace, source, group, p.keyHash(key), data)
}

func (p *namespacedStore) keyHash(key string) string {
	return utils.ComputeStructHash(key, nil)
}

type namespacedFS struct {
	core      core.Core
	store     metastore.Meta
	namespace string
}

func (n *namespacedFS) SaveEntry(ctx context.Context, parentURI, name string, properties plugintypes.Properties, reader io.ReadCloser) error {
	defer reader.Close()

	_, parent, err := n.core.GetEntryByPath(ctx, n.namespace, parentURI)
	if err != nil {
		return fmt.Errorf("get parent %s error %w", parentURI, err)
	}

	child, err := n.core.FindEntry(ctx, n.namespace, parent.ID, name)
	if err != nil && !errors.Is(err, types.ErrNotFound) {
		return fmt.Errorf("find %s error %w", name, err)
	}

	var entry *types.Entry
	if child == nil {
		attr := types.EntryAttr{
			Name: name,
			Kind: types.FileKind(name, types.RawKind),
		}

		entry, err = n.core.CreateEntry(ctx, n.namespace, parent.ID, attr)
		if err != nil {
			return fmt.Errorf("create %s error %w", name, err)
		}
		setUnread := true
		properties.Unread = &setUnread
	} else {
		entry, err = n.core.GetEntry(ctx, n.namespace, child.ChildID)
		if err != nil {
			return fmt.Errorf("get %s error %w", name, err)
		}
	}

	file, err := n.core.Open(ctx, n.namespace, entry.ID, types.OpenAttr{Write: true, Create: true})
	if err != nil {
		return err
	}
	defer file.Close(ctx)

	_, err = io.Copy(utils.NewWriterWithContextWriter(ctx, file), reader)
	if err != nil {
		return err
	}

	if err = file.Flush(ctx); err != nil {
		return err
	}

	return n.UpdateEntryDocumentProperties(ctx, entry, properties)
}

func (n *namespacedFS) UpdateEntry(ctx context.Context, entryURI string, properties plugintypes.Properties) error {
	_, en, err := n.core.GetEntryByPath(ctx, n.namespace, entryURI)
	if err != nil {
		return fmt.Errorf("get entry %s error %w", entryURI, err)
	}
	if en == nil {
		return fmt.Errorf("entry %s not found", entryURI)
	}

	return n.UpdateEntryDocumentProperties(ctx, en, properties)
}

func (n *namespacedFS) UpdateEntryDocumentProperties(ctx context.Context, en *types.Entry, properties plugintypes.Properties) error {
	var (
		current types.DocumentProperties
		err     error
	)
	if err = n.store.GetEntryProperties(ctx, n.namespace, types.PropertyTypeDocument, en.ID, &current); err != nil {
		return fmt.Errorf("get current properties error %w", err)
	}

	updated := toDocumentProperties(properties)
	if current.Title == "" && updated.Title != "" {
		current.Title = updated.Title
	}
	if updated.Author != "" {
		current.Author = updated.Author
	}
	if updated.Year != "" {
		current.Year = updated.Year
	}
	if updated.Source != "" {
		current.Source = updated.Source
	}
	if updated.Abstract != "" {
		current.Abstract = updated.Abstract
	}
	if updated.Notes != "" {
		current.Notes = updated.Notes
	}
	if len(updated.Keywords) > 0 {
		current.Keywords = updated.Keywords
	}
	if updated.URL != "" {
		current.URL = updated.URL
	}
	if updated.SiteName != "" {
		current.SiteName = updated.SiteName
	}
	if updated.SiteURL != "" {
		current.SiteURL = updated.SiteURL
	}
	if updated.HeaderImage != "" {
		current.HeaderImage = updated.HeaderImage
	}
	if updated.PublishAt != 0 {
		current.PublishAt = updated.PublishAt
	}

	if properties.Unread != nil {
		current.Unread = *properties.Unread
	}
	if properties.Marked != nil {
		current.Marked = *properties.Marked
	}

	return n.store.UpdateEntryProperties(ctx, n.namespace, types.PropertyTypeDocument, en.ID, current)
}

func newNamespacedFS(c core.Core, store metastore.Meta, namespace string) pluginapi.NanaFS {
	return &namespacedFS{core: c, store: store, namespace: namespace}
}

func toDocumentProperties(p plugintypes.Properties) types.DocumentProperties {
	return types.DocumentProperties{
		Title:       p.Title,
		Author:      p.Author,
		Year:        p.Year,
		Source:      p.Source,
		Abstract:    p.Abstract,
		Notes:       p.Notes,
		Keywords:    p.Keywords,
		URL:         p.URL,
		SiteName:    p.SiteName,
		SiteURL:     p.SiteURL,
		HeaderImage: p.HeaderImage,
		PublishAt:   p.PublishAt,
	}
}
