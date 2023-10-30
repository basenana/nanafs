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
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"

	"github.com/basenana/nanafs/utils/logger"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/basenana/nanafs/pkg/types"
)

var initMetrics sync.Once

type Entity struct {
	*gorm.DB
}

func NewDbEntity(db *gorm.DB) (*Entity, error) {
	ent := &Entity{DB: db}

	ctx, canF := context.WithTimeout(context.TODO(), time.Second*10)
	defer canF()

	if err := Migrate(db); err != nil {
		return nil, err
	}

	_, err := ent.SystemInfo(ctx)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	if err != nil {
		sysInfo := &SystemInfo{FsID: uuid.New().String()}
		if res := db.WithContext(ctx).Create(sysInfo); res.Error != nil {
			return nil, res.Error
		}
	}

	initMetrics.Do(func() {
		l := logger.NewLogger("initDbMetrics")
		goDb, innerErr := db.DB()
		if innerErr == nil {
			registerErr := prometheus.Register(collectors.NewDBStatsCollector(goDb, db.Name()))
			if registerErr != nil {
				l.Warnf("register dbstats collector failed: %s", err)
			}
			return
		}
		l.Warnf("init db metrics failed: %s", err)
	})

	return ent, nil
}

func (e *Entity) GetObjectByID(ctx context.Context, oid int64) (*types.Object, error) {
	var (
		objMod = &Object{ID: oid}
	)

	if err := e.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.First(objMod)
		if res.Error != nil {
			return res.Error
		}
		return nil
	}); err != nil {
		return nil, err
	}

	obj := objMod.Object()

	return obj, nil
}

func (e *Entity) GetObjectExtendData(ctx context.Context, obj *types.Object) error {
	var (
		ext      = &ObjectExtend{}
		property = make([]ObjectProperty, 0)
	)
	if err := e.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("id = ?", obj.ID).First(ext)
		if res.Error != nil {
			return res.Error
		}
		res = tx.Where("oid = ?", obj.ID).Find(&property)
		if res.Error != nil {
			return res.Error
		}
		return nil
	}); err != nil {
		return err
	}

	ed := ext.ToExtData()
	for _, p := range property {
		ed.Properties.Fields[p.Name] = p.Value
	}

	obj.ExtendData = &ed
	return nil
}

func (e *Entity) GetLabels(ctx context.Context, refType string, refID int64) (*types.Labels, error) {
	ol := make([]Label, 0)
	res := e.WithContext(ctx).Where("ref_type = ? AND ref_id = ?", refType, refID).Find(&ol)
	if res.Error != nil {
		return nil, res.Error
	}

	label := &types.Labels{}
	for _, kv := range ol {
		label.Labels = append(label.Labels, types.Label{Key: kv.Key, Value: kv.Value})
	}
	return label, nil
}

func (e *Entity) ListObjectChildren(ctx context.Context, filter types.Filter) ([]*types.Object, error) {
	var scopeIdList []int64
	if len(filter.Label.Include) > 0 || len(filter.Label.Exclude) > 0 {
		var err error
		scopeIdList, err = listObjectIdsWithLabelMatcher(ctx, e.DB, filter.Label)
		if err != nil {
			return nil, err
		}
	}

	objectList := make([]Object, 0)
	var result []*types.Object
	res := queryFilter(e.DB.WithContext(ctx), filter, scopeIdList).Find(&objectList)
	if res.Error != nil {
		return nil, res.Error
	}

	result = make([]*types.Object, len(objectList))
	for i, objMod := range objectList {
		o := objMod.Object()
		result[i] = o
	}

	return result, nil
}

func (e *Entity) SaveObjects(ctx context.Context, objects ...*types.Object) error {
	return e.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i := range objects {
			obj := objects[i]
			if obj == nil {
				continue
			}
			if err := saveRawObject(tx, obj); err != nil {
				return err
			}
		}
		return nil
	})
}

func (e *Entity) SaveMirroredObject(ctx context.Context, srcObj, dstParent, object *types.Object) error {
	return e.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var (
			srcObjModel       = &Object{ID: srcObj.ID}
			dstParentObjModel = &Object{ID: dstParent.ID}
			nowTime           = time.Now()
		)

		res := tx.First(srcObjModel)
		if res.Error != nil {
			return res.Error
		}
		currentVersion := srcObjModel.Version

		srcObj.Version = currentVersion + 1
		srcObj.RefCount += 1
		srcObj.ChangedAt = nowTime
		srcObjModel.Update(srcObj)
		if srcObjModel.Version < 0 {
			srcObjModel.Version = 1024
		}
		res = tx.Where("version = ?", currentVersion).Updates(srcObjModel)
		if res.Error != nil || res.RowsAffected == 0 {
			if res.RowsAffected == 0 {
				return types.ErrConflict
			}
			return res.Error
		}

		res = tx.First(dstParentObjModel)
		if res.Error != nil {
			return res.Error
		}
		currentVersion = dstParentObjModel.Version
		dstParent.Version = currentVersion + 1
		dstParent.ChangedAt = nowTime
		dstParent.ModifiedAt = nowTime
		dstParentObjModel.Update(dstParent)
		if dstParentObjModel.Version < 0 {
			dstParentObjModel.Version = 1024
		}
		res = tx.Where("version = ?", currentVersion).Updates(dstParentObjModel)
		if res.Error != nil || res.RowsAffected == 0 {
			if res.RowsAffected == 0 {
				return types.ErrConflict
			}
			return res.Error
		}

		if err := saveRawObject(tx, object); err != nil {
			return err
		}
		return nil
	})
}

func (e *Entity) SaveChangeParentObject(ctx context.Context, srcParent, dstParent, obj *types.Object, opt types.ChangeParentOption) error {
	return e.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var (
			objModel          = &Object{ID: obj.ID}
			srcParentObjModel = &Object{ID: srcParent.ID}
			dstParentObjModel = &Object{ID: dstParent.ID}
			nowTime           = time.Now()
		)
		res := tx.First(objModel)
		if res.Error != nil {
			return res.Error
		}
		currentVersion := objModel.Version
		obj.Version = currentVersion + 1
		obj.Name = opt.Name
		objModel.Update(obj)
		if objModel.Version < 0 {
			objModel.Version = 1024
		}
		res = tx.Where("version = ?", currentVersion).Updates(objModel)
		if res.Error != nil || res.RowsAffected == 0 {
			if res.RowsAffected == 0 {
				return types.ErrConflict
			}
			return res.Error
		}

		res = tx.First(srcParentObjModel)
		if res.Error != nil {
			return res.Error
		}
		currentVersion = srcParentObjModel.Version
		srcParent.Version = currentVersion + 1

		if obj.IsGroup() {
			srcParent.RefCount -= 1
		}
		srcParent.ChangedAt = nowTime
		srcParent.ModifiedAt = nowTime
		srcParentObjModel.Update(srcParent)
		if srcParentObjModel.Version < 0 {
			srcParentObjModel.Version = 1024
		}
		res = tx.Where("version = ?", currentVersion).Updates(srcParentObjModel)
		if res.Error != nil || res.RowsAffected == 0 {
			if res.RowsAffected == 0 {
				return types.ErrConflict
			}
			return res.Error
		}

		if obj.IsGroup() {
			res = tx.First(dstParentObjModel)
			if res.Error != nil {
				return res.Error
			}
			currentVersion = dstParentObjModel.Version
			dstParent.Version = currentVersion + 1

			dstParent.ChangedAt = nowTime
			dstParent.ModifiedAt = nowTime
			dstParent.RefCount += 1
			dstParentObjModel.Update(dstParent)
			if dstParentObjModel.Version < 0 {
				dstParentObjModel.Version = 1024
			}
			res = tx.Where("version = ?", currentVersion).Updates(dstParentObjModel)
			if res.Error != nil || res.RowsAffected == 0 {
				if res.RowsAffected == 0 {
					return types.ErrConflict
				}
				return res.Error
			}
		}
		return nil
	})
}

func (e *Entity) DeleteObject(ctx context.Context, srcObj, obj *types.Object) error {
	if obj == nil {
		return fmt.Errorf("object is nil")
	}
	return e.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if srcObj != nil {
			if srcObj.RefCount == 0 {
				if err := deleteRawObject(tx, srcObj); err != nil {
					return err
				}
			} else {
				if err := saveRawObject(tx, srcObj); err != nil {
					return err
				}
			}
		}
		if err := deleteRawObject(tx, obj); err != nil {
			return err
		}
		return nil
	})
}

func (e *Entity) GetPluginRecord(ctx context.Context, plugin types.PlugScope, rid string, record interface{}) error {
	pd := PluginData{
		PluginName: plugin.PluginName,
		RecordId:   rid,
	}
	res := e.DB.WithContext(ctx).Where("plugin_name = ? AND record_id = ?", plugin.PluginName, rid).First(&pd)
	if res.Error != nil {
		return res.Error
	}
	return json.Unmarshal([]byte(pd.Content), record)
}

func (e *Entity) ListPluginRecords(ctx context.Context, plugin types.PlugScope, groupId string) ([]string, error) {
	result := make([]string, 0)
	res := e.DB.WithContext(ctx).Model(&PluginData{}).Select("record_id").Where("plugin_name = ? AND group_id = ?", plugin.PluginName, groupId).Find(&result)
	return result, res.Error
}

func (e *Entity) SavePluginRecord(ctx context.Context, plugin types.PlugScope, groupId, rid string, record interface{}) error {
	pd := PluginData{
		PluginName: plugin.PluginName,
		GroupId:    groupId,
		RecordId:   rid,
	}

	needCreate := false
	res := e.DB.WithContext(ctx).Where("plugin_name = ? AND record_id = ?", plugin.PluginName, rid).First(&pd)
	if res.Error != nil {
		if res.Error != gorm.ErrRecordNotFound {
			return res.Error
		}
		needCreate = true
	}

	rawContent, err := json.Marshal(record)
	if err != nil {
		return err
	}

	pd.Version = plugin.Version
	pd.Type = plugin.PluginType
	pd.Content = string(rawContent)

	if needCreate {
		res = e.DB.WithContext(ctx).Create(&pd)
		if res.Error != nil {
			return res.Error
		}
		return nil
	}

	res = e.DB.WithContext(ctx).Save(pd)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func (e *Entity) DeletePluginRecord(ctx context.Context, plugin types.PlugScope, rid string) error {
	pd := PluginData{
		PluginName: plugin.PluginName,
		RecordId:   rid,
	}

	res := e.DB.WithContext(ctx).Where("plugin_name = ? AND record_id = ?", plugin.PluginName, rid).First(&pd)
	if res.Error != nil {
		return res.Error
	}

	res = e.DB.WithContext(ctx).Delete(pd)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func (e *Entity) NextSegmentID(ctx context.Context) (nextID int64, err error) {
	err = e.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		nextID, err = availableChunkSegID(tx)
		return err
	})
	return
}

func (e *Entity) InsertChunkSegment(ctx context.Context, seg types.ChunkSeg) (*types.Object, error) {
	var (
		obj    *types.Object
		objMod = &Object{ID: seg.ObjectID}
		err    error
	)
	err = e.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(objMod)
		if res.Error != nil {
			return res.Error
		}
		obj = objMod.Object()

		res = tx.Create(&ObjectChunk{
			ID:       seg.ID,
			OID:      seg.ObjectID,
			ChunkID:  seg.ChunkID,
			Off:      seg.Off,
			Len:      seg.Len,
			State:    seg.State,
			AppendAt: time.Now().UnixNano(),
		})
		if res.Error != nil {
			return res.Error
		}
		if seg.Off+seg.Len > obj.Size {
			obj.Size = seg.Off + seg.Len
		}
		obj.ModifiedAt = time.Now()

		if writeBackErr := saveRawObject(tx, obj); writeBackErr != nil {
			return writeBackErr
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func (e *Entity) DeleteChunkSegment(ctx context.Context, segID int64) error {
	res := e.WithContext(ctx).Delete(&ObjectChunk{ID: segID})
	return res.Error
}

func (e *Entity) ListChunkSegments(ctx context.Context, oid, chunkID int64, allChunk bool) ([]types.ChunkSeg, error) {
	segments := make([]ObjectChunk, 0)
	if allChunk {
		res := e.DB.WithContext(ctx).Where("oid = ?", oid).Order("append_at").Find(&segments)
		if res.Error != nil {
			return nil, res.Error
		}
	} else {
		res := e.DB.WithContext(ctx).Where("oid = ? AND chunk_id = ?", oid, chunkID).Order("append_at").Find(&segments)
		if res.Error != nil {
			return nil, res.Error
		}
	}
	result := make([]types.ChunkSeg, len(segments))
	for i, seg := range segments {
		result[i] = types.ChunkSeg{
			ID:       seg.ID,
			ChunkID:  seg.ChunkID,
			ObjectID: seg.OID,
			Off:      seg.Off,
			Len:      seg.Len,
			State:    seg.State,
		}

	}
	return result, nil
}

func (e *Entity) ListScheduledTask(ctx context.Context, taskID string, filter types.ScheduledTaskFilter) ([]*types.ScheduledTask, error) {
	tasks := make([]ScheduledTask, 0)
	query := e.DB.WithContext(ctx).Where("task_id = ?", taskID)

	if filter.RefID != 0 && filter.RefType != "" {
		query = query.Where("ref_type = ? AND ref_id = ?", filter.RefType, filter.RefID)
	}

	if len(filter.Status) > 0 {
		query = query.Where("status IN ?", filter.Status)
	}

	res := query.Order("created_time").Find(&tasks)
	if res.Error != nil {
		return nil, res.Error
	}

	result := make([]*types.ScheduledTask, len(tasks))
	for i, t := range tasks {
		evt := types.EntryEvent{}
		_ = json.Unmarshal([]byte(t.Event), &evt)
		result[i] = &types.ScheduledTask{
			ID:             t.ID,
			TaskID:         t.TaskID,
			Status:         t.Status,
			RefType:        t.RefType,
			RefID:          t.RefID,
			Result:         t.Result,
			CreatedTime:    t.CreatedTime,
			ExecutionTime:  t.ExecutionTime,
			ExpirationTime: t.ExpirationTime,
			Event:          evt,
		}
	}
	return result, nil
}

func (e *Entity) SaveScheduledTask(ctx context.Context, task *types.ScheduledTask) error {
	t := &ScheduledTask{
		ID:             task.ID,
		TaskID:         task.TaskID,
		Status:         task.Status,
		RefID:          task.RefID,
		RefType:        task.RefType,
		Result:         task.Result,
		CreatedTime:    task.CreatedTime,
		ExecutionTime:  task.ExecutionTime,
		ExpirationTime: task.ExpirationTime,
	}

	rawEvt, err := json.Marshal(task.Event)
	if err != nil {
		return err
	}
	t.Event = string(rawEvt)
	if task.ID == 0 {
		res := e.DB.WithContext(ctx).Create(t)
		if res.Error != nil {
			return err
		}
		task.ID = t.ID
		return nil
	}
	res := e.DB.WithContext(ctx).Save(t)
	if res.Error != nil {
		return err
	}
	return nil
}

func (e *Entity) DeleteFinishedScheduledTask(ctx context.Context, aliveTime time.Duration) error {
	res := e.DB.WithContext(ctx).Where("status = ? AND created_time < ?", types.ScheduledTaskFinish, time.Now().Add(-1*aliveTime)).Delete(&ScheduledTask{})
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func (e *Entity) GetWorkflow(ctx context.Context, wfID string) (*types.WorkflowSpec, error) {
	wf := &Workflow{}
	res := e.DB.WithContext(ctx).Where("id = ?", wfID).First(wf)
	if res.Error != nil {
		return nil, res.Error
	}
	return wf.ToWorkflowSpec()
}

func (e *Entity) ListWorkflow(ctx context.Context) ([]*types.WorkflowSpec, error) {
	wfList := make([]Workflow, 0)
	res := e.DB.WithContext(ctx).Order("created_at desc").Find(&wfList)
	if res.Error != nil {
		return nil, res.Error
	}

	var (
		result = make([]*types.WorkflowSpec, len(wfList))
		err    error
	)
	for i, wf := range wfList {
		result[i], err = wf.ToWorkflowSpec()
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (e *Entity) SaveWorkflow(ctx context.Context, wf *types.WorkflowSpec) error {
	return e.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		dbMod := &Workflow{}
		res := tx.Where("id = ?", wf.Id).First(dbMod)
		if res.Error == gorm.ErrRecordNotFound {
			if err := dbMod.Update(wf); err != nil {
				return err
			}
			res = tx.Create(dbMod)
			return res.Error
		}
		if err := dbMod.Update(wf); err != nil {
			return err
		}
		res = tx.Save(dbMod)
		return res.Error
	})
}

func (e *Entity) DeleteWorkflow(ctx context.Context, wfID string) error {
	return e.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("id = ?", wfID).First(&Workflow{})
		if res.Error != nil {
			return res.Error
		}
		res = tx.Where("id = ?", wfID).Delete(&Workflow{})
		return res.Error
	})
}

func (e *Entity) ListWorkflowJob(ctx context.Context, filter types.JobFilter) ([]*types.WorkflowJob, error) {
	jobList := make([]WorkflowJob, 0)
	query := e.DB.WithContext(ctx)

	if filter.JobID != "" {
		query = query.Where("id = ?", filter.JobID)
	}
	if filter.WorkFlowID != "" {
		query = query.Where("workflow = ?", filter.WorkFlowID)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.TargetEntry != 0 {
		query = query.Where("target_entry = ?", filter.TargetEntry)
	}

	res := query.Order("created_at desc").Find(&jobList)
	if res.Error != nil {
		return nil, res.Error
	}

	var (
		result = make([]*types.WorkflowJob, len(jobList))
		err    error
	)
	for i, wf := range jobList {
		result[i], err = wf.ToWorkflowJobSpec()
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (e *Entity) SaveWorkflowJob(ctx context.Context, job *types.WorkflowJob) error {
	return e.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		dbMod := &WorkflowJob{}
		res := tx.Where("id = ?", job.Id).First(dbMod)
		if res.Error == gorm.ErrRecordNotFound {
			if err := dbMod.Update(job); err != nil {
				return err
			}
			res = tx.Create(dbMod)
			return res.Error
		}
		if err := dbMod.Update(job); err != nil {
			return err
		}
		res = tx.Save(dbMod)
		return res.Error
	})
}

func (e *Entity) DeleteWorkflowJob(ctx context.Context, wfJobID ...string) error {
	res := e.DB.WithContext(ctx).Where("id IN ?", wfJobID).Delete(&WorkflowJob{})
	return res.Error
}

func (e *Entity) ListNotifications(ctx context.Context) ([]types.Notification, error) {
	noList := make([]Notification, 0)
	res := e.DB.WithContext(ctx).Order("time desc").Find(&noList)
	if res.Error != nil {
		return nil, res.Error
	}

	result := make([]types.Notification, len(noList))
	for i, no := range noList {
		result[i] = types.Notification{
			ID:      no.ID,
			Title:   no.Title,
			Message: no.Message,
			Type:    no.Type,
			Source:  no.Source,
			Action:  no.Action,
			Status:  no.Status,
			Time:    no.Time,
		}
	}
	return result, nil
}

func (e *Entity) RecordNotification(ctx context.Context, nid string, no types.Notification) error {
	model := Notification{
		ID:      no.ID,
		Title:   no.Title,
		Message: no.Message,
		Type:    no.Type,
		Source:  no.Source,
		Action:  no.Action,
		Status:  no.Status,
		Time:    no.Time,
	}
	res := e.DB.WithContext(ctx).Create(model)
	return res.Error
}

func (e *Entity) UpdateNotificationStatus(ctx context.Context, nid, status string) error {
	res := e.DB.WithContext(ctx).Model(&Notification{}).Where("id = ?", nid).UpdateColumn("status", status)
	return res.Error
}

func (e *Entity) SaveDocument(ctx context.Context, doc *types.Document) error {
	return e.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		dbDoc := &Document{}
		res := tx.Where("id = ?", doc.ID).First(dbDoc)
		if res.Error == gorm.ErrRecordNotFound {
			if err := dbDoc.Update(doc); err != nil {
				return err
			}
			res = tx.Create(dbDoc)
			return res.Error
		}
		if err := dbDoc.Update(doc); err != nil {
			return err
		}
		res = tx.Save(dbDoc)
		return res.Error
	})
}

func (e *Entity) ListDocument(ctx context.Context) ([]*types.Document, error) {
	docList := make([]Document, 0)
	res := e.DB.WithContext(ctx).Order("time desc").Find(&docList)
	if res.Error != nil {
		return nil, res.Error
	}

	result := make([]*types.Document, len(docList))
	for i, doc := range docList {
		keyWords := strings.Split(doc.KeyWords, ",")
		result[i] = &types.Document{
			ID:        doc.ID,
			Name:      doc.Name,
			Uri:       doc.Uri,
			Source:    doc.Source,
			KeyWords:  keyWords,
			Content:   doc.Content,
			Summary:   doc.Summary,
			CreatedAt: doc.CreatedAt,
			ChangedAt: doc.ChangedAt,
		}
	}
	return result, nil
}

func (e *Entity) GetDocument(ctx context.Context, id string) (*types.Document, error) {
	doc := &Document{}
	res := e.DB.WithContext(ctx).Where("id = ?", id).First(doc)
	if res.Error != nil {
		return nil, res.Error
	}
	return doc.ToDocument()
}

func (e *Entity) DeleteDocument(ctx context.Context, id string) error {
	return e.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("id = ?", id).First(&Document{})
		if res.Error != nil {
			return res.Error
		}
		res = tx.Where("id = ?", id).Delete(&Document{})
		return res.Error
	})
}

func (e *Entity) SystemInfo(ctx context.Context) (*types.SystemInfo, error) {
	info := &SystemInfo{}
	res := e.WithContext(ctx).First(info)
	if res.Error != nil {
		return nil, res.Error
	}
	result := &types.SystemInfo{
		FilesystemID:  info.FsID,
		MaxSegmentID:  info.ChunkSeg,
		ObjectCount:   0,
		FileSizeTotal: 0,
	}

	res = e.DB.WithContext(ctx).Model(&Object{}).Count(&result.ObjectCount)
	if res.Error != nil {
		return nil, res.Error
	}

	if result.ObjectCount == 0 {
		return result, nil
	}

	res = e.DB.WithContext(ctx).Model(&Object{}).Select("SUM(size) as file_size_total").Scan(&result.FileSizeTotal)
	if res.Error != nil {
		return nil, res.Error
	}
	return result, nil
}

func saveRawObject(tx *gorm.DB, obj *types.Object) error {
	var (
		objModel = &Object{ID: obj.ID}
		extModel = &ObjectExtend{ID: obj.ID}
	)
	res := tx.First(objModel)
	if res.Error != nil && res.Error != gorm.ErrRecordNotFound {
		return res.Error
	}

	if res.Error == gorm.ErrRecordNotFound {
		objModel.Update(obj)
		res = tx.Create(objModel)
		if res.Error != nil {
			return res.Error
		}

		extModel.Update(obj)
		res = tx.Create(extModel)
		if res.Error != nil {
			return res.Error
		}
		if obj.Labels != nil {
			if err := updateObjectLabels(tx, obj); err != nil {
				return err
			}
		}
		return nil
	}

	currentVersion := obj.Version
	obj.Version += 1
	if obj.Version < 0 {
		obj.Version = 1024
	}
	objModel.Update(obj)
	res = tx.Where("version = ?", currentVersion).Updates(objModel)
	if res.Error != nil || res.RowsAffected == 0 {
		if res.RowsAffected == 0 {
			return types.ErrConflict
		}
		return res.Error
	}

	if obj.ExtendDataChanged {
		if err := updateObjectExtendData(tx, obj); err != nil {
			return err
		}
	}
	if obj.LabelsChanged {
		if err := updateObjectLabels(tx, obj); err != nil {
			return err
		}
	}
	return nil
}

func deleteRawObject(tx *gorm.DB, obj *types.Object) error {
	var (
		objModel = &Object{ID: obj.ID}
		extModel = &ObjectExtend{ID: obj.ID}
	)
	res := tx.First(objModel)
	if res.Error != nil && res.Error != gorm.ErrRecordNotFound {
		return res.Error
	}

	res = tx.Delete(objModel)
	if res.Error != nil {
		return res.Error
	}
	res = tx.Delete(extModel)
	if res.Error != nil {
		return res.Error
	}
	res = tx.Where("ref_type = 'object' AND ref_id = ?", obj.ID).Delete(&Label{})
	if res.Error != nil {
		return res.Error
	}

	return nil
}

func availableChunkSegID(tx *gorm.DB) (int64, error) {
	info := &SystemInfo{}
	res := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(info)
	if res.Error != nil {
		return 0, res.Error
	}

	info.ChunkSeg += 1
	res = tx.Save(info)
	if res.Error != nil {
		return 0, res.Error
	}

	return info.ChunkSeg, nil
}

func updateObjectExtendData(tx *gorm.DB, obj *types.Object) error {
	var (
		extModel                   = &ObjectExtend{ID: obj.ID}
		objectProperties           = make([]ObjectProperty, 0)
		needCreatePropertiesModels = make([]ObjectProperty, 0)
		needUpdatePropertiesModels = make([]ObjectProperty, 0)
		needDeletePropertiesModels = make([]ObjectProperty, 0)
	)

	extModel.Update(obj)
	res := tx.Save(extModel)
	if res.Error != nil {
		return res.Error
	}

	res = tx.Where("oid = ?", obj.ID).Find(&objectProperties)
	if res.Error != nil {
		return res.Error
	}

	propertiesMap := map[string]string{}
	for k, v := range obj.Properties.Fields {
		propertiesMap[k] = v
	}

	for i := range objectProperties {
		oldKv := objectProperties[i]
		if newV, ok := propertiesMap[oldKv.Name]; ok {
			if oldKv.Value != newV {
				oldKv.Value = newV
				needUpdatePropertiesModels = append(needUpdatePropertiesModels, oldKv)
			}
			delete(propertiesMap, oldKv.Name)
			continue
		}
		needDeletePropertiesModels = append(needDeletePropertiesModels, oldKv)
	}

	for k, v := range propertiesMap {
		needCreatePropertiesModels = append(needCreatePropertiesModels, ObjectProperty{OID: obj.ID, Name: k, Value: v})
	}

	if len(needCreatePropertiesModels) > 0 {
		for i := range needCreatePropertiesModels {
			property := needCreatePropertiesModels[i]
			res = tx.Create(&property)
			if res.Error != nil {
				return res.Error
			}
		}
	}
	if len(needUpdatePropertiesModels) > 0 {
		for i := range needUpdatePropertiesModels {
			property := needUpdatePropertiesModels[i]
			res = tx.Save(&property)
			if res.Error != nil {
				return res.Error
			}
		}
	}

	if len(needDeletePropertiesModels) > 0 {
		for i := range needDeletePropertiesModels {
			property := needDeletePropertiesModels[i]
			res = tx.Delete(&property)
			if res.Error != nil {
				return res.Error
			}
		}
	}
	return nil
}

func updateObjectLabels(tx *gorm.DB, obj *types.Object) error {
	labelModels := make([]Label, 0)
	res := tx.Where("ref_type = ? AND ref_id = ?", "object", obj.ID).Find(&labelModels)
	if res.Error != nil {
		return res.Error
	}

	labelsMap := map[string]string{}
	for _, kv := range obj.Labels.Labels {
		labelsMap[kv.Key] = kv.Value
	}

	needCreateLabelModels := make([]Label, 0)
	needUpdateLabelModels := make([]Label, 0)
	needDeleteLabelModels := make([]Label, 0)
	for i := range labelModels {
		oldKv := labelModels[i]
		if newV, ok := labelsMap[oldKv.Key]; ok {
			if oldKv.Value != newV {
				oldKv.Value = newV
				oldKv.SearchKey = labelSearchKey(oldKv.Key, newV)
				needUpdateLabelModels = append(needUpdateLabelModels, oldKv)
			}
			delete(labelsMap, oldKv.Key)
			continue
		}
		needDeleteLabelModels = append(needDeleteLabelModels, oldKv)
	}

	for k, v := range labelsMap {
		needCreateLabelModels = append(needCreateLabelModels, Label{RefType: "object", RefID: obj.ID, Key: k, Value: v, SearchKey: labelSearchKey(k, v)})
	}

	if len(needCreateLabelModels) > 0 {
		for i := range needCreateLabelModels {
			label := needCreateLabelModels[i]
			res = tx.Create(&label)
			if res.Error != nil {
				return res.Error
			}
		}
	}
	if len(needUpdateLabelModels) > 0 {
		for i := range needUpdateLabelModels {
			label := needUpdateLabelModels[i]
			res = tx.Save(label)
			if res.Error != nil {
				return res.Error
			}
		}
	}

	if len(needDeleteLabelModels) > 0 {
		for i := range needDeleteLabelModels {
			label := needDeleteLabelModels[i]
			res = tx.Delete(label)
			if res.Error != nil {
				return res.Error
			}
		}
	}
	return nil
}

func listObjectIdsWithLabelMatcher(ctx context.Context, db *gorm.DB, labelMatch types.LabelMatch) ([]int64, error) {
	db = db.WithContext(ctx)
	includeSearchKeys := make([]string, len(labelMatch.Include))
	for i, inKey := range labelMatch.Include {
		includeSearchKeys[i] = labelSearchKey(inKey.Key, inKey.Value)
	}

	var includeLabels []Label
	res := db.Select("ref_id").Where("ref_type = ? AND search_key IN ?", "object", includeSearchKeys).Find(&includeLabels)
	if res.Error != nil {
		return nil, res.Error
	}

	var excludeLabels []Label
	res = db.Select("ref_id").Where("ref_type = ? AND key IN ?", "object", labelMatch.Exclude).Find(&excludeLabels)
	if res.Error != nil {
		return nil, res.Error
	}

	targetObjects := make(map[int64]struct{})
	for _, in := range includeLabels {
		targetObjects[in.RefID] = struct{}{}
	}
	for _, ex := range excludeLabels {
		delete(targetObjects, ex.RefID)
	}

	idList := make([]int64, 0, len(targetObjects))
	for i := range targetObjects {
		idList = append(idList, i)
	}
	return idList, nil
}
