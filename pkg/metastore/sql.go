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
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/metastore/db"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"runtime/trace"
	"sync"
	"time"
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

func (s *sqliteMetaStore) SaveObjects(ctx context.Context, objList ...*types.Object) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.SaveObjects(ctx, objList...)
}

func (s *sqliteMetaStore) DestroyObject(ctx context.Context, src, obj *types.Object) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.DestroyObject(ctx, src, obj)
}

func (s *sqliteMetaStore) ListChildren(ctx context.Context, parentId int64) (Iterator, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.dbStore.ListChildren(ctx, parentId)
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

func (s *sqliteMetaStore) ListSegments(ctx context.Context, oid, chunkID int64, allChunk bool) ([]types.ChunkSeg, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.ListSegments(ctx, oid, chunkID, allChunk)
}

func (s *sqliteMetaStore) AppendSegments(ctx context.Context, seg types.ChunkSeg) (*types.Object, error) {
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
	return s.RecordNotification(ctx, nid, no)
}

func (s *sqliteMetaStore) UpdateNotificationStatus(ctx context.Context, nid, status string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.UpdateNotificationStatus(ctx, nid, status)
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
	return &sqlMetaStore{dbEntity: entity, logger: logger.NewLogger("dbStore")}
}

func (s *sqlMetaStore) SystemInfo(ctx context.Context) (*types.SystemInfo, error) {
	return s.dbEntity.SystemInfo(ctx)
}

func (s *sqlMetaStore) GetObject(ctx context.Context, id int64) (*types.Object, error) {
	defer trace.StartRegion(ctx, "metastore.sql.GetObject").End()
	obj, err := s.dbEntity.GetObjectByID(ctx, id)
	if err != nil {
		s.logger.Errorw("query object by id failed", "id", id, "err", err.Error())
		return nil, db.SqlError2Error(err)
	}
	return obj, nil
}

func (s *sqlMetaStore) GetObjectExtendData(ctx context.Context, obj *types.Object) error {
	defer trace.StartRegion(ctx, "metastore.sql.GetObjectExtendData").End()
	if err := s.dbEntity.GetObjectExtendData(ctx, obj); err != nil {
		s.logger.Errorw("query object extend data failed", "id", obj.ID, "err", err.Error())
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) ListObjects(ctx context.Context, filter types.Filter) ([]*types.Object, error) {
	defer trace.StartRegion(ctx, "metastore.sql.ListObjects").End()
	objList, err := s.dbEntity.ListObjectChildren(ctx, filter)
	if err != nil {
		s.logger.Errorw("list object failed", "filter", filter, "err", err.Error())
		return nil, db.SqlError2Error(err)
	}
	return objList, nil
}

func (s *sqlMetaStore) SaveObjects(ctx context.Context, objList ...*types.Object) error {
	defer trace.StartRegion(ctx, "metastore.sql.SaveMirroredObject").End()
	if err := s.dbEntity.SaveObjects(ctx, objList...); err != nil {
		s.logger.Errorw("save objects failed", "err", err.Error())
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) DestroyObject(ctx context.Context, src, obj *types.Object) error {
	defer trace.StartRegion(ctx, "metastore.sql.DestroyObject").End()
	err := s.dbEntity.DeleteObject(ctx, src, obj)
	if err != nil {
		s.logger.Errorw("destroy object failed", "id", obj.ID, "err", err.Error())
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) ListChildren(ctx context.Context, parentId int64) (Iterator, error) {
	defer trace.StartRegion(ctx, "metastore.sql.ListChildren").End()
	children, err := s.dbEntity.ListObjectChildren(ctx, types.Filter{ParentID: parentId})
	if err != nil {
		s.logger.Errorw("list object children failed", "id", parentId, "err", err.Error())
		return nil, db.SqlError2Error(err)
	}
	return &iterator{objects: children}, nil
}

func (s *sqlMetaStore) ChangeParent(ctx context.Context, srcParent, dstParent, obj *types.Object, opt types.ChangeParentOption) error {
	defer trace.StartRegion(ctx, "metastore.sql.ChangeParent").End()
	obj.ParentID = dstParent.ID
	err := s.dbEntity.SaveChangeParentObject(ctx, srcParent, dstParent, obj, opt)
	if err != nil {
		s.logger.Errorw("change object parent failed", "id", obj.ID, "err", err.Error())
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) MirrorObject(ctx context.Context, srcObj, dstParent, object *types.Object) error {
	defer trace.StartRegion(ctx, "metastore.sql.MirrorObject").End()
	err := s.dbEntity.SaveMirroredObject(ctx, srcObj, dstParent, object)
	if err != nil {
		s.logger.Errorw("mirror object failed", "id", object.ID, "err", err.Error())
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) NextSegmentID(ctx context.Context) (int64, error) {
	defer trace.StartRegion(ctx, "metastore.sql.NextSegmentID").End()
	return s.dbEntity.NextSegmentID(ctx)
}

func (s *sqlMetaStore) ListSegments(ctx context.Context, oid, chunkID int64, allChunk bool) ([]types.ChunkSeg, error) {
	defer trace.StartRegion(ctx, "metastore.sql.ListSegments").End()
	return s.dbEntity.ListChunkSegments(ctx, oid, chunkID, allChunk)
}

func (s *sqlMetaStore) AppendSegments(ctx context.Context, seg types.ChunkSeg) (*types.Object, error) {
	defer trace.StartRegion(ctx, "metastore.sql.AppendSegments").End()
	return s.dbEntity.InsertChunkSegment(ctx, seg)
}

func (s *sqlMetaStore) DeleteSegment(ctx context.Context, segID int64) error {
	defer trace.StartRegion(ctx, "metastore.sql.DeleteSegment").End()
	return db.SqlError2Error(s.dbEntity.DeleteChunkSegment(ctx, segID))
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
	err := s.dbEntity.SaveWorkflow(ctx, wf)
	if err != nil {
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) SaveWorkflowJob(ctx context.Context, wf *types.WorkflowJob) error {
	defer trace.StartRegion(ctx, "metastore.sql.SaveWorkflowJob").End()
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
	//TODO implement me
	panic("implement me")
}

func (s *sqlMetaStore) RecordNotification(ctx context.Context, nid string, no types.Notification) error {
	//TODO implement me
	panic("implement me")
}

func (s *sqlMetaStore) UpdateNotificationStatus(ctx context.Context, nid, status string) error {
	//TODO implement me
	panic("implement me")
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

func newPostgresMetaStore(meta config.Meta) (*sqlMetaStore, error) {
	dbObj, err := gorm.Open(postgres.Open(meta.DSN), &gorm.Config{Logger: db.NewDbLogger()})
	if err != nil {
		panic(err)
	}

	dbConn, err := dbObj.DB()
	if err != nil {
		return nil, err
	}

	dbConn.SetMaxIdleConns(5)
	dbConn.SetMaxOpenConns(50)
	dbConn.SetConnMaxLifetime(time.Hour)

	if err = dbConn.Ping(); err != nil {
		return nil, err
	}

	dbEnt, err := db.NewDbEntity(dbObj)
	if err != nil {
		return nil, err
	}

	return buildSqlMetaStore(dbEnt), nil
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
