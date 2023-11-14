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

package metastore

import (
	"context"
	"time"

	"github.com/basenana/nanafs/pkg/types"
)

type Meta interface {
	DEntry
	ChunkStore
	NotificationRecorder
	ScheduledTaskRecorder
	DocumentRecorder
}

type DEntry interface {
	SystemInfo(ctx context.Context) (*types.SystemInfo, error)

	GetEntry(ctx context.Context, id int64) (*types.Metadata, error)
	FindEntry(ctx context.Context, parentID int64, name string) (*types.Metadata, error)
	CreateEntry(ctx context.Context, parentID int64, newEntry *types.Metadata) error
	RemoveEntry(ctx context.Context, parentID, entryID int64) error
	DeleteRemovedEntry(ctx context.Context, entryID int64) error
	UpdateEntryMetadata(ctx context.Context, entry *types.Metadata) error

	SaveEntryUri(ctx context.Context, entryUri *types.EntryUri) error
	GetEntryUri(ctx context.Context, uri string) (*types.EntryUri, error)
	DeleteEntryUri(ctx context.Context, id int64) error

	ListEntryChildren(ctx context.Context, parentId int64) (EntryIterator, error)
	FilterEntries(ctx context.Context, filter types.Filter) (EntryIterator, error)

	Open(ctx context.Context, id int64, attr types.OpenAttr) (*types.Metadata, error)
	Flush(ctx context.Context, id int64, size int64) error
	MirrorEntry(ctx context.Context, newEntry *types.Metadata) error
	ChangeEntryParent(ctx context.Context, targetEntryId int64, newParentId int64, newName string, opt types.ChangeParentAttr) error

	GetEntryExtendData(ctx context.Context, id int64) (types.ExtendData, error)
	UpdateEntryExtendData(ctx context.Context, id int64, ed types.ExtendData) error
	GetEntryLabels(ctx context.Context, id int64) (types.Labels, error)
	UpdateEntryLabels(ctx context.Context, id int64, labels types.Labels) error
}

type ChunkStore interface {
	NextSegmentID(ctx context.Context) (int64, error)
	ListSegments(ctx context.Context, oid, chunkID int64, allChunk bool) ([]types.ChunkSeg, error)
	AppendSegments(ctx context.Context, seg types.ChunkSeg) (*types.Metadata, error)
	DeleteSegment(ctx context.Context, segID int64) error
}

type ScheduledTaskRecorder interface {
	ListTask(ctx context.Context, taskID string, filter types.ScheduledTaskFilter) ([]*types.ScheduledTask, error)
	SaveTask(ctx context.Context, task *types.ScheduledTask) error
	DeleteFinishedTask(ctx context.Context, aliveTime time.Duration) error

	GetWorkflow(ctx context.Context, wfID string) (*types.WorkflowSpec, error)
	ListWorkflow(ctx context.Context) ([]*types.WorkflowSpec, error)
	DeleteWorkflow(ctx context.Context, wfID string) error
	GetWorkflowJob(ctx context.Context, jobID string) (*types.WorkflowJob, error)
	ListWorkflowJob(ctx context.Context, filter types.JobFilter) ([]*types.WorkflowJob, error)
	SaveWorkflow(ctx context.Context, wf *types.WorkflowSpec) error
	SaveWorkflowJob(ctx context.Context, wf *types.WorkflowJob) error
	DeleteWorkflowJob(ctx context.Context, wfJobID ...string) error
}

type NotificationRecorder interface {
	ListNotifications(ctx context.Context) ([]types.Notification, error)
	RecordNotification(ctx context.Context, nid string, no types.Notification) error
	UpdateNotificationStatus(ctx context.Context, nid, status string) error
}

type DocumentRecorder interface {
	SaveDocument(ctx context.Context, doc *types.Document) error
	ListDocument(ctx context.Context) ([]*types.Document, error)
	GetDocument(ctx context.Context, id string) (*types.Document, error)
	FindDocument(ctx context.Context, uri string) (*types.Document, error)
	DeleteDocument(ctx context.Context, id string) error
}
