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

package storage

import (
	"context"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/storage/db"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"sync"
)

const (
	SqliteMeta = "sqlite"
)

type sqliteMetaStore struct {
	dbStore *sqlMetaStore
	mux     sync.RWMutex
}

var _ Meta = &sqliteMetaStore{}

func (s *sqliteMetaStore) GetObject(ctx context.Context, id int64) (*types.Object, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.dbStore.GetObject(ctx, id)
}

func (s *sqliteMetaStore) GetObjectExtendData(ctx context.Context, obj *types.Object) error {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.dbStore.GetObjectExtendData(ctx, obj)
}

func (s *sqliteMetaStore) ListObjects(ctx context.Context, filter types.Filter) ([]*types.Object, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.dbStore.ListObjects(ctx, filter)
}

func (s *sqliteMetaStore) SaveObject(ctx context.Context, parent, obj *types.Object) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.SaveObject(ctx, parent, obj)
}

func (s *sqliteMetaStore) DestroyObject(ctx context.Context, src, parent, obj *types.Object) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.DestroyObject(ctx, src, parent, obj)
}

func (s *sqliteMetaStore) ListChildren(ctx context.Context, obj *types.Object) (Iterator, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.dbStore.ListChildren(ctx, obj)
}

func (s *sqliteMetaStore) ChangeParent(ctx context.Context, srcParent, dstParent, obj *types.Object, opt types.ChangeParentOption) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.ChangeParent(ctx, srcParent, dstParent, obj, opt)
}

func (s *sqliteMetaStore) MirrorObject(ctx context.Context, srcObj, dstParent, object *types.Object) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.MirrorObject(ctx, srcObj, dstParent, object)
}

func (s *sqliteMetaStore) NextSegmentID(ctx context.Context) (int64, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.NextSegmentID(ctx)
}

func (s *sqliteMetaStore) ListSegments(ctx context.Context, oid, chunkID int64) ([]types.ChunkSeg, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.ListSegments(ctx, oid, chunkID)
}

func (s *sqliteMetaStore) AppendSegments(ctx context.Context, seg types.ChunkSeg) (*types.Object, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.AppendSegments(ctx, seg)
}

func (s *sqliteMetaStore) PluginRecorder(plugin types.PlugScope) PluginRecorder {
	return &sqlitePluginRecorder{
		sqlPluginRecorder: s,
		plugin:            plugin,
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

func newSqliteMetaStore(meta config.Meta) (*sqliteMetaStore, error) {
	dbObj, err := gorm.Open(sqlite.Open(meta.Path), &gorm.Config{Logger: db.NewDbLogger()})
	if err != nil {
		return nil, err
	}

	dbConn, err := dbObj.DB()
	if err != nil {
		return nil, err
	}

	if err = dbConn.Ping(); err != nil {
		return nil, err
	}

	dbEnt, err := db.NewDbEntity(dbObj)
	if err != nil {
		return nil, err
	}

	return &sqliteMetaStore{dbStore: buildSqlMetaStore(dbEnt)}, nil
}

type sqlMetaStore struct {
	dbEntity *db.Entity
	logger   *zap.SugaredLogger
}

var _ Meta = &sqlMetaStore{}

func buildSqlMetaStore(entity *db.Entity) *sqlMetaStore {
	return &sqlMetaStore{
		dbEntity: entity,
		logger:   logger.NewLogger("dbStore"),
	}
}

func (s *sqlMetaStore) GetObject(ctx context.Context, id int64) (*types.Object, error) {
	defer utils.TraceRegion(ctx, "sql.getobject")()
	obj, err := s.dbEntity.GetObjectByID(ctx, id)
	if err != nil {
		s.logger.Errorw("query object by id failed", "id", id, "err", err.Error())
		return nil, db.SqlError2Error(err)
	}
	return obj, nil
}

func (s *sqlMetaStore) GetObjectExtendData(ctx context.Context, obj *types.Object) error {
	defer utils.TraceRegion(ctx, "sql.getextenddata")()
	if err := s.dbEntity.GetObjectExtendData(ctx, obj); err != nil {
		s.logger.Errorw("query object extend data failed", "id", obj.ID, "err", err.Error())
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) ListObjects(ctx context.Context, filter types.Filter) ([]*types.Object, error) {
	defer utils.TraceRegion(ctx, "sql.listobject")()
	objList, err := s.dbEntity.ListObjectChildren(ctx, filter)
	if err != nil {
		s.logger.Errorw("list object failed", "filter", filter, "err", err.Error())
		return nil, db.SqlError2Error(err)
	}
	return objList, nil
}

func (s *sqlMetaStore) SaveObject(ctx context.Context, parent, obj *types.Object) error {
	defer utils.TraceRegion(ctx, "sql.saveobject")()
	if err := s.dbEntity.SaveObject(ctx, parent, obj); err != nil {
		s.logger.Errorw("save object failed", "id", obj.ID, "err", err.Error())
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) DestroyObject(ctx context.Context, src, parent, obj *types.Object) error {
	defer utils.TraceRegion(ctx, "sql.destroyobject")()
	err := s.dbEntity.DeleteObject(ctx, src, parent, obj)
	if err != nil {
		s.logger.Errorw("destroy object failed", "id", obj.ID, "err", err.Error())
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) ListChildren(ctx context.Context, obj *types.Object) (Iterator, error) {
	defer utils.TraceRegion(ctx, "sql.listchildren")()
	children, err := s.dbEntity.ListObjectChildren(ctx, types.Filter{ParentID: obj.ID})
	if err != nil {
		s.logger.Errorw("list object children failed", "id", obj.ID, "err", err.Error())
		return nil, db.SqlError2Error(err)
	}
	return &iterator{objects: children}, nil
}

func (s *sqlMetaStore) ChangeParent(ctx context.Context, srcParent, dstParent, obj *types.Object, opt types.ChangeParentOption) error {
	defer utils.TraceRegion(ctx, "sql.changeparent")()
	obj.ParentID = dstParent.ID
	err := s.dbEntity.SaveChangeParentObject(ctx, srcParent, dstParent, obj, opt)
	if err != nil {
		s.logger.Errorw("change object parent failed", "id", obj.ID, "err", err.Error())
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) MirrorObject(ctx context.Context, srcObj, dstParent, object *types.Object) error {
	defer utils.TraceRegion(ctx, "sql.mirrorobject")()
	err := s.dbEntity.SaveMirroredObject(ctx, srcObj, dstParent, object)
	if err != nil {
		s.logger.Errorw("mirror object failed", "id", object.ID, "err", err.Error())
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) NextSegmentID(ctx context.Context) (int64, error) {
	defer utils.TraceRegion(ctx, "sql.nextchunkid")()
	return s.dbEntity.NextSegmentID(ctx)
}

func (s *sqlMetaStore) ListSegments(ctx context.Context, oid, chunkID int64) ([]types.ChunkSeg, error) {
	defer utils.TraceRegion(ctx, "sql.listsegment")()
	return s.dbEntity.ListChunkSegments(ctx, oid, chunkID)
}

func (s *sqlMetaStore) AppendSegments(ctx context.Context, seg types.ChunkSeg) (*types.Object, error) {
	defer utils.TraceRegion(ctx, "sql.appendsegment")()
	return s.dbEntity.InsertChunkSegment(ctx, seg)
}

func (s *sqlMetaStore) PluginRecorder(plugin types.PlugScope) PluginRecorder {
	return &sqlitePluginRecorder{
		sqlPluginRecorder: s,
		plugin:            plugin,
	}
}

func (s *sqlMetaStore) getPluginRecord(ctx context.Context, plugin types.PlugScope, rid string, record interface{}) error {
	return s.dbEntity.GetPluginRecord(ctx, plugin, rid, record)
}

func (s *sqlMetaStore) listPluginRecords(ctx context.Context, plugin types.PlugScope, groupId string) ([]string, error) {
	return s.dbEntity.ListPluginRecords(ctx, plugin, groupId)
}

func (s *sqlMetaStore) savePluginRecord(ctx context.Context, plugin types.PlugScope, groupId, rid string, record interface{}) error {
	return s.dbEntity.SavePluginRecord(ctx, plugin, groupId, rid, record)
}

func (s *sqlMetaStore) deletePluginRecord(ctx context.Context, plugin types.PlugScope, rid string) error {
	return s.dbEntity.DeletePluginRecord(ctx, plugin, rid)
}

type sqlPluginRecorder interface {
	getPluginRecord(ctx context.Context, plugin types.PlugScope, rid string, record interface{}) error
	listPluginRecords(ctx context.Context, plugin types.PlugScope, groupId string) ([]string, error)
	savePluginRecord(ctx context.Context, plugin types.PlugScope, groupId, rid string, record interface{}) error
	deletePluginRecord(ctx context.Context, plugin types.PlugScope, rid string) error
}

type sqlitePluginRecorder struct {
	sqlPluginRecorder
	plugin types.PlugScope
}

func (s *sqlitePluginRecorder) GetRecord(ctx context.Context, rid string, record interface{}) error {
	return db.SqlError2Error(s.getPluginRecord(ctx, s.plugin, rid, record))
}

func (s *sqlitePluginRecorder) ListRecords(ctx context.Context, groupId string) ([]string, error) {
	result, err := s.listPluginRecords(ctx, s.plugin, groupId)
	if err != nil {
		return nil, db.SqlError2Error(err)
	}
	return result, nil
}

func (s *sqlitePluginRecorder) SaveRecord(ctx context.Context, groupId, rid string, record interface{}) error {
	return db.SqlError2Error(s.savePluginRecord(ctx, s.plugin, groupId, rid, record))
}

func (s *sqlitePluginRecorder) DeleteRecord(ctx context.Context, rid string) error {
	return db.SqlError2Error(s.deletePluginRecord(ctx, s.plugin, rid))
}
