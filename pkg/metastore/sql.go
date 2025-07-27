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
	"encoding/json"
	"errors"
	"runtime/trace"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"gorm.io/gorm/clause"

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

var (
	initMetrics sync.Once
	requireLock = func() {}
	releaseLock = func() {}
	bigLock     sync.Mutex
)

func newSqliteMetaStore(meta config.Meta) (*sqlMetaStore, error) {
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

	dbConn.SetMaxIdleConns(1)
	dbConn.SetMaxOpenConns(1)
	dbConn.SetConnMaxLifetime(time.Hour)

	dbStore, err := buildSqlMetaStore(dbEntity)
	if err != nil {
		return nil, err
	}

	requireLock = func() { bigLock.Lock() }
	releaseLock = func() { bigLock.Unlock() }

	return dbStore, nil
}

type sqlMetaStore struct {
	*gorm.DB
	logger *zap.SugaredLogger
}

var _ Meta = &sqlMetaStore{}

func buildSqlMetaStore(dbEntity *gorm.DB) (*sqlMetaStore, error) {
	s := &sqlMetaStore{DB: dbEntity, logger: logger.NewLogger("dbStore")}

	s.logger.Info("migrate db")
	err := s.WithContext(context.TODO()).Transaction(func(tx *gorm.DB) error {
		return db.Migrate(tx)
	})
	if err != nil {
		s.logger.Fatalf("migrate db failed: %s", err)
		return nil, db.SqlError2Error(err)
	}
	s.logger.Info("migrate db finish")

	_, err = s.SystemInfo(context.TODO())
	if err != nil {
		if err != types.ErrNotFound {
			return nil, err
		}
		sysInfo := &db.SystemInfo{FsID: uuid.New().String()}
		if res := s.WithContext(context.Background()).Create(sysInfo); res.Error != nil {
			return nil, db.SqlError2Error(res.Error)
		}
	}

	initMetrics.Do(func() {
		goDb, innerErr := dbEntity.DB()
		if innerErr == nil {
			registerErr := prometheus.Register(collectors.NewDBStatsCollector(goDb, dbEntity.Name()))
			if registerErr != nil {
				s.logger.Warnf("register dbstats collector failed: %s", err)
			}
			return
		}
		s.logger.Warnf("init db metrics failed: %s", err)
	})
	return s, nil
}

func (s *sqlMetaStore) SystemInfo(ctx context.Context) (*types.SystemInfo, error) {
	defer trace.StartRegion(ctx, "metastore.sql.SystemInfo").End()
	requireLock()
	defer releaseLock()
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

	res = s.WithContext(ctx).Model(&db.Entry{}).Count(&result.ObjectCount)
	if res.Error != nil {
		return nil, db.SqlError2Error(res.Error)
	}

	if result.ObjectCount == 0 {
		return result, nil
	}

	var totalChunk int64
	res = s.WithContext(ctx).Model(&db.EntryChunk{}).Count(&totalChunk)
	if res.Error != nil {
		return nil, db.SqlError2Error(res.Error)
	}
	if totalChunk == 0 {
		return result, nil
	}

	// real usage in storage
	res = s.WithContext(ctx).Model(&db.EntryChunk{}).Select("SUM(len) as file_size_total").Scan(&result.FileSizeTotal)
	if res.Error != nil {
		return nil, db.SqlError2Error(res.Error)
	}
	return result, nil
}

func (s *sqlMetaStore) GetConfigValue(ctx context.Context, namespace, group, name string) (string, error) {
	defer trace.StartRegion(ctx, "metastore.sql.GetConfigValue").End()
	requireLock()
	defer releaseLock()
	var sysCfg = db.SystemConfig{}
	res := s.WithNamespace(ctx, namespace).Where("cfg_group = ? AND cfg_name = ?", group, name).First(&sysCfg)
	if res.Error != nil {
		return "", db.SqlError2Error(res.Error)
	}
	return sysCfg.Value, nil
}

func (s *sqlMetaStore) SetConfigValue(ctx context.Context, namespace, group, name, value string) error {
	defer trace.StartRegion(ctx, "metastore.sql.SetConfigValue").End()
	requireLock()
	defer releaseLock()
	var sysCfg = db.SystemConfig{}
	res := s.WithNamespace(ctx, namespace).Where("cfg_group = ? AND cfg_name = ?", group, name).First(&sysCfg)
	if res.Error != nil && !errors.Is(res.Error, gorm.ErrRecordNotFound) {
		return res.Error
	}

	sysCfg.Namespace = namespace
	sysCfg.Group = group
	sysCfg.Name = name
	sysCfg.Value = value
	sysCfg.ChangedAt = time.Now()
	res = s.WithContext(ctx).Save(&sysCfg)
	return res.Error
}

func (s *sqlMetaStore) GetEntry(ctx context.Context, namespace string, id int64) (*types.Entry, error) {
	defer trace.StartRegion(ctx, "metastore.sql.GetEntry").End()
	requireLock()
	defer releaseLock()
	var objMod = &db.Entry{ID: id}
	res := s.WithNamespace(ctx, namespace).Where("id = ?", id).First(objMod)
	if err := res.Error; err != nil {
		return nil, db.SqlError2Error(err)
	}
	return objMod.ToEntry(), nil
}

func (s *sqlMetaStore) FindEntry(ctx context.Context, namespace string, parentID int64, name string) (*types.Child, error) {
	defer trace.StartRegion(ctx, "metastore.sql.FindEntry").End()
	requireLock()
	defer releaseLock()
	var (
		child = &db.Children{}
	)
	res := s.WithNamespace(ctx, namespace).Where("parent_id = ? AND name = ?", parentID, name).First(child)
	if err := res.Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Errorw("find entry by name failed", "parent", parentID, "name", name, "err", err)
		}
		return nil, db.SqlError2Error(err)
	}
	return child.To(), nil
}

func (s *sqlMetaStore) GetChild(ctx context.Context, namespace string, parentID, id int64) (*types.Child, error) {
	defer trace.StartRegion(ctx, "metastore.sql.GetChild").End()
	requireLock()
	defer releaseLock()
	var (
		child = &db.Children{}
	)
	res := s.WithNamespace(ctx, namespace).Where("parent_id = ? AND child_id = ?", parentID, id).First(child)
	if err := res.Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Errorw("find entry by id failed", "parent", parentID, "id", id, "err", err)
		}
		return nil, db.SqlError2Error(err)
	}
	return child.To(), nil
}

func (s *sqlMetaStore) CreateEntry(ctx context.Context, namespace string, parentID int64, newEntry *types.Entry) error {
	defer trace.StartRegion(ctx, "metastore.sql.CreateEntry").End()
	requireLock()
	defer releaseLock()
	var (
		parentMod = &db.Entry{ID: parentID}
		entryMod  = (&db.Entry{}).FromEntry(newEntry)
		nowTime   = time.Now().UnixNano()
	)
	entryMod.Namespace = namespace
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Create(entryMod)
		if res.Error != nil {
			return res.Error
		}

		if parentID != 0 && parentID != entryMod.ID {
			res = tx.Create(&db.Children{
				ParentID:  parentID,
				ChildID:   entryMod.ID,
				Name:      entryMod.Name,
				Namespace: namespace,
			})
			if res.Error != nil {
				return res.Error
			}
		}

		if parentID == 0 {
			return nil
		}

		res = tx.Where("id = ? AND namespace = ?", parentID, namespace).First(parentMod)
		if res.Error != nil {
			return res.Error
		}

		parentMod.ModifiedAt = nowTime
		parentMod.ChangedAt = nowTime
		if newEntry.IsGroup {
			refCount := (*parentMod.RefCount) + 1
			parentMod.RefCount = &refCount
		}

		return updateEntryModelWithVersion(tx, parentMod)
	})
	if err != nil {
		s.logger.Errorw("create entry failed", "parent", parentID, "entry", newEntry.ID, "err", err)
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) UpdateEntry(ctx context.Context, namespace string, entry *types.Entry) error {
	defer trace.StartRegion(ctx, "metastore.sql.UpdateEntry").End()
	requireLock()
	defer releaseLock()
	entry.ChangedAt = time.Now()
	var entryMod = (&db.Entry{}).FromEntry(entry)
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return updateEntryModelWithVersion(tx, entryMod)
	})
	if err != nil {
		s.logger.Errorw("update entry failed", "entry", entry.ID, "err", err)
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) RemoveEntry(ctx context.Context, namespace string, parentID, entryID int64, entryName string, attr types.DeleteEntry) error {
	defer trace.StartRegion(ctx, "metastore.sql.RemoveEntry").End()
	requireLock()
	defer releaseLock()
	var (
		parentMod = &db.Entry{ID: parentID}
		entryMod  = &db.Entry{ID: entryID}
		childRef  = &db.Children{}
		nowTime   = time.Now().UnixNano()
	)
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := namespaceQuery(tx, namespace).Where("id = ?", parentID).First(parentMod)
		if res.Error != nil {
			return res.Error
		}
		res = namespaceQuery(tx, namespace).Where("id = ?", entryID).First(entryMod)
		if res.Error != nil {
			return res.Error
		}

		res = tx.Where("parent_id = ? AND child_id = ? AND name = ? AND namespace = ?", parentID, entryID, entryName, namespace).First(childRef)
		if res.Error != nil {
			if !errors.Is(res.Error, gorm.ErrRecordNotFound) {
				return nil
			}
			return res.Error
		}

		parentMod.ModifiedAt = nowTime
		parentMod.ChangedAt = nowTime
		if entryMod.IsGroup {
			var entryChildCount int64
			res = namespaceQuery(tx, namespace).Model(&db.Children{}).Where("parent_id = ?", entryID).Count(&entryChildCount)
			if res.Error != nil {
				return res.Error
			}

			if entryChildCount > 0 {
				s.logger.Infow("delete a not empty group", "entryChildCount", entryChildCount)
				return types.ErrNotEmpty
			}

			parentRef := (*parentMod.RefCount) - 1
			parentMod.RefCount = &parentRef
		}
		if err := updateEntryModelWithVersion(tx, parentMod); err != nil {
			return err
		}

		res = tx.Delete(childRef)
		if res.Error != nil {
			return res.Error
		}
		entryRef := (*entryMod.RefCount) - 1
		entryMod.RefCount = &entryRef
		entryMod.ModifiedAt = nowTime
		entryMod.ChangedAt = nowTime
		return updateEntryModelWithVersion(tx, entryMod)
	})
	if err != nil {
		s.logger.Errorw("mark entry removed failed", "parent", parentID, "entry", entryID, "err", err)
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) DeleteRemovedEntry(ctx context.Context, namespace string, entryID int64) error {
	defer trace.StartRegion(ctx, "metastore.sql.DeleteRemovedEntry").End()
	requireLock()
	defer releaseLock()
	var (
		entryMod = &db.Entry{ID: entryID}
	)
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := namespaceQuery(tx, namespace).Where("id = ?", entryID).First(entryMod)
		if res.Error != nil {
			return res.Error
		}

		if entryMod.RefCount != nil && *entryMod.RefCount > 0 && entryMod.IsGroup {
			s.logger.Warnf("entry %d ref_count is %d, more than 0", entryMod.ID, *entryMod.RefCount)
		}

		res = tx.Where("id = ?", entryMod.ID).Delete(&db.Entry{})
		if res.Error != nil {
			return res.Error
		}
		res = tx.Where("entry = ?", entryMod.ID).Delete(&db.EntryProperty{})
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

func (s *sqlMetaStore) ListChildren(ctx context.Context, namespace string, parentId int64) ([]*types.Child, error) {
	defer trace.StartRegion(ctx, "metastore.sql.ListEntryChildren").End()
	requireLock()
	defer releaseLock()
	var (
		models []db.Children
		result []*types.Child
	)
	res := s.WithContext(ctx).Where("parent_id = ? AND namespace = ?", parentId, namespace).Find(&models)
	if res.Error != nil {
		return nil, res.Error
	}

	for _, model := range models {
		result = append(result, model.To())
	}
	return result, nil
}

func (s *sqlMetaStore) Open(ctx context.Context, namespace string, id int64, attr types.OpenAttr) (*types.Entry, error) {
	defer trace.StartRegion(ctx, "metastore.sql.Open").End()
	requireLock()
	defer releaseLock()
	var (
		enMod   = &db.Entry{}
		nowTime = time.Now().UnixNano()
	)
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := namespaceQuery(tx, namespace).Where("id = ?", id).First(enMod)
		if res.Error != nil {
			return res.Error
		}
		if enMod.IsGroup {
			return types.ErrIsGroup
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
		return updateEntryModelWithVersion(tx, enMod)
	})
	if err != nil {
		s.logger.Errorw("open entry failed", "entry", id, "err", err)
		return nil, db.SqlError2Error(err)
	}
	return enMod.ToEntry(), nil
}

func (s *sqlMetaStore) Flush(ctx context.Context, namespace string, id int64, size int64) error {
	defer trace.StartRegion(ctx, "metastore.sql.Flush").End()
	requireLock()
	defer releaseLock()
	var (
		enMod   = &db.Entry{}
		nowTime = time.Now().UnixNano()
	)
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := namespaceQuery(tx, namespace).Where("id = ?", id).First(enMod)
		if res.Error != nil {
			return res.Error
		}
		enMod.ModifiedAt = nowTime
		enMod.ChangedAt = nowTime
		enMod.Size = &size
		return updateEntryModelWithVersion(tx, enMod)
	})
	if err != nil {
		s.logger.Errorw("open entry failed", "entry", id, "err", err)
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) MirrorEntry(ctx context.Context, namespace string, entryID int64, newName string, newParentID int64, attr types.EntryAttr) error {
	defer trace.StartRegion(ctx, "metastore.sql.MirrorEntry").End()
	requireLock()
	defer releaseLock()
	nowAt := time.Now().UnixNano()
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var (
			entryModel  = &db.Entry{}
			parentModel = &db.Entry{}
			child       = &db.Children{}
		)
		res := tx.Where("id = ?", entryID).First(entryModel)
		if res.Error != nil {
			return res.Error
		}

		res = tx.Where("parent_id = ? AND child_id = ? AND namespace = ? AND name = ?", newParentID, entryID, namespace, newName).First(child)
		if res.Error != nil && !errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return res.Error
		}

		if res.Error == nil {
			return types.ErrIsExist
		}

		oldRef := *entryModel.RefCount
		oldRef += 1
		entryModel.RefCount = &oldRef
		entryModel.ChangedAt = nowAt

		err := updateEntryModelWithVersion(tx, entryModel)
		if err != nil {
			return err
		}

		res = tx.Where("id = ?", newParentID).First(parentModel)
		if res.Error != nil {
			return res.Error
		}
		parentModel.ChangedAt = nowAt
		parentModel.ModifiedAt = nowAt
		err = updateEntryModelWithVersion(tx, parentModel)
		if err != nil {
			return err
		}

		child.Name = newName
		child.ChildID = entryID
		child.ParentID = newParentID
		child.Namespace = namespace

		res = tx.Create(child)
		if res.Error != nil {
			return res.Error
		}
		return nil
	})
	if err != nil {
		s.logger.Errorw("mirror entry failed", "entry", entryID, "parentID", newParentID, "err", err)
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) ChangeEntryParent(ctx context.Context, namespace string, targetEntryId int64, oldParentID int64, newParentId int64, oldName string, newName string, opt types.ChangeParentAttr) error {
	defer trace.StartRegion(ctx, "metastore.sql.ChangeEntryParent").End()
	requireLock()
	defer releaseLock()
	return s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var (
			enModel          = &db.Entry{ID: targetEntryId}
			srcParentEnModel = &db.Entry{}
			dstParentEnModel = &db.Entry{}
			refChild         = &db.Children{}
			nowTime          = time.Now().UnixNano()
			updateErr        error
		)
		res := namespaceQuery(tx, namespace).Where("id = ?", targetEntryId).First(enModel)
		if res.Error != nil {
			return res.Error
		}

		res = tx.Where("parent_id = ? AND child_id = ? AND name = ? AND namespace = ?", oldParentID, targetEntryId, oldName, namespace).First(refChild)
		if res.Error != nil {
			s.logger.Errorw("fetch source parent error", "entry", targetEntryId, "err", res.Error)
			return res.Error
		}

		enModel.ChangedAt = nowTime
		if newName != "" {
			enModel.Name = newName
			refChild.Name = newName
		}
		if updateErr = updateEntryModelWithVersion(tx, enModel); updateErr != nil {
			s.logger.Errorw("update target entry failed when change parent",
				"entry", targetEntryId, "srcParent", oldParentID, "dstParent", newParentId, "err", updateErr)
			return updateErr
		}

		if oldParentID == newParentId && (oldName == newName || newName == "") {
			return nil
		}

		if refChild.ParentID != newParentId {
			if refChild.Dynamic {
				return types.ErrNoAccess
			}
			refChild.ParentID = newParentId
		}
		res = tx.Updates(refChild)
		if res.Error != nil {
			return res.Error
		}

		res = namespaceQuery(tx, namespace).Where("id = ?", oldParentID).First(srcParentEnModel)
		if res.Error != nil {
			return res.Error
		}
		if enModel.IsGroup && oldParentID != newParentId {
			res = namespaceQuery(tx, namespace).Where("id = ?", newParentId).First(dstParentEnModel)
			if res.Error != nil {
				return res.Error
			}

			dstParentEnModel.ChangedAt = nowTime
			dstParentEnModel.ModifiedAt = nowTime
			dstParentRef := *dstParentEnModel.RefCount + 1
			dstParentEnModel.RefCount = &dstParentRef
			dstParentEnModel.ChangedAt = nowTime
			if updateErr = updateEntryModelWithVersion(tx, dstParentEnModel); updateErr != nil {
				s.logger.Errorw("update dst parent entry failed when change parent",
					"entry", targetEntryId, "srcParent", oldParentID, "dstParent", newParentId, "err", updateErr)
				return updateErr
			}

			srcParentRef := *srcParentEnModel.RefCount - 1
			srcParentEnModel.RefCount = &srcParentRef
		}

		srcParentEnModel.ChangedAt = nowTime
		srcParentEnModel.ModifiedAt = nowTime
		if updateErr = updateEntryModelWithVersion(tx, srcParentEnModel); updateErr != nil {
			s.logger.Errorw("update src parent entry failed when change parent",
				"entry", targetEntryId, "srcParent", oldParentID, "dstParent", newParentId, "err", updateErr)
			return updateErr
		}

		return nil
	})
}

func (s *sqlMetaStore) GetEntryProperties(ctx context.Context, namespace string, ptype types.PropertyType, id int64, data any) error {
	defer trace.StartRegion(ctx, "metastore.sql.GetEntryProperties").End()
	requireLock()
	defer releaseLock()
	var itemModel = &db.EntryProperty{}
	res := s.WithNamespace(ctx, namespace).Where("entry = ? AND type = ?", id, ptype).First(&itemModel)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			// just return
			return nil
		}
		return db.SqlError2Error(res.Error)
	}
	return json.Unmarshal(itemModel.Value, data)
}

func (s *sqlMetaStore) UpdateEntryProperties(ctx context.Context, namespace string, ptype types.PropertyType, id int64, data any) error {
	defer trace.StartRegion(ctx, "metastore.sql.UpdateEntryProperties").End()
	requireLock()
	defer releaseLock()
	var itemModel = &db.EntryProperty{
		Entry:     id,
		Type:      string(ptype),
		Namespace: namespace,
	}

	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	itemModel.Value = raw
	res := s.WithNamespace(ctx, namespace).Save(itemModel)
	if res.Error != nil {
		return db.SqlError2Error(res.Error)
	}
	return nil
}

func (s *sqlMetaStore) NextSegmentID(ctx context.Context) (int64, error) {
	defer trace.StartRegion(ctx, "metastore.sql.NextSegmentID").End()
	requireLock()
	defer releaseLock()
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
	requireLock()
	defer releaseLock()
	segments := make([]db.EntryChunk, 0)
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
			ID:      seg.ID,
			ChunkID: seg.ChunkID,
			EntryID: seg.Entry,
			Off:     seg.Off,
			Len:     seg.Len,
			State:   seg.State,
		}

	}
	return result, nil
}

func (s *sqlMetaStore) AppendSegments(ctx context.Context, seg types.ChunkSeg) (*types.Entry, error) {
	defer trace.StartRegion(ctx, "metastore.sql.AppendSegments").End()
	requireLock()
	defer releaseLock()
	var (
		enMod   = &db.Entry{ID: seg.EntryID}
		nowTime = time.Now().UnixNano()
		err     error
	)
	err = s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(enMod)
		if res.Error != nil {
			return res.Error
		}

		res = tx.Create(&db.EntryChunk{
			ID:       seg.ID,
			Entry:    seg.EntryID,
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
		enMod.ChangedAt = nowTime

		if writeBackErr := updateEntryModelWithVersion(tx, enMod); writeBackErr != nil {
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
	requireLock()
	defer releaseLock()
	res := s.WithContext(ctx).Delete(&db.EntryChunk{ID: segID})
	return db.SqlError2Error(res.Error)
}

func (s *sqlMetaStore) ListTask(ctx context.Context, taskID string, filter types.ScheduledTaskFilter) ([]*types.ScheduledTask, error) {
	defer trace.StartRegion(ctx, "metastore.sql.ListTask").End()
	requireLock()
	defer releaseLock()
	tasks := make([]db.ScheduledTask, 0)
	query := s.WithContext(ctx).Where("task_id = ?", taskID)

	if filter.RefID != 0 && filter.RefType != "" {
		query = query.Where("ref_type = ? AND ref_id = ?", filter.RefType, filter.RefID)
	}

	if len(filter.Status) > 0 {
		query = query.Where("status IN ?", filter.Status)
	}
	if page := types.GetPagination(ctx); page != nil {
		query = query.Offset(page.Offset()).Limit(page.Limit())
	}

	res := query.Order("created_time").Find(&tasks)
	if res.Error != nil {
		return nil, db.SqlError2Error(res.Error)
	}

	result := make([]*types.ScheduledTask, len(tasks))
	for i, t := range tasks {
		evt := types.Event{}
		_ = json.Unmarshal(t.Event, &evt)
		result[i] = &types.ScheduledTask{
			ID:             t.ID,
			Namespace:      t.Namespace,
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

func (s *sqlMetaStore) SaveTask(ctx context.Context, task *types.ScheduledTask) error {
	defer trace.StartRegion(ctx, "metastore.sql.SaveTask").End()
	requireLock()
	defer releaseLock()
	t := &db.ScheduledTask{
		ID:             task.ID,
		TaskID:         task.TaskID,
		Status:         task.Status,
		RefID:          task.RefID,
		RefType:        task.RefType,
		Result:         task.Result,
		Namespace:      task.Namespace,
		CreatedTime:    task.CreatedTime,
		ExecutionTime:  task.ExecutionTime,
		ExpirationTime: task.ExpirationTime,
	}
	rawEvt, err := json.Marshal(task.Event)
	if err != nil {
		return err
	}
	t.Event = rawEvt
	if task.ID == 0 {
		res := s.WithContext(ctx).Create(t)
		if res.Error != nil {
			return db.SqlError2Error(res.Error)
		}
		task.ID = t.ID
		return nil
	}
	res := s.WithContext(ctx).Save(t)
	if res.Error != nil {
		return db.SqlError2Error(res.Error)
	}
	return nil
}

func (s *sqlMetaStore) DeleteFinishedTask(ctx context.Context, aliveTime time.Duration) error {
	defer trace.StartRegion(ctx, "metastore.sql.DeleteFinishedTask").End()
	requireLock()
	defer releaseLock()
	res := s.WithContext(ctx).
		Where("status IN ? AND created_time < ?",
			[]string{types.ScheduledTaskFinish, types.ScheduledTaskSucceed, types.ScheduledTaskFailed},
			time.Now().Add(-1*aliveTime)).
		Delete(&db.ScheduledTask{})
	if res.Error != nil {
		return db.SqlError2Error(res.Error)
	}
	return nil
}

func (s *sqlMetaStore) GetWorkflow(ctx context.Context, namespace string, wfID string) (*types.Workflow, error) {
	defer trace.StartRegion(ctx, "metastore.sql.GetWorkflow").End()
	requireLock()
	defer releaseLock()
	wf := &db.Workflow{}
	res := s.WithNamespace(ctx, namespace).Where("id = ?", wfID).First(wf)
	if res.Error != nil {
		return nil, db.SqlError2Error(res.Error)
	}
	result, err := wf.To()
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *sqlMetaStore) ListAllNamespaceWorkflows(ctx context.Context) ([]*types.Workflow, error) {
	requireLock()
	defer releaseLock()
	return s.listWorkflow(ctx, types.AllNamespace)
}

func (s *sqlMetaStore) ListAllNamespaceWorkflowJobs(ctx context.Context, filter types.JobFilter) ([]*types.WorkflowJob, error) {
	requireLock()
	defer releaseLock()
	return s.listWorkflowJobs(ctx, types.AllNamespace, filter)
}

func (s *sqlMetaStore) ListWorkflows(ctx context.Context, namespace string) ([]*types.Workflow, error) {
	requireLock()
	defer releaseLock()
	return s.listWorkflow(ctx, namespace)
}

func (s *sqlMetaStore) listWorkflow(ctx context.Context, namespace string) ([]*types.Workflow, error) {
	defer trace.StartRegion(ctx, "metastore.sql.ListWorkflow").End()
	wfList := make([]db.Workflow, 0)
	res := s.WithContext(ctx).Order("created_at desc")
	if namespace != types.AllNamespace {
		res = res.Where("namespace = ?", namespace)
	}
	if page := types.GetPagination(ctx); page != nil {
		res = res.Offset(page.Offset()).Limit(page.Limit())
	}
	res = res.Find(&wfList)
	if res.Error != nil {
		return nil, db.SqlError2Error(res.Error)
	}

	var (
		result = make([]*types.Workflow, len(wfList))
		err    error
	)
	for i, wf := range wfList {
		result[i], err = wf.To()
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (s *sqlMetaStore) DeleteWorkflow(ctx context.Context, namespace string, wfID string) error {
	defer trace.StartRegion(ctx, "metastore.sql.DeleteWorkflow").End()
	requireLock()
	defer releaseLock()
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := namespaceQuery(tx, namespace).Where("id = ?", wfID).Delete(&db.Workflow{})
		if res.Error != nil {
			return res.Error
		}
		res = namespaceQuery(tx, namespace).Where("workflow = ?", wfID).Delete(&db.WorkflowJob{})
		if res.Error != nil {
			return res.Error
		}
		return nil
	})
	if err != nil {
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) GetWorkflowJob(ctx context.Context, namespace string, jobID string) (*types.WorkflowJob, error) {
	defer trace.StartRegion(ctx, "metastore.sql.GetWorkflowJob").End()
	requireLock()
	defer releaseLock()
	jobMod := &db.WorkflowJob{}
	res := s.WithNamespace(ctx, namespace).Where("id = ?", jobID).First(jobMod)
	if res.Error != nil {
		return nil, db.SqlError2Error(res.Error)
	}
	return jobMod.To()
}
func (s *sqlMetaStore) ListWorkflowJobs(ctx context.Context, namespace string, filter types.JobFilter) ([]*types.WorkflowJob, error) {
	requireLock()
	defer releaseLock()
	return s.listWorkflowJobs(ctx, namespace, filter)
}

func (s *sqlMetaStore) listWorkflowJobs(ctx context.Context, namespace string, filter types.JobFilter) ([]*types.WorkflowJob, error) {
	defer trace.StartRegion(ctx, "metastore.sql.ListWorkflowJob").End()
	jobList := make([]db.WorkflowJob, 0)
	query := s.WithContext(ctx)
	if page := types.GetPagination(ctx); page != nil {
		query = query.Offset(page.Offset()).Limit(page.Limit())
	}

	if namespace != types.AllNamespace {
		query = query.Where("namespace = ?", namespace)
	}

	if filter.JobID != "" {
		query = query.Where("id = ?", filter.JobID)
	}
	if filter.WorkFlowID != "" {
		query = query.Where("workflow = ?", filter.WorkFlowID)
	}
	if filter.QueueName != "" {
		query = query.Where("queue_name = ?", filter.QueueName)
	}
	if filter.Executor != "" {
		query = query.Where("executor = ?", filter.Executor)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.TargetEntry != 0 {
		query = query.Where("target_entry = ?", filter.TargetEntry)
	}

	res := query.Order("created_at desc").Find(&jobList)
	if res.Error != nil {
		return nil, db.SqlError2Error(res.Error)
	}

	var (
		result = make([]*types.WorkflowJob, len(jobList))
		err    error
	)
	for i, wf := range jobList {
		result[i], err = wf.To()
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (s *sqlMetaStore) SaveWorkflow(ctx context.Context, namespace string, wf *types.Workflow) error {
	defer trace.StartRegion(ctx, "metastore.sql.SaveWorkflow").End()
	requireLock()
	defer releaseLock()
	if namespace == "" {
		return types.ErrNoNamespace
	}
	wf.UpdatedAt = time.Now()
	wf.Namespace = namespace
	defer trace.StartRegion(ctx, "metastore.sql.SaveWorkflow").End()
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var loadErr error
		dbMod := &db.Workflow{}
		res := namespaceQuery(tx, namespace).Where("id = ?", wf.Id).First(dbMod)
		if res.Error != nil {
			if errors.Is(res.Error, gorm.ErrRecordNotFound) {
				// create
				dbMod, loadErr = dbMod.From(wf)
				if loadErr != nil {
					return loadErr
				}
				res = tx.Create(dbMod)
				return res.Error
			}
			return res.Error
		}

		// update
		if dbMod, loadErr = dbMod.From(wf); loadErr != nil {
			return loadErr
		}
		res = tx.Save(dbMod)
		return res.Error
	})
	if err != nil {
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) SaveWorkflowJob(ctx context.Context, namespace string, job *types.WorkflowJob) error {
	defer trace.StartRegion(ctx, "metastore.sql.SaveWorkflowJob").End()
	requireLock()
	defer releaseLock()
	if namespace == "" {
		return types.ErrNoNamespace
	}
	job.UpdatedAt = time.Now()
	job.Namespace = namespace
	defer trace.StartRegion(ctx, "metastore.sql.SaveWorkflowJob").End()
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		wfModel := &db.Workflow{}
		res := namespaceQuery(tx, namespace).Where("id = ?", job.Workflow).First(wfModel)
		if res.Error != nil {
			return res.Error
		}

		jobModel := &db.WorkflowJob{}
		var loadErr error
		res = namespaceQuery(tx, namespace).Where("id = ?", job.Id).First(jobModel)
		if res.Error != nil {
			if errors.Is(res.Error, gorm.ErrRecordNotFound) {
				if jobModel, loadErr = jobModel.From(job); loadErr != nil {
					return loadErr
				}
				res = tx.Create(jobModel)
				return res.Error
			}
			return res.Error
		}

		if jobModel, loadErr = jobModel.From(job); loadErr != nil {
			return loadErr
		}
		res = tx.Save(jobModel)
		if res.Error != nil {
			return res.Error
		}

		wfModel.LastTriggeredAt = job.UpdatedAt
		res = tx.Save(wfModel)
		if res.Error != nil {
			return res.Error
		}
		return nil
	})
	if err != nil {
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) DeleteWorkflowJobs(ctx context.Context, wfJobID ...string) error {
	defer trace.StartRegion(ctx, "metastore.sql.DeleteWorkflowJob").End()
	requireLock()
	defer releaseLock()
	res := s.WithContext(ctx).Where("id IN ?", wfJobID).Delete(&db.WorkflowJob{})
	if res.Error != nil {
		return db.SqlError2Error(res.Error)
	}
	return nil
}

func (s *sqlMetaStore) LoadWorkflowContext(ctx context.Context, namespace, source, group, key string, data any) error {
	record := db.WorkflowContext{}
	res := s.WithNamespace(ctx, namespace).Where("source = ? AND group = ? AND key = ?", source, group, key).First(&record)
	if res.Error != nil && !errors.Is(res.Error, gorm.ErrRecordNotFound) {
		return db.SqlError2Error(res.Error)
	}

	if record.Value == "" {
		return nil
	}
	return json.Unmarshal([]byte(record.Value), data)
}

func (s *sqlMetaStore) SaveWorkflowContext(ctx context.Context, namespace, source, group, key string, data any) error {
	record := db.WorkflowContext{
		Source:    source,
		Group:     group,
		Key:       key,
		Namespace: namespace,
		UpdatedAt: time.Now(),
	}

	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	record.Value = string(raw)

	res := s.WithContext(ctx).Save(&record)
	if res.Error != nil {
		return db.SqlError2Error(res.Error)
	}
	return nil
}

func (s *sqlMetaStore) ListNotifications(ctx context.Context, namespace string) ([]types.Notification, error) {
	defer trace.StartRegion(ctx, "metastore.sql.ListNotifications").End()
	requireLock()
	defer releaseLock()
	noList := make([]db.Notification, 0)
	res := s.WithNamespace(ctx, namespace).Order("time desc").Find(&noList)
	if res.Error != nil {
		return nil, db.SqlError2Error(res.Error)
	}

	result := make([]types.Notification, len(noList))
	for i, no := range noList {
		result[i] = types.Notification{
			ID:        no.ID,
			Namespace: no.Namespace,
			Title:     no.Title,
			Message:   no.Message,
			Type:      no.Type,
			Source:    no.Source,
			Action:    no.Action,
			Status:    no.Status,
			Time:      no.Time,
		}
	}
	return result, nil
}

func (s *sqlMetaStore) RecordNotification(ctx context.Context, namespace string, nid string, no types.Notification) error {
	defer trace.StartRegion(ctx, "metastore.sql.RecordNotification").End()
	requireLock()
	defer releaseLock()
	model := db.Notification{
		ID:        nid,
		Namespace: namespace,
		Title:     no.Title,
		Message:   no.Message,
		Type:      no.Type,
		Source:    no.Source,
		Action:    no.Action,
		Status:    no.Status,
		Time:      no.Time,
	}
	res := s.WithContext(ctx).Create(model)
	if res.Error != nil {
		return db.SqlError2Error(res.Error)
	}
	return nil
}

func (s *sqlMetaStore) UpdateNotificationStatus(ctx context.Context, namespace, nid, status string) error {
	defer trace.StartRegion(ctx, "metastore.sql.UpdateNotificationStatus").End()
	requireLock()
	defer releaseLock()
	res := s.WithNamespace(ctx, namespace).Model(&db.Notification{}).Where("id = ?", nid).UpdateColumn("status", status)
	if res.Error != nil {
		return db.SqlError2Error(res.Error)
	}
	return nil
}

func (s *sqlMetaStore) WithNamespace(ctx context.Context, namespace string) *gorm.DB {
	return s.WithContext(ctx).Where("namespace = ?", namespace)
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

	return buildSqlMetaStore(dbEntity)
}

func updateEntryModelWithVersion(tx *gorm.DB, entryMod *db.Entry) error {
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

func namespaceQuery(tx *gorm.DB, namespace string) *gorm.DB {
	return tx.Where("namespace = ?", namespace)
}
