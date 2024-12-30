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

package db

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/basenana/nanafs/pkg/types"
)

type SystemInfo struct {
	FsID     string `gorm:"column:fs_id;primaryKey"`
	ChunkSeg int64  `gorm:"column:chunk_seg"`
}

func (i SystemInfo) TableName() string {
	return "system_info"
}

type SystemConfig struct {
	ID        int64     `gorm:"column:id;primaryKey"`
	Group     string    `gorm:"column:cfg_group;index:cfg_group"`
	Name      string    `gorm:"column:cfg_name;index:cfg_name"`
	Value     string    `gorm:"column:cfg_value"`
	Namespace string    `gorm:"column:namespace;index:cfg_namespace"`
	ChangedAt time.Time `gorm:"column:changed_at"`
}

func (i SystemConfig) TableName() string {
	return "system_config"
}

type AccessToken struct {
	TokenKey       string    `gorm:"column:token_key;primaryKey"`
	SecretToken    string    `gorm:"column:secret_token"`
	UID            int64     `gorm:"column:uid;index:tk_uid"`
	GID            int64     `gorm:"column:gid"`
	ClientCrt      string    `gorm:"column:client_crt"`
	ClientKey      string    `gorm:"column:client_key"`
	CertExpiration time.Time `gorm:"column:cert_expiration"`
	LastSeenAt     time.Time `gorm:"column:last_seen_at"`
	Namespace      string    `gorm:"column:namespace;index:tk_ns"`
}

func (o *AccessToken) TableName() string {
	return "access_token"
}

type Entry struct {
	ID         int64   `gorm:"column:id;primaryKey"`
	Name       string  `gorm:"column:name;index:obj_name"`
	Aliases    *string `gorm:"column:aliases"`
	ParentID   *int64  `gorm:"column:parent_id;index:parent_id"`
	RefID      *int64  `gorm:"column:ref_id;index:ref_id"`
	RefCount   *int    `gorm:"column:ref_count"`
	Kind       string  `gorm:"column:kind"`
	KindMap    *int64  `gorm:"column:kind_map"`
	IsGroup    bool    `gorm:"column:is_group;index:obj_isgrp"`
	Size       *int64  `gorm:"column:size"`
	Version    int64   `gorm:"column:version;index:obj_version"`
	Dev        int64   `gorm:"column:dev"`
	Owner      *int64  `gorm:"column:owner"`
	GroupOwner *int64  `gorm:"column:group_owner"`
	Permission *int64  `gorm:"column:permission"`
	Storage    string  `gorm:"column:storage"`
	Namespace  string  `gorm:"column:namespace;index:obj_ns"`
	CreatedAt  int64   `gorm:"column:created_at"`
	ChangedAt  int64   `gorm:"column:changed_at"`
	ModifiedAt int64   `gorm:"column:modified_at"`
	AccessAt   int64   `gorm:"column:access_at"`
}

func (o *Entry) TableName() string {
	return "object"
}

func (o *Entry) FromEntry(en *types.Metadata) *Entry {
	o.ID = en.ID
	o.Name = en.Name
	o.Aliases = &en.Aliases
	o.ParentID = &en.ParentID
	o.RefID = &en.RefID
	o.RefCount = &en.RefCount
	o.Kind = string(en.Kind)
	o.KindMap = &en.KindMap
	o.IsGroup = en.IsGroup
	o.Size = &en.Size
	o.Version = en.Version
	o.Dev = en.Dev
	o.Storage = en.Storage
	o.Namespace = en.Namespace
	o.CreatedAt = en.CreatedAt.UnixNano()
	o.ChangedAt = en.ChangedAt.UnixNano()
	o.ModifiedAt = en.ModifiedAt.UnixNano()
	o.AccessAt = en.AccessAt.UnixNano()
	o.Owner = &en.Access.UID
	o.GroupOwner = &en.Access.GID
	o.Permission = updateEntryPermission(en.Access)
	return o
}

func (o *Entry) ToEntry() *types.Metadata {
	result := &types.Metadata{
		ID:         o.ID,
		Name:       o.Name,
		Kind:       types.Kind(o.Kind),
		IsGroup:    o.IsGroup,
		Version:    o.Version,
		Dev:        o.Dev,
		Storage:    o.Storage,
		Namespace:  o.Namespace,
		CreatedAt:  time.Unix(0, o.CreatedAt),
		ChangedAt:  time.Unix(0, o.ChangedAt),
		ModifiedAt: time.Unix(0, o.ModifiedAt),
		AccessAt:   time.Unix(0, o.AccessAt),
		Access:     buildEntryAccess(o.Permission, o.Owner, o.GroupOwner),
	}
	if o.Aliases != nil {
		result.Aliases = *o.Aliases
	}
	if o.ParentID != nil {
		result.ParentID = *o.ParentID
	}
	if o.RefID != nil {
		result.RefID = *o.RefID
	}
	if o.RefCount != nil {
		result.RefCount = *o.RefCount
	}
	if o.KindMap != nil {
		result.KindMap = *o.KindMap
	}
	if o.Size != nil {
		result.Size = *o.Size
	}
	return result
}

type Label struct {
	ID        int64  `gorm:"column:id;autoIncrement"`
	RefID     int64  `gorm:"column:ref_id;index:label_refid"`
	RefType   string `gorm:"column:ref_type;index:label_reftype"`
	Namespace string `gorm:"column:namespace;index:label_ns"`
	Key       string `gorm:"column:key"`
	Value     string `gorm:"column:value"`
	SearchKey string `gorm:"column:search_key;index:label_search_key"`
}

func (o Label) TableName() string {
	return "label"
}

type EntryProperty struct {
	ID        int64  `gorm:"column:id;autoIncrement"`
	OID       int64  `gorm:"column:oid;index:prop_oid"`
	Name      string `gorm:"column:key;index:prop_name"`
	Namespace string `gorm:"column:namespace;index:prop_ns"`
	Value     string `gorm:"column:value"`
	Encoded   bool   `gorm:"column:encoded"`
}

func (o EntryProperty) TableName() string {
	return "object_property"
}

type EntryExtend struct {
	ID          int64  `gorm:"column:id;primaryKey"`
	Symlink     string `gorm:"column:symlink"`
	GroupFilter []byte `gorm:"column:group_filter"`
	PlugScope   []byte `gorm:"column:plug_scope"`
}

func (o *EntryExtend) TableName() string {
	return "object_extend"
}

func (o *EntryExtend) From(ed *types.ExtendData) *EntryExtend {
	if ed == nil {
		return o
	}
	o.Symlink = ed.Symlink
	if ed.GroupFilter != nil {
		o.GroupFilter, _ = json.Marshal(ed.GroupFilter)
	}
	if ed.PlugScope != nil {
		o.PlugScope, _ = json.Marshal(ed.PlugScope)
	}
	return o
}

func (o *EntryExtend) ToExtData() types.ExtendData {
	ext := types.ExtendData{
		Symlink:     o.Symlink,
		GroupFilter: nil,
		PlugScope:   nil,
	}
	if o.GroupFilter != nil {
		_ = json.Unmarshal(o.GroupFilter, &ext.GroupFilter)
	}
	if o.PlugScope != nil {
		_ = json.Unmarshal(o.PlugScope, &ext.PlugScope)
	}
	return ext
}

type EntryURI struct {
	OID       int64  `gorm:"column:oid;primaryKey"`
	Uri       string `gorm:"column:uri;index:obj_uri"`
	Namespace string `gorm:"column:namespace;index:obj_uri_ns"`
	Invalid   bool   `gorm:"column:invalid"`
}

func (o *EntryURI) TableName() string {
	return "object_uri"
}

func (o *EntryURI) ToEntryUri() *types.EntryUri {
	return &types.EntryUri{
		ID:        o.OID,
		Uri:       o.Uri,
		Namespace: o.Namespace,
		Invalid:   o.Invalid,
	}
}

func (o *EntryURI) FromEntryUri(entryUri *types.EntryUri) *EntryURI {
	o.OID = entryUri.ID
	o.Uri = entryUri.Uri
	o.Namespace = entryUri.Namespace
	o.Invalid = entryUri.Invalid
	return o
}

type EntryChunk struct {
	ID       int64 `gorm:"column:id;primaryKey"`
	OID      int64 `gorm:"column:oid;index:ck_oid"`
	ChunkID  int64 `gorm:"column:chunk_id;index:ck_id"`
	Off      int64 `gorm:"column:off"`
	Len      int64 `gorm:"column:len"`
	State    int16 `gorm:"column:state"`
	AppendAt int64 `gorm:"column:append_at;index:ck_append_at"`
}

func (o EntryChunk) TableName() string {
	return "object_chunk"
}

type ScheduledTask struct {
	ID             int64     `gorm:"column:id;autoIncrement"`
	TaskID         string    `gorm:"column:task_id;index:st_task_id"`
	Namespace      string    `gorm:"column:namespace;index:st_task_ns"`
	RefType        string    `gorm:"column:ref_type;index:sche_task_reftype"`
	RefID          int64     `gorm:"column:ref_id;index:sche_task_refid"`
	Status         string    `gorm:"column:status;index:st_task_status"`
	Result         string    `gorm:"column:result"`
	CreatedTime    time.Time `gorm:"column:created_time"`
	ExecutionTime  time.Time `gorm:"column:execution_time"`
	ExpirationTime time.Time `gorm:"column:expiration_time"`
	Event          string    `gorm:"column:event"`
}

func (d ScheduledTask) TableName() string {
	return "scheduled_task"
}

type Notification struct {
	ID        string    `gorm:"column:id;primaryKey"`
	Title     string    `gorm:"column:title"`
	Message   string    `gorm:"column:message"`
	Namespace string    `gorm:"column:namespace;index:notif_ns"`
	Type      string    `gorm:"column:type"`
	Source    string    `gorm:"column:source"`
	Action    string    `gorm:"column:action"`
	Status    string    `gorm:"column:status"`
	Time      time.Time `gorm:"column:time;index:notif_time"`
}

func (o *Notification) TableName() string {
	return "notification"
}

type Event struct {
	ID              string    `gorm:"column:id;primaryKey"`
	Type            string    `gorm:"column:type;index:evt_type"`
	Source          string    `gorm:"column:source"`
	SpecVersion     string    `gorm:"column:specversion"`
	RefID           int64     `gorm:"column:ref_id;index:evt_refid"`
	RefType         string    `gorm:"column:ref_type;index:evt_reftype"`
	DataContentType string    `gorm:"column:datacontenttype"`
	Data            string    `gorm:"column:data"`
	Sequence        int64     `gorm:"column:sequence;index:evt_seq"`
	Namespace       string    `gorm:"column:namespace;index:evt_ns"`
	Time            time.Time `gorm:"column:time;index:evt_time"`
}

func (o *Event) TableName() string {
	return "event"
}

func (o *Event) From(event types.Event) {
	o.ID = event.Id
	o.Type = event.Type
	o.Source = event.Source
	o.SpecVersion = event.SpecVersion
	o.RefID = event.RefID
	o.RefType = event.RefType
	o.DataContentType = event.DataContentType
	o.Data = event.Data.String()
	o.Sequence = event.Sequence
	o.Namespace = event.Data.Namespace
	o.Time = event.Time
}

func (o *Event) To() (event types.Event, err error) {
	event = types.Event{
		Id:              o.ID,
		Type:            o.Type,
		Source:          o.Source,
		SpecVersion:     o.SpecVersion,
		Time:            o.Time,
		Sequence:        o.Sequence,
		Namespace:       o.Namespace,
		RefID:           o.RefID,
		RefType:         o.RefType,
		DataContentType: o.DataContentType,
		Data:            types.EventData{},
	}
	err = json.Unmarshal([]byte(o.Data), &(event.Data))
	return
}

type RegisteredDevice struct {
	ID             string    `gorm:"column:id;primaryKey"`
	SyncedSequence int64     `gorm:"column:synced_sequence"`
	LastSeenAt     time.Time `gorm:"column:last_seen_at"`
	Namespace      string    `gorm:"column:namespace;index:device_ns"`
}

func (o *RegisteredDevice) TableName() string {
	return "registered_device"
}

type Workflow struct {
	ID              string    `gorm:"column:id;primaryKey"`
	Name            string    `gorm:"column:name"`
	Rule            string    `gorm:"column:rule"`
	Cron            string    `gorm:"column:cron"`
	Steps           string    `gorm:"column:steps"`
	Enable          bool      `gorm:"column:enable;index:wf_enable"`
	QueueName       string    `gorm:"column:queue_name;index:wf_q"`
	Namespace       string    `gorm:"column:namespace;index:wf_ns"`
	CreatedAt       time.Time `gorm:"column:created_at;index:wf_creat"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
	LastTriggeredAt time.Time `gorm:"column:last_triggered_at"`
}

func (o *Workflow) TableName() string {
	return "workflow"
}

func (o *Workflow) From(wf *types.Workflow) (*Workflow, error) {
	o.ID = wf.Id
	o.Name = wf.Name
	o.Namespace = wf.Namespace
	o.Enable = wf.Enable
	o.Cron = wf.Cron
	o.QueueName = wf.QueueName
	o.CreatedAt = wf.CreatedAt
	o.UpdatedAt = wf.UpdatedAt
	o.LastTriggeredAt = wf.LastTriggeredAt

	if wf.Rule != nil {
		rawRule, err := json.Marshal(wf.Rule)
		if err != nil {
			return o, fmt.Errorf("marshal workflow rule config failed: %s", err)
		}
		o.Rule = string(rawRule)
	}

	rawSteps, err := json.Marshal(wf.Steps)
	if err != nil {
		return o, fmt.Errorf("marshal workflow rule config failed: %s", err)
	}
	o.Steps = string(rawSteps)
	return o, nil
}

func (o *Workflow) To() (*types.Workflow, error) {
	result := &types.Workflow{
		Id:              o.ID,
		Name:            o.Name,
		Namespace:       o.Namespace,
		Enable:          o.Enable,
		Cron:            o.Cron,
		Steps:           []types.WorkflowStepSpec{},
		QueueName:       o.QueueName,
		CreatedAt:       o.CreatedAt,
		UpdatedAt:       o.UpdatedAt,
		LastTriggeredAt: o.LastTriggeredAt,
	}

	if o.Rule != "" {
		if err := json.Unmarshal([]byte(o.Rule), &result.Rule); err != nil {
			return nil, fmt.Errorf("load workflow rule config failed: %s", err)
		}
	}

	if err := json.Unmarshal([]byte(o.Steps), &result.Steps); err != nil {
		return nil, fmt.Errorf("load workflow steps config failed: %s", err)
	}
	return result, nil
}

type WorkflowJob struct {
	ID            string    `gorm:"column:id;autoIncrement"`
	Workflow      string    `gorm:"column:workflow;index:job_wf_id"`
	TriggerReason string    `gorm:"column:trigger_reason"`
	TargetEntry   int64     `gorm:"column:target_entry;index:job_tgt_en"`
	Target        string    `gorm:"column:target"`
	Steps         string    `gorm:"column:steps"`
	Status        string    `gorm:"column:status;index:job_status"`
	Message       string    `gorm:"column:message"`
	QueueName     string    `gorm:"column:queue_name;index:job_queue"`
	Executor      string    `gorm:"column:executor;index:job_executor"`
	Namespace     string    `gorm:"column:namespace;index:job_ns"`
	StartAt       time.Time `gorm:"column:start_at"`
	FinishAt      time.Time `gorm:"column:finish_at"`
	CreatedAt     time.Time `gorm:"column:created_at;index:job_created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (o *WorkflowJob) TableName() string {
	return "workflow_job"
}

func (o *WorkflowJob) From(job *types.WorkflowJob) (*WorkflowJob, error) {
	o.ID = job.Id
	o.Workflow = job.Workflow
	o.TriggerReason = job.TriggerReason
	o.Status = job.Status
	o.Message = job.Message
	o.QueueName = job.QueueName
	o.Executor = job.Executor
	o.StartAt = job.StartAt
	o.FinishAt = job.FinishAt
	o.CreatedAt = job.CreatedAt
	o.UpdatedAt = job.UpdatedAt
	o.Namespace = job.Namespace

	rawTarget, err := json.Marshal(job.Target)
	if err != nil {
		return o, fmt.Errorf("marshal workflow job target failed: %s", err)
	}
	o.Target = string(rawTarget)

	rawStep, err := json.Marshal(job.Steps)
	if err != nil {
		return o, fmt.Errorf("marshal workflow job steps failed: %s", err)
	}
	o.Steps = string(rawStep)

	return o, nil
}

func (o *WorkflowJob) To() (*types.WorkflowJob, error) {
	result := &types.WorkflowJob{
		Id:            o.ID,
		Workflow:      o.Workflow,
		TriggerReason: o.TriggerReason,
		Steps:         []types.WorkflowJobStep{},
		Status:        o.Status,
		Message:       o.Message,
		Executor:      o.Executor,
		QueueName:     o.QueueName,
		StartAt:       o.StartAt,
		FinishAt:      o.FinishAt,
		CreatedAt:     o.CreatedAt,
		UpdatedAt:     o.UpdatedAt,
		Namespace:     o.Namespace,
	}

	err := json.Unmarshal([]byte(o.Target), &result.Target)
	if err != nil {
		return nil, fmt.Errorf("load workflow job target failed: %s", err)
	}

	err = json.Unmarshal([]byte(o.Steps), &result.Steps)
	if err != nil {
		return nil, fmt.Errorf("load workflow job steps failed: %s", err)
	}

	return result, nil
}

type Room struct {
	ID        int64     `gorm:"column:id;primaryKey"`
	Namespace string    `gorm:"column:namespace;index:room_namespace"`
	Title     string    `gorm:"column:title"`
	EntryId   int64     `gorm:"column:entry_id;index:room_entry_id"`
	Prompt    string    `gorm:"column:prompt"`
	History   string    `gorm:"column:history"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (r *Room) TableName() string { return "room" }

func (r *Room) From(room *types.Room) (*Room, error) {
	res := &Room{
		ID:        room.ID,
		Namespace: room.Namespace,
		Title:     room.Title,
		EntryId:   room.EntryId,
		Prompt:    room.Prompt,
		CreatedAt: room.CreatedAt,
	}

	history, err := json.Marshal(room.History)
	if err != nil {
		return nil, err
	}
	res.History = string(history)
	return res, nil
}

func (r *Room) To() (room *types.Room, err error) {
	room = &types.Room{
		ID:        r.ID,
		Namespace: r.Namespace,
		Title:     r.Title,
		EntryId:   r.EntryId,
		Prompt:    r.Prompt,
		History:   []map[string]string{},
		CreatedAt: r.CreatedAt,
	}
	if r.History != "" {
		err = json.Unmarshal([]byte(r.History), &(room.History))
	}
	return
}

type RoomMessage struct {
	ID        int64     `gorm:"column:id;primaryKey"`
	Namespace string    `gorm:"column:namespace;index:message_namespace"`
	RoomID    int64     `gorm:"column:room_id;index:message_room_id"`
	Sender    string    `gorm:"column:sender"`
	Message   string    `gorm:"column:message"`
	SendAt    time.Time `gorm:"column:send_at"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (r *RoomMessage) TableName() string { return "room_message" }

func (r *RoomMessage) From(roomMessage *types.RoomMessage) *RoomMessage {
	return &RoomMessage{
		ID:        roomMessage.ID,
		Namespace: roomMessage.Namespace,
		RoomID:    roomMessage.RoomID,
		Sender:    roomMessage.Sender,
		Message:   roomMessage.Message,
		SendAt:    roomMessage.SendAt,
		CreatedAt: roomMessage.CreatedAt,
	}
}

func (r *RoomMessage) To() (roomMessage *types.RoomMessage) {
	roomMessage = &types.RoomMessage{
		ID:        r.ID,
		Namespace: r.Namespace,
		RoomID:    r.RoomID,
		Sender:    r.Sender,
		Message:   r.Message,
		SendAt:    r.SendAt,
		CreatedAt: r.CreatedAt,
	}
	return
}
