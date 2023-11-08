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
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm/clause"
	"runtime/trace"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/metastore/db"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
)

const (
	MemoryMeta   = config.MemoryMeta
	SqliteMeta   = config.SqliteMeta
	PostgresMeta = config.PostgresMeta
)

type sqliteMetaStore struct {
	dbStore *sqlMetaStore
	mux     sync.RWMutex
}

var _ Meta = &sqliteMetaStore{}

func (s *sqliteMetaStore) SystemInfo(ctx context.Context) (*types.SystemInfo, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.dbStore.SystemInfo(ctx)
}

func (s *sqliteMetaStore) GetEntry(ctx context.Context, id int64) (*types.Metadata, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.dbStore.GetEntry(ctx, id)
}

func (s *sqliteMetaStore) FindEntry(ctx context.Context, parentID int64, name string) (*types.Metadata, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.dbStore.FindEntry(ctx, parentID, name)
}

func (s *sqliteMetaStore) CreateEntry(ctx context.Context, parentID int64, newEntry *types.Metadata) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.CreateEntry(ctx, parentID, newEntry)
}

func (s *sqliteMetaStore) RemoveEntry(ctx context.Context, parentID, entryID int64) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.RemoveEntry(ctx, parentID, entryID)
}

func (s *sqliteMetaStore) DeleteRemovedEntry(ctx context.Context, entryID int64) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.DeleteRemovedEntry(ctx, entryID)
}

func (s *sqliteMetaStore) UpdateEntryMetadata(ctx context.Context, ed *types.Metadata) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.UpdateEntryMetadata(ctx, ed)
}

func (s *sqliteMetaStore) ListEntryChildren(ctx context.Context, parentId int64) (EntryIterator, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.dbStore.ListEntryChildren(ctx, parentId)
}

func (s *sqliteMetaStore) FilterEntries(ctx context.Context, filter types.Filter) (EntryIterator, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.dbStore.FilterEntries(ctx, filter)
}

func (s *sqliteMetaStore) Open(ctx context.Context, id int64, attr types.OpenAttr) (*types.Metadata, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.Open(ctx, id, attr)
}

func (s *sqliteMetaStore) Flush(ctx context.Context, id int64, size int64) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.Flush(ctx, id, size)
}

func (s *sqliteMetaStore) MirrorEntry(ctx context.Context, newEntry *types.Metadata) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.MirrorEntry(ctx, newEntry)
}

func (s *sqliteMetaStore) ChangeEntryParent(ctx context.Context, targetEntryId int64, newParentId int64, newName string, opt types.ChangeParentAttr) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.ChangeEntryParent(ctx, targetEntryId, newParentId, newName, opt)
}

func (s *sqliteMetaStore) GetEntryExtendData(ctx context.Context, id int64) (types.ExtendData, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.dbStore.GetEntryExtendData(ctx, id)
}

func (s *sqliteMetaStore) UpdateEntryExtendData(ctx context.Context, id int64, ed types.ExtendData) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.UpdateEntryExtendData(ctx, id, ed)
}

func (s *sqliteMetaStore) GetEntryLabels(ctx context.Context, id int64) (types.Labels, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.dbStore.GetEntryLabels(ctx, id)
}

func (s *sqliteMetaStore) UpdateEntryLabels(ctx context.Context, id int64, labels types.Labels) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.UpdateEntryLabels(ctx, id, labels)
}

func (s *sqliteMetaStore) NextSegmentID(ctx context.Context) (int64, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.NextSegmentID(ctx)
}

func (s *sqliteMetaStore) ListSegments(ctx context.Context, oid, chunkID int64, allChunk bool) ([]types.ChunkSeg, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.ListSegments(ctx, oid, chunkID, allChunk)
}

func (s *sqliteMetaStore) AppendSegments(ctx context.Context, seg types.ChunkSeg) (*types.Metadata, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.AppendSegments(ctx, seg)
}

func (s *sqliteMetaStore) DeleteSegment(ctx context.Context, segID int64) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.DeleteSegment(ctx, segID)
}

func (s *sqliteMetaStore) ListTask(ctx context.Context, taskID string, filter types.ScheduledTaskFilter) ([]*types.ScheduledTask, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.ListTask(ctx, taskID, filter)
}

func (s *sqliteMetaStore) SaveTask(ctx context.Context, task *types.ScheduledTask) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.SaveTask(ctx, task)
}

func (s *sqliteMetaStore) DeleteFinishedTask(ctx context.Context, aliveTime time.Duration) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.DeleteFinishedTask(ctx, aliveTime)
}

func (s *sqliteMetaStore) GetWorkflow(ctx context.Context, wfID string) (*types.WorkflowSpec, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.GetWorkflow(ctx, wfID)
}

func (s *sqliteMetaStore) ListWorkflow(ctx context.Context) ([]*types.WorkflowSpec, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.ListWorkflow(ctx)
}

func (s *sqliteMetaStore) DeleteWorkflow(ctx context.Context, wfID string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.DeleteWorkflow(ctx, wfID)
}

func (s *sqliteMetaStore) GetWorkflowJob(ctx context.Context, jobID string) (*types.WorkflowJob, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.GetWorkflowJob(ctx, jobID)
}

func (s *sqliteMetaStore) ListWorkflowJob(ctx context.Context, filter types.JobFilter) ([]*types.WorkflowJob, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.ListWorkflowJob(ctx, filter)
}

func (s *sqliteMetaStore) SaveWorkflow(ctx context.Context, wf *types.WorkflowSpec) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.SaveWorkflow(ctx, wf)
}

func (s *sqliteMetaStore) SaveWorkflowJob(ctx context.Context, wf *types.WorkflowJob) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.SaveWorkflowJob(ctx, wf)
}

func (s *sqliteMetaStore) DeleteWorkflowJob(ctx context.Context, wfJobID ...string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.DeleteWorkflowJob(ctx, wfJobID...)
}

func (s *sqliteMetaStore) ListNotifications(ctx context.Context) ([]types.Notification, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.ListNotifications(ctx)
}

func (s *sqliteMetaStore) RecordNotification(ctx context.Context, nid string, no types.Notification) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.RecordNotification(ctx, nid, no)
}

func (s *sqliteMetaStore) UpdateNotificationStatus(ctx context.Context, nid, status string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.UpdateNotificationStatus(ctx, nid, status)
}

func (s *sqliteMetaStore) PluginRecorder(plugin types.PlugScope) PluginRecorder {
	return &sqlPluginRecorder{
		pluginRecordHandler: s,
		plugin:              plugin,
	}
}

func (s *sqliteMetaStore) getPluginRecord(ctx context.Context, plugin types.PlugScope, rid string, record interface{}) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.getPluginRecord(ctx, plugin, rid, record)
}

func (s *sqliteMetaStore) listPluginRecords(ctx context.Context, plugin types.PlugScope, groupId string) ([]string, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.listPluginRecords(ctx, plugin, groupId)
}

func (s *sqliteMetaStore) savePluginRecord(ctx context.Context, plugin types.PlugScope, groupId, rid string, record interface{}) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.savePluginRecord(ctx, plugin, groupId, rid, record)
}

func (s *sqliteMetaStore) deletePluginRecord(ctx context.Context, plugin types.PlugScope, rid string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.deletePluginRecord(ctx, plugin, rid)
}

func (s *sqliteMetaStore) SaveDocument(ctx context.Context, doc *types.Document) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.SaveDocument(ctx, doc)
}

func (s *sqliteMetaStore) ListDocument(ctx context.Context) ([]*types.Document, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.ListDocument(ctx)
}

func (s *sqliteMetaStore) GetDocument(ctx context.Context, id string) (*types.Document, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.GetDocument(ctx, id)
}

func (s *sqliteMetaStore) DeleteDocument(ctx context.Context, id string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.DeleteDocument(ctx, id)
}

func newSqliteMetaStore(meta config.Meta) (*sqliteMetaStore, error) {
	dbEntity, err := gorm.Open(sqlite.Open(meta.Path), &gorm.Config{Logger: db.NewDbLogger()})
	if err != nil {
		return nil, err
	}

	dbConn, err := dbEntity.DB()
	if err != nil {
		return nil, err
	}

	if err = dbConn.Ping(); err != nil {
		return nil, err
	}

	dbEnt, err := db.NewDbEntity(dbEntity)
	if err != nil {
		return nil, err
	}

	dbStore, err := buildSqlMetaStore(dbEnt, dbEntity)
	if err != nil {
		return nil, err
	}

	return &sqliteMetaStore{dbStore: dbStore}, nil
}

type sqlMetaStore struct {
	*gorm.DB

	dbEntity *db.Entity
	logger   *zap.SugaredLogger
}

var _ Meta = &sqlMetaStore{}

func buildSqlMetaStore(entity *db.Entity, dbEntity *gorm.DB) (*sqlMetaStore, error) {
	s := &sqlMetaStore{dbEntity: entity, DB: dbEntity, logger: logger.NewLogger("dbStore")}

	if err := db.Migrate(s.DB); err != nil {
		return nil, db.SqlError2Error(err)
	}

	_, err := s.SystemInfo(context.TODO())
	if err != nil {
		if err != types.ErrNotFound {
			return nil, err
		}
		sysInfo := &db.SystemInfo{FsID: uuid.New().String()}
		if res := s.WithContext(context.Background()).Create(sysInfo); res.Error != nil {
			return nil, db.SqlError2Error(res.Error)
		}
	}
	return s, nil
}

func (s *sqlMetaStore) SystemInfo(ctx context.Context) (*types.SystemInfo, error) {
	defer trace.StartRegion(ctx, "metastore.sql.SystemInfo").End()
	info := &db.SystemInfo{}
	res := s.WithContext(ctx).First(info)
	if res.Error != nil {
		return nil, db.SqlError2Error(res.Error)
	}
	result := &types.SystemInfo{
		FilesystemID:  info.FsID,
		MaxSegmentID:  info.ChunkSeg,
		ObjectCount:   0,
		FileSizeTotal: 0,
	}

	res = s.WithContext(ctx).Model(&db.Object{}).Count(&result.ObjectCount)
	if res.Error != nil {
		return nil, db.SqlError2Error(res.Error)
	}

	if result.ObjectCount == 0 {
		return result, nil
	}

	res = s.WithContext(ctx).Model(&db.Object{}).Select("SUM(size) as file_size_total").Scan(&result.FileSizeTotal)
	if res.Error != nil {
		return nil, db.SqlError2Error(res.Error)
	}
	return result, nil
}

func (s *sqlMetaStore) GetEntry(ctx context.Context, id int64) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "metastore.sql.GetEntry").End()
	var objMod = &db.Object{ID: id}
	res := s.WithContext(ctx).Where("id = ?", id).First(objMod)
	if err := res.Error; err != nil {
		s.logger.Errorw("get entry by id failed", "entry", id, "err", err)
		return nil, db.SqlError2Error(err)
	}
	return objMod.ToEntry(), nil
}

func (s *sqlMetaStore) FindEntry(ctx context.Context, parentID int64, name string) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "metastore.sql.FindEntry").End()
	var objMod = &db.Object{ParentID: &parentID, Name: name}
	res := s.WithContext(ctx).Where("parent_id = ? AND name = ?", parentID, name).First(objMod)
	if err := res.Error; err != nil {
		s.logger.Errorw("find entry by name failed", "parent", parentID, "name", name, "err", err)
		return nil, db.SqlError2Error(err)
	}
	return objMod.ToEntry(), nil
}

func (s *sqlMetaStore) CreateEntry(ctx context.Context, parentID int64, newEntry *types.Metadata) error {
	defer trace.StartRegion(ctx, "metastore.sql.CreateEntry").End()
	var (
		parentMod = &db.Object{ID: parentID}
		entryMod  = (&db.Object{}).FromEntry(newEntry)
		nowTime   = time.Now().UnixNano()
	)
	if parentID != 0 {
		entryMod.ParentID = &parentID
	}
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Create(entryMod)
		if res.Error != nil {
			return res.Error
		}

		if parentID == 0 {
			return nil
		}

		res = tx.Where("id = ?", parentID).First(parentMod)
		if res.Error != nil {
			return res.Error
		}

		parentMod.ModifiedAt = nowTime
		parentMod.ChangedAt = nowTime
		if types.IsGroup(newEntry.Kind) {
			refCount := (*parentMod.RefCount) + 1
			parentMod.RefCount = &refCount
		}

		return updateEntryWithVersion(tx, parentMod)
	})
	if err != nil {
		s.logger.Errorw("create entry failed", "parent", parentID, "entry", newEntry.ID, "err", err)
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) UpdateEntryMetadata(ctx context.Context, entry *types.Metadata) error {
	defer trace.StartRegion(ctx, "metastore.sql.UpdateEntry").End()
	var entryMod = (&db.Object{}).FromEntry(entry)
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return updateEntryWithVersion(tx, entryMod)
	})
	if err != nil {
		s.logger.Errorw("create entry failed", "entry", entry.ID, "err", err)
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) RemoveEntry(ctx context.Context, parentID, entryID int64) error {
	defer trace.StartRegion(ctx, "metastore.sql.RemoveEntry").End()
	var (
		noneParentID int64 = 0
		srcMod       *db.Object
		parentMod    = &db.Object{ID: parentID}
		entryMod     = &db.Object{ID: entryID}
		nowTime      = time.Now().UnixNano()
	)
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("id = ?", parentID).First(parentMod)
		if res.Error != nil {
			return res.Error
		}
		res = tx.Where("id = ?", entryID).First(entryMod)
		if res.Error != nil {
			return res.Error
		}

		if entryMod.RefID != nil && *entryMod.RefID != 0 {
			srcMod = &db.Object{ID: *entryMod.RefID}
			res = tx.Where("id = ?", *entryMod.RefID).First(srcMod)
			if res.Error != nil {
				return res.Error
			}
		}

		parentMod.ModifiedAt = nowTime
		if types.IsGroup(types.Kind(entryMod.Kind)) {

			var entryChildCount int64
			res = tx.Model(&db.Object{}).Where("parent_id = ?", entryID).Count(&entryChildCount)
			if res.Error != nil {
				return res.Error
			}

			if entryChildCount > 0 {
				return types.ErrNotEmpty
			}

			parentRef := (*parentMod.RefCount) - 1
			parentMod.RefCount = &parentRef
		}
		if err := updateEntryWithVersion(tx, parentMod); err != nil {
			return err
		}

		if srcMod != nil {
			srcRef := (*srcMod.RefCount) - 1
			srcMod.RefCount = &srcRef
			srcMod.ModifiedAt = nowTime

			if err := updateEntryWithVersion(tx, srcMod); err != nil {
				return err
			}
		}

		entryRef := (*entryMod.RefCount) - 1
		entryMod.ParentID = &noneParentID
		entryMod.RefCount = &entryRef
		entryMod.ModifiedAt = nowTime

		return updateEntryWithVersion(tx, entryMod)
	})
	if err != nil {
		s.logger.Errorw("mark entry removed failed", "parent", parentID, "entry", entryID, "err", err)
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) DeleteRemovedEntry(ctx context.Context, entryID int64) error {
	defer trace.StartRegion(ctx, "metastore.sql.DeleteRemovedEntry").End()
	var (
		entryMod = &db.Object{ID: entryID}
		extModel = &db.ObjectExtend{ID: entryID}
	)
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("id = ?", entryID).First(entryMod)
		if res.Error != nil {
			return res.Error
		}

		if entryMod.RefCount != nil && *entryMod.RefCount > 0 {
			return fmt.Errorf("entry ref_count is %d, more than 0", *entryMod.RefCount)
		}

		res = tx.Delete(entryMod)
		if res.Error != nil {
			return res.Error
		}
		res = tx.Delete(extModel)
		if res.Error != nil {
			return res.Error
		}
		res = tx.Where("ref_type = 'object' AND ref_id = ?", entryID).Delete(&db.Label{})
		if res.Error != nil {
			return res.Error
		}
		return nil
	})
	if err != nil {
		s.logger.Errorw("delete removed entry failed", "entry", entryID, "err", err)
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) ListEntryChildren(ctx context.Context, parentId int64) (EntryIterator, error) {
	defer trace.StartRegion(ctx, "metastore.sql.ListEntryChildren").End()
	var total int64
	tx := s.WithContext(ctx).Model(&db.Object{}).Where("parent_id = ?", parentId)
	res := tx.Count(&total)
	if err := res.Error; err != nil {
		s.logger.Errorw("count children entry failed", "parent", parentId, "err", err)
		return nil, db.SqlError2Error(err)
	}
	return newTransactionEntryIterator(tx, total), nil
}

func (s *sqlMetaStore) FilterEntries(ctx context.Context, filter types.Filter) (EntryIterator, error) {
	defer trace.StartRegion(ctx, "metastore.sql.FilterEntries").End()
	var (
		scopeIds []int64
		err      error
	)
	if len(filter.Label.Include) > 0 || len(filter.Label.Exclude) > 0 {
		scopeIds, err = listObjectIdsWithLabelMatcher(ctx, s.DB, filter.Label)
		if err != nil {
			return nil, db.SqlError2Error(err)
		}
	}

	var total int64
	tx := queryFilter(s.WithContext(ctx).Model(&db.Object{}), filter, scopeIds)
	res := tx.Count(&total)
	if err = res.Error; err != nil {
		s.logger.Errorw("count filtered entry failed", "filter", filter, "err", err)
		return nil, db.SqlError2Error(err)
	}
	return newTransactionEntryIterator(tx, total), nil
}

func (s *sqlMetaStore) Open(ctx context.Context, id int64, attr types.OpenAttr) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "metastore.sql.Open").End()
	var (
		enMod   = &db.Object{}
		nowTime = time.Now().UnixNano()
	)
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("id = ?", id).First(enMod)
		if res.Error != nil {
			return res.Error
		}
		// do not update ctime
		enMod.AccessAt = nowTime
		if attr.Write {
			enMod.ModifiedAt = nowTime
		}
		if attr.Trunc {
			var zeroSize int64 = 0
			enMod.Size = &zeroSize
		}
		return updateEntryWithVersion(tx, enMod)
	})
	if err != nil {
		s.logger.Errorw("open entry failed", "entry", id, "err", err)
		return nil, db.SqlError2Error(err)
	}
	return enMod.ToEntry(), nil
}

func (s *sqlMetaStore) Flush(ctx context.Context, id int64, size int64) error {
	defer trace.StartRegion(ctx, "metastore.sql.Flush").End()
	var (
		enMod   = &db.Object{}
		nowTime = time.Now().UnixNano()
	)
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("id = ?", id).First(enMod)
		if res.Error != nil {
			return res.Error
		}
		enMod.ModifiedAt = nowTime
		enMod.Size = &size
		return updateEntryWithVersion(tx, enMod)
	})
	if err != nil {
		s.logger.Errorw("open entry failed", "entry", id, "err", err)
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) GetEntryExtendData(ctx context.Context, id int64) (types.ExtendData, error) {
	defer trace.StartRegion(ctx, "metastore.sql.GetEntryExtendData").End()
	var (
		ext      = &db.ObjectExtend{}
		property = make([]db.ObjectProperty, 0)
	)

	if err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("id = ?", id).First(ext)
		if res.Error != nil {
			return res.Error
		}
		res = tx.Where("oid = ?", id).Find(&property)
		if res.Error != nil {
			return res.Error
		}
		return nil
	}); err != nil {
		return types.ExtendData{}, db.SqlError2Error(err)
	}

	ed := ext.ToExtData()
	for _, p := range property {
		ed.Properties.Fields[p.Name] = p.Value
	}
	return ed, nil
}

func (s *sqlMetaStore) UpdateEntryExtendData(ctx context.Context, id int64, ed types.ExtendData) error {
	defer trace.StartRegion(ctx, "metastore.sql.UpdateEntryExtendData").End()
	var (
		extModel                   = &db.ObjectExtend{ID: id}
		objectProperties           = make([]db.ObjectProperty, 0)
		needCreatePropertiesModels = make([]db.ObjectProperty, 0)
		needUpdatePropertiesModels = make([]db.ObjectProperty, 0)
		needDeletePropertiesModels = make([]db.ObjectProperty, 0)
	)

	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		extModel.From(ed)
		res := tx.Save(extModel)
		if res.Error != nil {
			return res.Error
		}

		res = tx.Where("oid = ?", id).Find(&objectProperties)
		if res.Error != nil {
			return res.Error
		}

		propertiesMap := map[string]string{}
		for k, v := range ed.Properties.Fields {
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
			needCreatePropertiesModels = append(needCreatePropertiesModels, db.ObjectProperty{OID: id, Name: k, Value: v})
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
	})
	if err != nil {
		s.logger.Errorw("save entry extend data failed", "entry", id, "err", err)
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) MirrorEntry(ctx context.Context, newEntry *types.Metadata) error {
	if newEntry.ParentID == 0 || newEntry.RefID == 0 {
		s.logger.Errorw("mirror entry failed", "parentID", newEntry.ParentID, "srcID", newEntry.RefID, "err", "parent or src id is empty")
		return types.ErrNotFound
	}

	defer trace.StartRegion(ctx, "metastore.sql.MirrorEntry").End()
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var (
			enModel          = (&db.Object{}).FromEntry(newEntry)
			srcEnModel       = &db.Object{ID: newEntry.RefID}
			dstParentEnModel = &db.Object{ID: newEntry.ParentID}
			nowTime          = time.Now().UnixNano()
			updateErr        error
		)

		res := tx.First(srcEnModel)
		if res.Error != nil {
			return res.Error
		}
		srcRefCount := *srcEnModel.RefCount + 1
		srcEnModel.RefCount = &srcRefCount
		srcEnModel.ChangedAt = nowTime
		if updateErr = updateEntryWithVersion(tx, srcEnModel); updateErr != nil {
			return updateErr
		}

		res = tx.First(dstParentEnModel)
		if res.Error != nil {
			return res.Error
		}
		dstParentEnModel.ChangedAt = nowTime
		dstParentEnModel.ModifiedAt = nowTime
		if updateErr = updateEntryWithVersion(tx, dstParentEnModel); updateErr != nil {
			return updateErr
		}

		res = tx.Create(enModel)
		if res.Error != nil {
			return res.Error
		}
		return nil
	})
	if err != nil {
		s.logger.Errorw("mirror entry failed", "entry", newEntry.ID, "parentID", newEntry.ParentID, "srcID", newEntry.RefID, "err", err)
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) ChangeEntryParent(ctx context.Context, targetEntryId int64, newParentId int64, newName string, opt types.ChangeParentAttr) error {
	defer trace.StartRegion(ctx, "metastore.sql.ChangeEntryParent").End()
	return s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var (
			enModel          = &db.Object{ID: targetEntryId}
			srcParentEnModel = &db.Object{}
			dstParentEnModel = &db.Object{}
			nowTime          = time.Now().UnixNano()
			updateErr        error
		)
		res := tx.First(enModel)
		if res.Error != nil {
			return res.Error
		}
		res = tx.Where("id = ?", enModel.ParentID).First(srcParentEnModel)
		if res.Error != nil {
			return res.Error
		}

		enModel.Name = newName
		enModel.ParentID = &newParentId
		if updateErr = updateEntryWithVersion(tx, enModel); updateErr != nil {
			return updateErr
		}

		if types.IsGroup(types.Kind(enModel.Kind)) {
			res = tx.Where("id = ?", newParentId).First(dstParentEnModel)
			if res.Error != nil {
				return res.Error
			}
			dstParentEnModel.ChangedAt = nowTime
			dstParentEnModel.ModifiedAt = nowTime
			dstParentRef := *dstParentEnModel.RefCount + 1
			dstParentEnModel.RefCount = &dstParentRef
			if updateErr = updateEntryWithVersion(tx, dstParentEnModel); updateErr != nil {
				return updateErr
			}

			srcParentRef := *srcParentEnModel.RefCount - 1
			srcParentEnModel.RefCount = &srcParentRef
		}
		srcParentEnModel.ChangedAt = nowTime
		srcParentEnModel.ModifiedAt = nowTime
		if updateErr = updateEntryWithVersion(tx, srcParentEnModel); updateErr != nil {
			return updateErr
		}
		return nil
	})
}

func (s *sqlMetaStore) GetEntryLabels(ctx context.Context, id int64) (types.Labels, error) {
	defer trace.StartRegion(ctx, "metastore.sql.GetEntryLabels").End()
	var (
		result    types.Labels
		labelMods []db.Label
	)

	res := s.WithContext(ctx).Where("ref_type = ? and ref_id = ?", "object", id).Find(&labelMods)
	if res.Error != nil {
		s.logger.Errorw("get entry labels failed", "entry", id, "err", res.Error)
		return result, db.SqlError2Error(res.Error)
	}

	for _, l := range labelMods {
		result.Labels = append(result.Labels, types.Label{Key: l.Key, Value: l.Value})
	}
	return result, nil
}

func (s *sqlMetaStore) UpdateEntryLabels(ctx context.Context, id int64, labels types.Labels) error {
	defer trace.StartRegion(ctx, "metastore.sql.UpdateEntryLabels").End()

	var (
		needCreateLabelModels = make([]db.Label, 0)
		needUpdateLabelModels = make([]db.Label, 0)
		needDeleteLabelModels = make([]db.Label, 0)
	)

	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		labelModels := make([]db.Label, 0)
		res := tx.Where("ref_type = ? AND ref_id = ?", "object", id).Find(&labelModels)
		if res.Error != nil {
			return res.Error
		}

		labelsMap := map[string]string{}
		for _, kv := range labels.Labels {
			labelsMap[kv.Key] = kv.Value
		}
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
			needCreateLabelModels = append(needCreateLabelModels, db.Label{RefType: "object", RefID: id, Key: k, Value: v, SearchKey: labelSearchKey(k, v)})
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
	})
	if err != nil {
		s.logger.Errorw("update entry labels failed", "entry", id, "err", err)
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) NextSegmentID(ctx context.Context) (int64, error) {
	defer trace.StartRegion(ctx, "metastore.sql.NextSegmentID").End()
	info := &db.SystemInfo{}
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(info)
		if res.Error != nil {
			return res.Error
		}
		info.ChunkSeg += 1
		res = tx.Save(info)
		if res.Error != nil {
			return res.Error
		}
		return nil
	})

	if err != nil {
		return 0, db.SqlError2Error(err)
	}
	return info.ChunkSeg, nil
}

func (s *sqlMetaStore) ListSegments(ctx context.Context, oid, chunkID int64, allChunk bool) ([]types.ChunkSeg, error) {
	defer trace.StartRegion(ctx, "metastore.sql.ListSegments").End()
	segments := make([]db.ObjectChunk, 0)
	if allChunk {
		res := s.WithContext(ctx).Where("oid = ?", oid).Order("append_at").Find(&segments)
		if res.Error != nil {
			return nil, db.SqlError2Error(res.Error)
		}
	} else {
		res := s.WithContext(ctx).Where("oid = ? AND chunk_id = ?", oid, chunkID).Order("append_at").Find(&segments)
		if res.Error != nil {
			return nil, db.SqlError2Error(res.Error)
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

func (s *sqlMetaStore) AppendSegments(ctx context.Context, seg types.ChunkSeg) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "metastore.sql.AppendSegments").End()
	var (
		enMod   = &db.Object{ID: seg.ObjectID}
		nowTime = time.Now().UnixNano()
		err     error
	)
	err = s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(enMod)
		if res.Error != nil {
			return res.Error
		}

		res = tx.Create(&db.ObjectChunk{
			ID:       seg.ID,
			OID:      seg.ObjectID,
			ChunkID:  seg.ChunkID,
			Off:      seg.Off,
			Len:      seg.Len,
			State:    seg.State,
			AppendAt: nowTime,
		})
		if res.Error != nil {
			return res.Error
		}

		newSize := seg.Off + seg.Len
		if newSize > *enMod.Size {
			enMod.Size = &newSize
		}
		enMod.ModifiedAt = nowTime

		if writeBackErr := updateEntryWithVersion(tx, enMod); writeBackErr != nil {
			return writeBackErr
		}
		return nil
	})
	if err != nil {
		return nil, db.SqlError2Error(err)
	}
	return enMod.ToEntry(), nil
}

func (s *sqlMetaStore) DeleteSegment(ctx context.Context, segID int64) error {
	defer trace.StartRegion(ctx, "metastore.sql.DeleteSegment").End()
	res := s.WithContext(ctx).Delete(&db.ObjectChunk{ID: segID})
	return db.SqlError2Error(res.Error)
}

func (s *sqlMetaStore) ListTask(ctx context.Context, taskID string, filter types.ScheduledTaskFilter) ([]*types.ScheduledTask, error) {
	defer trace.StartRegion(ctx, "metastore.sql.ListTask").End()
	result, err := s.dbEntity.ListScheduledTask(ctx, taskID, filter)
	if err != nil {
		return nil, db.SqlError2Error(err)
	}
	return result, nil
}

func (s *sqlMetaStore) SaveTask(ctx context.Context, task *types.ScheduledTask) error {
	defer trace.StartRegion(ctx, "metastore.sql.SaveTask").End()
	return db.SqlError2Error(s.dbEntity.SaveScheduledTask(ctx, task))
}

func (s *sqlMetaStore) DeleteFinishedTask(ctx context.Context, aliveTime time.Duration) error {
	defer trace.StartRegion(ctx, "metastore.sql.DeleteFinishedTask").End()
	return db.SqlError2Error(s.dbEntity.DeleteFinishedScheduledTask(ctx, aliveTime))
}

func (s *sqlMetaStore) GetWorkflow(ctx context.Context, wfID string) (*types.WorkflowSpec, error) {
	defer trace.StartRegion(ctx, "metastore.sql.GetWorkflow").End()
	wf, err := s.dbEntity.GetWorkflow(ctx, wfID)
	if err != nil {
		return nil, db.SqlError2Error(err)
	}
	return wf, nil
}

func (s *sqlMetaStore) ListWorkflow(ctx context.Context) ([]*types.WorkflowSpec, error) {
	defer trace.StartRegion(ctx, "metastore.sql.ListWorkflow").End()
	wfList, err := s.dbEntity.ListWorkflow(ctx)
	if err != nil {
		return nil, db.SqlError2Error(err)
	}
	return wfList, nil
}

func (s *sqlMetaStore) DeleteWorkflow(ctx context.Context, wfID string) error {
	defer trace.StartRegion(ctx, "metastore.sql.DeleteWorkflow").End()
	err := s.dbEntity.DeleteWorkflow(ctx, wfID)
	if err != nil {
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) GetWorkflowJob(ctx context.Context, jobID string) (*types.WorkflowJob, error) {
	defer trace.StartRegion(ctx, "metastore.sql.GetWorkflowJob").End()
	jobList, err := s.dbEntity.ListWorkflowJob(ctx, types.JobFilter{JobID: jobID})
	if err != nil {
		return nil, db.SqlError2Error(err)
	}
	if len(jobList) == 0 {
		return nil, types.ErrNotFound
	}
	return jobList[0], nil
}

func (s *sqlMetaStore) ListWorkflowJob(ctx context.Context, filter types.JobFilter) ([]*types.WorkflowJob, error) {
	defer trace.StartRegion(ctx, "metastore.sql.ListWorkflowJob").End()
	jobList, err := s.dbEntity.ListWorkflowJob(ctx, filter)
	if err != nil {
		return nil, db.SqlError2Error(err)
	}
	return jobList, nil
}

func (s *sqlMetaStore) SaveWorkflow(ctx context.Context, wf *types.WorkflowSpec) error {
	defer trace.StartRegion(ctx, "metastore.sql.SaveWorkflow").End()
	wf.UpdatedAt = time.Now()
	err := s.dbEntity.SaveWorkflow(ctx, wf)
	if err != nil {
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) SaveWorkflowJob(ctx context.Context, wf *types.WorkflowJob) error {
	defer trace.StartRegion(ctx, "metastore.sql.SaveWorkflowJob").End()
	wf.UpdatedAt = time.Now()
	err := s.dbEntity.SaveWorkflowJob(ctx, wf)
	if err != nil {
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) DeleteWorkflowJob(ctx context.Context, wfJobID ...string) error {
	defer trace.StartRegion(ctx, "metastore.sql.DeleteWorkflowJob").End()
	err := s.dbEntity.DeleteWorkflowJob(ctx, wfJobID...)
	if err != nil {
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) ListNotifications(ctx context.Context) ([]types.Notification, error) {
	defer trace.StartRegion(ctx, "metastore.sql.ListNotifications").End()
	noList, err := s.dbEntity.ListNotifications(ctx)
	if err != nil {
		return nil, db.SqlError2Error(err)
	}
	return noList, nil
}

func (s *sqlMetaStore) RecordNotification(ctx context.Context, nid string, no types.Notification) error {
	defer trace.StartRegion(ctx, "metastore.sql.RecordNotification").End()
	err := s.dbEntity.RecordNotification(ctx, nid, no)
	if err != nil {
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) UpdateNotificationStatus(ctx context.Context, nid, status string) error {
	defer trace.StartRegion(ctx, "metastore.sql.UpdateNotificationStatus").End()
	err := s.dbEntity.UpdateNotificationStatus(ctx, nid, status)
	if err != nil {
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) PluginRecorder(plugin types.PlugScope) PluginRecorder {
	return &sqlPluginRecorder{pluginRecordHandler: s, plugin: plugin}
}

func (s *sqlMetaStore) getPluginRecord(ctx context.Context, plugin types.PlugScope, rid string, record interface{}) error {
	defer trace.StartRegion(ctx, "metastore.sql.getPluginRecord").End()
	return s.dbEntity.GetPluginRecord(ctx, plugin, rid, record)
}

func (s *sqlMetaStore) listPluginRecords(ctx context.Context, plugin types.PlugScope, groupId string) ([]string, error) {
	defer trace.StartRegion(ctx, "metastore.sql.listPluginRecords").End()
	return s.dbEntity.ListPluginRecords(ctx, plugin, groupId)
}

func (s *sqlMetaStore) savePluginRecord(ctx context.Context, plugin types.PlugScope, groupId, rid string, record interface{}) error {
	defer trace.StartRegion(ctx, "metastore.sql.savePluginRecord").End()
	return s.dbEntity.SavePluginRecord(ctx, plugin, groupId, rid, record)
}

func (s *sqlMetaStore) deletePluginRecord(ctx context.Context, plugin types.PlugScope, rid string) error {
	defer trace.StartRegion(ctx, "metastore.sql.deletePluginRecord").End()
	return s.dbEntity.DeletePluginRecord(ctx, plugin, rid)
}

func (s *sqlMetaStore) SaveDocument(ctx context.Context, doc *types.Document) error {
	defer trace.StartRegion(ctx, "metastore.sql.saveDocument").End()
	return s.dbEntity.SaveDocument(ctx, doc)
}

func (s *sqlMetaStore) ListDocument(ctx context.Context) ([]*types.Document, error) {
	defer trace.StartRegion(ctx, "metastore.sql.listDocuments").End()
	return s.dbEntity.ListDocument(ctx)
}

func (s *sqlMetaStore) GetDocument(ctx context.Context, id string) (*types.Document, error) {
	defer trace.StartRegion(ctx, "metastore.sql.getDocument").End()
	return s.dbEntity.GetDocument(ctx, id)
}

func (s *sqlMetaStore) DeleteDocument(ctx context.Context, id string) error {
	defer trace.StartRegion(ctx, "metastore.sql.deleteDocument").End()
	return s.dbEntity.DeleteDocument(ctx, id)
}

func newPostgresMetaStore(meta config.Meta) (*sqlMetaStore, error) {
	dbEntity, err := gorm.Open(postgres.Open(meta.DSN), &gorm.Config{Logger: db.NewDbLogger()})
	if err != nil {
		panic(err)
	}

	dbConn, err := dbEntity.DB()
	if err != nil {
		return nil, err
	}

	dbConn.SetMaxIdleConns(5)
	dbConn.SetMaxOpenConns(50)
	dbConn.SetConnMaxLifetime(time.Hour)

	if err = dbConn.Ping(); err != nil {
		return nil, err
	}

	dbEnt, err := db.NewDbEntity(dbEntity)
	if err != nil {
		return nil, err
	}

	return buildSqlMetaStore(dbEnt, dbEntity)
}

type pluginRecordHandler interface {
	getPluginRecord(ctx context.Context, plugin types.PlugScope, rid string, record interface{}) error
	listPluginRecords(ctx context.Context, plugin types.PlugScope, groupId string) ([]string, error)
	savePluginRecord(ctx context.Context, plugin types.PlugScope, groupId, rid string, record interface{}) error
	deletePluginRecord(ctx context.Context, plugin types.PlugScope, rid string) error
}

type sqlPluginRecorder struct {
	pluginRecordHandler
	plugin types.PlugScope
}

func (s *sqlPluginRecorder) GetRecord(ctx context.Context, rid string, record interface{}) error {
	return db.SqlError2Error(s.getPluginRecord(ctx, s.plugin, rid, record))
}

func (s *sqlPluginRecorder) ListRecords(ctx context.Context, groupId string) ([]string, error) {
	result, err := s.listPluginRecords(ctx, s.plugin, groupId)
	if err != nil {
		return nil, db.SqlError2Error(err)
	}
	return result, nil
}

func (s *sqlPluginRecorder) SaveRecord(ctx context.Context, groupId, rid string, record interface{}) error {
	return db.SqlError2Error(s.savePluginRecord(ctx, s.plugin, groupId, rid, record))
}

func (s *sqlPluginRecorder) DeleteRecord(ctx context.Context, rid string) error {
	return db.SqlError2Error(s.deletePluginRecord(ctx, s.plugin, rid))
}

func updateEntryWithVersion(tx *gorm.DB, entryMod *db.Object) error {
	currentVersion := entryMod.Version
	entryMod.Version += 1
	if entryMod.Version < 0 {
		entryMod.Version = 1024
	}

	res := tx.Where("version = ?", currentVersion).Updates(entryMod)
	if res.Error != nil || res.RowsAffected == 0 {
		if res.RowsAffected == 0 {
			return types.ErrConflict
		}
		return res.Error
	}
	return nil
}

func listObjectIdsWithLabelMatcher(ctx context.Context, tx *gorm.DB, labelMatch types.LabelMatch) ([]int64, error) {
	tx = tx.WithContext(ctx)
	includeSearchKeys := make([]string, len(labelMatch.Include))
	for i, inKey := range labelMatch.Include {
		includeSearchKeys[i] = labelSearchKey(inKey.Key, inKey.Value)
	}

	var includeLabels []db.Label
	res := tx.Where("ref_type = ? AND search_key IN ?", "object", includeSearchKeys).Find(&includeLabels)
	if res.Error != nil {
		return nil, res.Error
	}

	var excludeLabels []db.Label
	res = tx.Select("ref_id").Where("ref_type = ? AND key IN ?", "object", labelMatch.Exclude).Find(&excludeLabels)
	if res.Error != nil {
		return nil, res.Error
	}

	targetObjects := make(map[int64]int)
	for _, in := range includeLabels {
		targetObjects[in.RefID] += 1
	}
	for _, ex := range excludeLabels {
		delete(targetObjects, ex.RefID)
	}

	idList := make([]int64, 0, len(targetObjects))
	for i, matchedKeyCount := range targetObjects {
		if matchedKeyCount != len(labelMatch.Include) {
			// part match
			continue
		}
		idList = append(idList, i)
	}
	return idList, nil
}

func queryFilter(tx *gorm.DB, filter types.Filter, scopeIds []int64) *gorm.DB {
	if filter.ID != 0 {
		tx = tx.Where("id = ?", filter.ID)
	} else if len(scopeIds) > 0 {
		tx = tx.Where("id IN ?", scopeIds)
	}

	if filter.ParentID != 0 {
		tx = tx.Where("parent_id = ?", filter.ParentID)
	}
	if filter.RefID != 0 {
		tx = tx.Where("ref_id = ?", filter.RefID)
	}
	if filter.Name != "" {
		tx = tx.Where("name = ?", filter.Name)
	}
	if filter.Namespace != "" {
		tx = tx.Where("namespace = ?", filter.Namespace)
	}
	if filter.Kind != "" {
		tx = tx.Where("kind = ?", filter.Kind)
	}
	return tx
}

func labelSearchKey(k, v string) string {
	return fmt.Sprintf("%s=%s", k, v)
}
