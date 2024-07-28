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
	"fmt"
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

var initMetrics sync.Once

type sqliteMetaStore struct {
	dbStore *sqlMetaStore
	mux     sync.Mutex
}

var _ Meta = &sqliteMetaStore{}

func (s *sqliteMetaStore) GetAccessToken(ctx context.Context, tokenKey string, secretKey string) (*types.AccessToken, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.GetAccessToken(ctx, tokenKey, secretKey)
}

func (s *sqliteMetaStore) CreateAccessToken(ctx context.Context, token *types.AccessToken) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.CreateAccessToken(ctx, token)
}

func (s *sqliteMetaStore) UpdateAccessTokenCerts(ctx context.Context, token *types.AccessToken) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.UpdateAccessTokenCerts(ctx, token)
}

func (s *sqliteMetaStore) RevokeAccessToken(ctx context.Context, tokenKey string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.RevokeAccessToken(ctx, tokenKey)
}

func (s *sqliteMetaStore) SystemInfo(ctx context.Context) (*types.SystemInfo, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.SystemInfo(ctx)
}

func (s *sqliteMetaStore) GetConfigValue(ctx context.Context, group string, name string) (string, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.GetConfigValue(ctx, group, name)
}

func (s *sqliteMetaStore) SetConfigValue(ctx context.Context, group, name, value string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.SetConfigValue(ctx, group, name, value)
}

func (s *sqliteMetaStore) GetEntry(ctx context.Context, id int64) (*types.Metadata, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.GetEntry(ctx, id)
}

func (s *sqliteMetaStore) FindEntry(ctx context.Context, parentID int64, name string) (*types.Metadata, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.FindEntry(ctx, parentID, name)
}

func (s *sqliteMetaStore) CreateEntry(ctx context.Context, parentID int64, newEntry *types.Metadata, ed *types.ExtendData) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.CreateEntry(ctx, parentID, newEntry, ed)
}

func (s *sqliteMetaStore) RemoveEntry(ctx context.Context, parentID, entryID int64) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.RemoveEntry(ctx, parentID, entryID)
}

func (s *sqliteMetaStore) SaveEntryUri(ctx context.Context, entryUri *types.EntryUri) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.SaveEntryUri(ctx, entryUri)
}

func (s *sqliteMetaStore) GetEntryUri(ctx context.Context, uri string) (*types.EntryUri, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.GetEntryUri(ctx, uri)
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

func (s *sqliteMetaStore) ListEntryChildren(ctx context.Context, parentId int64, order *types.EntryOrder, filters ...types.Filter) (EntryIterator, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.ListEntryChildren(ctx, parentId, order, filters...)
}

func (s *sqliteMetaStore) ListDocumentGroups(ctx context.Context, parentId int64, filter types.DocFilter) (EntryIterator, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.ListDocumentGroups(ctx, parentId, filter)
}

func (s *sqliteMetaStore) FilterEntries(ctx context.Context, filter types.Filter) (EntryIterator, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
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
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.GetEntryExtendData(ctx, id)
}

func (s *sqliteMetaStore) UpdateEntryExtendData(ctx context.Context, id int64, ed types.ExtendData) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.UpdateEntryExtendData(ctx, id, ed)
}

func (s *sqliteMetaStore) GetEntryLabels(ctx context.Context, id int64) (types.Labels, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.GetEntryLabels(ctx, id)
}

func (s *sqliteMetaStore) UpdateEntryLabels(ctx context.Context, id int64, labels types.Labels) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.UpdateEntryLabels(ctx, id, labels)
}

func (s *sqliteMetaStore) ListEntryProperties(ctx context.Context, id int64) (types.Properties, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.ListEntryProperties(ctx, id)
}

func (s *sqliteMetaStore) GetEntryProperty(ctx context.Context, id int64, key string) (types.PropertyItem, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.GetEntryProperty(ctx, id, key)
}

func (s *sqliteMetaStore) AddEntryProperty(ctx context.Context, id int64, key string, item types.PropertyItem) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.AddEntryProperty(ctx, id, key, item)
}

func (s *sqliteMetaStore) RemoveEntryProperty(ctx context.Context, id int64, key string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.RemoveEntryProperty(ctx, id, key)
}

func (s *sqliteMetaStore) UpdateEntryProperties(ctx context.Context, id int64, properties types.Properties) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.UpdateEntryProperties(ctx, id, properties)
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

func (s *sqliteMetaStore) GetWorkflow(ctx context.Context, wfID string) (*types.Workflow, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.GetWorkflow(ctx, wfID)
}

func (s *sqliteMetaStore) ListGlobalWorkflow(ctx context.Context) ([]*types.Workflow, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.ListGlobalWorkflow(ctx)
}

func (s *sqliteMetaStore) ListWorkflow(ctx context.Context) ([]*types.Workflow, error) {
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

func (s *sqliteMetaStore) SaveWorkflow(ctx context.Context, wf *types.Workflow) error {
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

func (s *sqliteMetaStore) RecordEvents(ctx context.Context, events []types.Event) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.RecordEvents(ctx, events)
}

func (s *sqliteMetaStore) ListEvents(ctx context.Context, filter types.EventFilter) ([]types.Event, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.ListEvents(ctx, filter)
}

func (s *sqliteMetaStore) DeviceSync(ctx context.Context, deviceID string, syncedSequence int64) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.DeviceSync(ctx, deviceID, syncedSequence)
}

func (s *sqliteMetaStore) SaveDocument(ctx context.Context, doc *types.Document) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.SaveDocument(ctx, doc)
}

func (s *sqliteMetaStore) ListDocument(ctx context.Context, filter types.DocFilter, order *types.DocumentOrder) ([]*types.Document, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.ListDocument(ctx, filter, order)
}

func (s *sqliteMetaStore) GetDocument(ctx context.Context, id int64) (*types.Document, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.GetDocument(ctx, id)
}

func (s *sqliteMetaStore) GetDocumentByName(ctx context.Context, name string) (*types.Document, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.GetDocumentByName(ctx, name)
}

func (s *sqliteMetaStore) GetDocumentByEntryId(ctx context.Context, oid int64) (*types.Document, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.GetDocumentByEntryId(ctx, oid)
}

func (s *sqliteMetaStore) DeleteDocument(ctx context.Context, id int64) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.DeleteDocument(ctx, id)
}

func (s *sqliteMetaStore) ListFridayAccount(ctx context.Context, refId int64) ([]*types.FridayAccount, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.ListFridayAccount(ctx, refId)
}

func (s *sqliteMetaStore) CreateFridayAccount(ctx context.Context, account *types.FridayAccount) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.CreateFridayAccount(ctx, account)
}

func (s *sqliteMetaStore) SaveRoom(ctx context.Context, room *types.Room) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.SaveRoom(ctx, room)
}

func (s *sqliteMetaStore) GetRoom(ctx context.Context, id int64) (*types.Room, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.GetRoom(ctx, id)
}

func (s *sqliteMetaStore) FindRoom(ctx context.Context, entryId int64) (*types.Room, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.FindRoom(ctx, entryId)
}

func (s *sqliteMetaStore) DeleteRoom(ctx context.Context, id int64) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.DeleteRoom(ctx, id)
}

func (s *sqliteMetaStore) ListRooms(ctx context.Context, entryId int64) ([]*types.Room, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.ListRooms(ctx, entryId)
}

func (s *sqliteMetaStore) ListRoomMessage(ctx context.Context, roomId int64) ([]*types.RoomMessage, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.ListRoomMessage(ctx, roomId)
}

func (s *sqliteMetaStore) SaveRoomMessage(ctx context.Context, msg *types.RoomMessage) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.SaveRoomMessage(ctx, msg)
}

func (s *sqliteMetaStore) GetRoomMessage(ctx context.Context, msgId int64) (*types.RoomMessage, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.GetRoomMessage(ctx, msgId)
}

func (s *sqliteMetaStore) DeleteRoomMessages(ctx context.Context, roomId int64) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.dbStore.DeleteRoomMessages(ctx, roomId)
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

	dbConn.SetMaxIdleConns(1)
	dbConn.SetMaxOpenConns(1)
	dbConn.SetConnMaxLifetime(time.Hour)

	dbStore, err := buildSqlMetaStore(dbEntity)
	if err != nil {
		return nil, err
	}

	return &sqliteMetaStore{dbStore: dbStore}, nil
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

func (s *sqlMetaStore) GetAccessToken(ctx context.Context, tokenKey string, secretKey string) (*types.AccessToken, error) {
	defer trace.StartRegion(ctx, "metastore.sql.GetAccessToken").End()
	token := db.AccessToken{}
	res := s.WithContext(ctx).Where("token_key = ? AND secret_token = ?", tokenKey, secretKey).First(&token)
	if res.Error != nil {
		return nil, db.SqlError2Error(res.Error)
	}
	token.LastSeenAt = time.Now()
	if res = s.WithContext(ctx).Save(&token); res.Error != nil {
		s.logger.Errorw("update access token lastSeenAt failed", "err", res.Error)
	}
	return &types.AccessToken{
		TokenKey:       token.TokenKey,
		SecretToken:    token.SecretToken,
		UID:            token.UID,
		GID:            token.GID,
		ClientCrt:      token.ClientCrt,
		ClientKey:      token.ClientKey,
		CertExpiration: token.CertExpiration,
		LastSeenAt:     token.LastSeenAt,
		Namespace:      token.Namespace,
	}, nil
}

func (s *sqlMetaStore) CreateAccessToken(ctx context.Context, token *types.AccessToken) error {
	defer trace.StartRegion(ctx, "metastore.sql.CreateAccessToken").End()
	tokenModel := db.AccessToken{}
	res := s.WithContext(ctx).Where("token_key = ?", token.TokenKey).First(&tokenModel)
	if res.Error == nil {
		return types.ErrIsExist
	} else if !errors.Is(res.Error, gorm.ErrRecordNotFound) {
		return db.SqlError2Error(res.Error)
	}

	token.LastSeenAt = time.Now()
	tokenModel = db.AccessToken{
		TokenKey:       token.TokenKey,
		SecretToken:    token.SecretToken,
		UID:            token.UID,
		GID:            token.GID,
		ClientCrt:      token.ClientCrt,
		ClientKey:      token.ClientKey,
		CertExpiration: token.CertExpiration,
		LastSeenAt:     token.LastSeenAt,
		Namespace:      token.Namespace,
	}
	if res = s.WithContext(ctx).Create(&tokenModel); res.Error != nil {
		s.logger.Errorw("create access token failed", "err", res.Error)
		return db.SqlError2Error(res.Error)
	}
	return nil
}

func (s *sqlMetaStore) UpdateAccessTokenCerts(ctx context.Context, token *types.AccessToken) error {
	defer trace.StartRegion(ctx, "metastore.sql.UpdateAccessTokenCerts").End()
	tokenModel := db.AccessToken{}
	res := s.WithContext(ctx).Where("token_key = ? AND secret_token = ?", token.TokenKey, token.SecretToken).First(&tokenModel)
	if res.Error != nil {
		return db.SqlError2Error(res.Error)
	}

	token.LastSeenAt = time.Now()
	tokenModel.ClientCrt = token.ClientCrt
	tokenModel.ClientKey = token.ClientKey
	tokenModel.CertExpiration = token.CertExpiration
	tokenModel.LastSeenAt = token.LastSeenAt
	if res = s.WithContext(ctx).Save(&tokenModel); res.Error != nil {
		s.logger.Errorw("update access token certs failed", "err", res.Error)
	}
	return nil
}

func (s *sqlMetaStore) RevokeAccessToken(ctx context.Context, tokenKey string) error {
	defer trace.StartRegion(ctx, "metastore.sql.RevokeAccessToken").End()
	tokenModel := db.AccessToken{}
	res := s.WithContext(ctx).Where("token_key = ?", tokenKey).First(&tokenModel)
	if res.Error != nil {
		return db.SqlError2Error(res.Error)
	}
	res = s.WithContext(ctx).Delete(tokenModel)
	if res.Error != nil {
		return db.SqlError2Error(res.Error)
	}
	return nil
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

	var totalChunk int64
	res = s.WithContext(ctx).Model(&db.ObjectChunk{}).Count(&totalChunk)
	if res.Error != nil {
		return nil, db.SqlError2Error(res.Error)
	}
	if totalChunk == 0 {
		return result, nil
	}

	// real usage in storage
	res = s.WithContext(ctx).Model(&db.ObjectChunk{}).Select("SUM(len) as file_size_total").Scan(&result.FileSizeTotal)
	if res.Error != nil {
		return nil, db.SqlError2Error(res.Error)
	}
	return result, nil
}

func (s *sqlMetaStore) GetConfigValue(ctx context.Context, group string, name string) (string, error) {
	defer trace.StartRegion(ctx, "metastore.sql.GetConfigValue").End()
	var sysCfg = db.SystemConfig{}
	res := s.WithNamespace(ctx).Where("cfg_group = ? AND cfg_name = ?", group, name).First(&sysCfg)
	if res.Error != nil {
		return "", db.SqlError2Error(res.Error)
	}
	return sysCfg.Value, nil
}

func (s *sqlMetaStore) SetConfigValue(ctx context.Context, group, name, value string) error {
	defer trace.StartRegion(ctx, "metastore.sql.SetConfigValue").End()
	var sysCfg = db.SystemConfig{}
	res := s.WithNamespace(ctx).Where("cfg_group = ? AND cfg_name = ?", group, name).First(&sysCfg)
	if res.Error != nil && !errors.Is(res.Error, gorm.ErrRecordNotFound) {
		return res.Error
	}
	sysCfg.Namespace = types.GetNamespace(ctx).String()
	sysCfg.Group = group
	sysCfg.Name = name
	sysCfg.Value = value
	sysCfg.ChangedAt = time.Now()
	res = s.WithNamespace(ctx).Save(&sysCfg)
	return res.Error
}

func (s *sqlMetaStore) GetEntry(ctx context.Context, id int64) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "metastore.sql.GetEntry").End()
	var objMod = &db.Object{ID: id}
	res := s.WithNamespace(ctx).Where("id = ?", id).First(objMod)
	if err := res.Error; err != nil {
		return nil, db.SqlError2Error(err)
	}
	return objMod.ToEntry(), nil
}

func (s *sqlMetaStore) FindEntry(ctx context.Context, parentID int64, name string) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "metastore.sql.FindEntry").End()
	var objMod = &db.Object{ParentID: &parentID, Name: name}
	res := s.WithNamespace(ctx).Where("parent_id = ? AND name = ?", parentID, name).First(objMod)
	if err := res.Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Errorw("find entry by name failed", "parent", parentID, "name", name, "err", err)
		}
		return nil, db.SqlError2Error(err)
	}
	return objMod.ToEntry(), nil
}

func (s *sqlMetaStore) CreateEntry(ctx context.Context, parentID int64, newEntry *types.Metadata, ed *types.ExtendData) error {
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

		if ed != nil {
			edMod := (&db.ObjectExtend{ID: entryMod.ID}).From(ed)
			res = tx.Create(edMod)
			if res.Error != nil {
				return res.Error
			}
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
		if newEntry.IsGroup {
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

func (s *sqlMetaStore) SaveEntryUri(ctx context.Context, entryUri *types.EntryUri) error {
	defer trace.StartRegion(ctx, "metastore.sql.CreateEntryUri").End()
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		entryUriMod := &db.ObjectURI{}
		res := tx.Where("oid = ?", entryUri.ID).First(entryUriMod)
		if res.Error != nil {
			if res.Error == gorm.ErrRecordNotFound {
				entryUriMod = entryUriMod.FromEntryUri(entryUri)
				res = tx.Create(entryUriMod)
				return res.Error
			}
			return res.Error
		}
		entryUriMod = entryUriMod.FromEntryUri(entryUri)
		res = tx.Updates(entryUriMod)
		return res.Error
	})
	if err != nil {
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) GetEntryUri(ctx context.Context, uri string) (*types.EntryUri, error) {
	defer trace.StartRegion(ctx, "metastore.sql.GetEntryUri").End()
	var entryUri = &db.ObjectURI{Uri: uri}
	res := s.WithNamespace(ctx).Where("uri = ?", uri).First(entryUri)
	if err := res.Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			s.logger.Errorw("get entryUri by uri failed", "uri", uri, "err", err)
		}
		return nil, db.SqlError2Error(err)
	}
	return entryUri.ToEntryUri(), nil
}

func (s *sqlMetaStore) UpdateEntryMetadata(ctx context.Context, entry *types.Metadata) error {
	defer trace.StartRegion(ctx, "metastore.sql.UpdateEntry").End()
	var entryMod = (&db.Object{}).FromEntry(entry)
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		entryMod.ChangedAt = time.Now().UnixNano()
		return updateEntryAndCleanURICache(tx, entryMod)
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
		res := namespaceQuery(ctx, tx).Where("id = ?", parentID).First(parentMod)
		if res.Error != nil {
			return res.Error
		}
		res = namespaceQuery(ctx, tx).Where("id = ?", entryID).First(entryMod)
		if res.Error != nil {
			return res.Error
		}

		if entryMod.RefID != nil && *entryMod.RefID != 0 {
			srcMod = &db.Object{ID: *entryMod.RefID}
			res = namespaceQuery(ctx, tx).Where("id = ?", *entryMod.RefID).First(srcMod)
			if res.Error != nil {
				return res.Error
			}
		}

		parentMod.ModifiedAt = nowTime
		parentMod.ChangedAt = nowTime
		if entryMod.IsGroup {

			var entryChildCount int64
			res = namespaceQuery(ctx, tx.Model(&db.Object{})).Where("parent_id = ?", entryID).Count(&entryChildCount)
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
		if err := updateEntryWithVersion(tx, parentMod); err != nil {
			return err
		}

		if srcMod != nil {
			srcRef := (*srcMod.RefCount) - 1
			srcMod.RefCount = &srcRef
			srcMod.ModifiedAt = nowTime
			srcMod.ChangedAt = nowTime
			if err := updateEntryWithVersion(tx, srcMod); err != nil {
				return err
			}
		}

		entryRef := (*entryMod.RefCount) - 1
		entryMod.ParentID = &noneParentID
		entryMod.RefCount = &entryRef
		entryMod.ModifiedAt = nowTime
		entryMod.ChangedAt = nowTime
		return updateEntryAndCleanURICache(tx, entryMod)
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
	)
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := namespaceQuery(ctx, tx).Where("id = ?", entryID).First(entryMod)
		if res.Error != nil {
			return res.Error
		}

		if entryMod.RefCount != nil && *entryMod.RefCount > 0 && entryMod.IsGroup {
			s.logger.Warnf("entry %d ref_count is %d, more than 0", entryMod.ID, *entryMod.RefCount)
		}

		res = namespaceQuery(ctx, tx).Where("id = ?", entryMod.ID).Delete(&db.Object{})
		if res.Error != nil {
			return res.Error
		}
		res = namespaceQuery(ctx, tx).Where("id = ?", entryMod.ID).Delete(&db.ObjectExtend{})
		if res.Error != nil {
			return res.Error
		}
		res = namespaceQuery(ctx, tx).Where("oid = ?", entryMod.ID).Delete(&db.ObjectProperty{})
		if res.Error != nil {
			return res.Error
		}
		res = namespaceQuery(ctx, tx).Where("ref_type = 'object' AND ref_id = ?", entryID).Delete(&db.Label{})
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

func (s *sqlMetaStore) ListEntryChildren(ctx context.Context, parentId int64, order *types.EntryOrder, filters ...types.Filter) (EntryIterator, error) {
	defer trace.StartRegion(ctx, "metastore.sql.ListEntryChildren").End()
	var total int64
	if len(filters) > 0 {
		filters[0].ParentID = parentId
	} else {
		filters = append(filters, types.Filter{ParentID: parentId})
	}
	tx := enOrder(queryFilter(s.WithNamespace(ctx).Model(&db.Object{}), filters[0], nil), order)
	if page := types.GetPagination(ctx); page != nil {
		tx = tx.Offset(page.Offset()).Limit(page.Limit())
	}
	res := tx.Count(&total)
	if err := res.Error; err != nil {
		s.logger.Errorw("count children entry failed", "parent", parentId, "err", err)
		return nil, db.SqlError2Error(err)
	}
	return newTransactionEntryIterator(tx, total), nil
}

func (s *sqlMetaStore) ListDocumentGroups(ctx context.Context, parentId int64, filter types.DocFilter) (EntryIterator, error) {
	defer trace.StartRegion(ctx, "metastore.sql.ListDocumentGroups").End()
	var total int64
	t := true
	efilter := types.Filter{IsGroup: &t}
	if parentId != 0 {
		filter.ParentID = parentId
	}
	tx := queryFilter(s.WithNamespaceOfTable(ctx, "object").Model(&db.Object{}), efilter, nil)

	if page := types.GetPagination(ctx); page != nil {
		tx = tx.Offset(page.Offset()).Limit(page.Limit())
	}

	tx = tx.Joins("LEFT JOIN document ON object.id = document.parent_entry_id")
	tx = docQueryFilter(tx, filter).Distinct()

	res := tx.Count(&total)
	if err := res.Error; err != nil {
		s.logger.Errorw("count groups entry failed", "parent", parentId, "err", err)
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
		scopeIds, err = listEntryIdsWithLabelMatcher(ctx, s.DB, filter.Label)
		if err != nil {
			return nil, db.SqlError2Error(err)
		}
		if len(scopeIds) == 0 {
			return newTransactionEntryIterator(s.DB, 0), nil
		}
	}

	var total int64
	tx := queryFilter(s.WithNamespace(ctx).Model(&db.Object{}), filter, scopeIds)
	if page := types.GetPagination(ctx); page != nil {
		tx = tx.Offset(page.Offset()).Limit(page.Limit())
	}
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
		res := namespaceQuery(ctx, tx).Where("id = ?", id).First(enMod)
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
		res := namespaceQuery(ctx, tx).Where("id = ?", id).First(enMod)
		if res.Error != nil {
			return res.Error
		}
		enMod.ModifiedAt = nowTime
		enMod.ChangedAt = nowTime
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
	var ext = &db.ObjectExtend{}
	res := s.WithContext(ctx).Where("id = ?", id).First(ext)
	if res.Error != nil {
		err := db.SqlError2Error(res.Error)
		if errors.Is(err, types.ErrNotFound) {
			return types.ExtendData{}, nil
		}
		return types.ExtendData{}, err
	}
	return ext.ToExtData(), nil
}

func (s *sqlMetaStore) UpdateEntryExtendData(ctx context.Context, id int64, ed types.ExtendData) error {
	defer trace.StartRegion(ctx, "metastore.sql.UpdateEntryExtendData").End()
	var (
		entryModel = &db.Object{ID: id}
		extModel   = &db.ObjectExtend{ID: id}
	)

	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := namespaceQuery(ctx, tx).Where("id = ?", id).First(entryModel)
		if res.Error != nil {
			return res.Error
		}

		extModel.From(&ed)
		res = tx.Save(extModel)
		if res.Error != nil {
			return res.Error
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

		res := namespaceQuery(ctx, tx).Where("id = ?", newEntry.RefID).First(srcEnModel)
		if res.Error != nil {
			return res.Error
		}
		srcRefCount := *srcEnModel.RefCount + 1
		srcEnModel.RefCount = &srcRefCount
		srcEnModel.ChangedAt = nowTime
		if updateErr = updateEntryWithVersion(tx, srcEnModel); updateErr != nil {
			return updateErr
		}

		res = namespaceQuery(ctx, tx).Where("id = ?", newEntry.ParentID).First(dstParentEnModel)
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
			srcParentEntryID int64
			updateErr        error
		)
		res := namespaceQuery(ctx, tx).Where("id = ?", targetEntryId).First(enModel)
		if res.Error != nil {
			return res.Error
		}
		res = namespaceQuery(ctx, tx).Where("id = ?", newParentId).First(dstParentEnModel)
		if res.Error != nil {
			return res.Error
		}

		if enModel.ParentID != nil {
			srcParentEntryID = *enModel.ParentID
		}
		enModel.Name = newName
		enModel.ParentID = &newParentId
		enModel.ChangedAt = nowTime
		if updateErr = updateEntryAndCleanURICache(tx, enModel); updateErr != nil {
			s.logger.Errorw("update target entry failed when change parent",
				"entry", targetEntryId, "srcParent", srcParentEntryID, "dstParent", newParentId, "err", updateErr)
			return updateErr
		}

		res = namespaceQuery(ctx, tx).Model(&db.Document{}).Where("oid = ?", targetEntryId).Update("parent_entry_id", newParentId)
		if res.Error != nil {
			return res.Error
		}

		res = namespaceQuery(ctx, tx).Where("id = ?", srcParentEntryID).First(srcParentEnModel)
		if res.Error != nil {
			return res.Error
		}
		if enModel.IsGroup && newParentId != srcParentEntryID {
			dstParentEnModel.ChangedAt = nowTime
			dstParentEnModel.ModifiedAt = nowTime
			dstParentRef := *dstParentEnModel.RefCount + 1
			dstParentEnModel.RefCount = &dstParentRef
			dstParentEnModel.ChangedAt = nowTime
			if updateErr = updateEntryWithVersion(tx, dstParentEnModel); updateErr != nil {
				s.logger.Errorw("update dst parent entry failed when change parent",
					"entry", targetEntryId, "srcParent", srcParentEntryID, "dstParent", newParentId, "err", updateErr)
				return updateErr
			}

			srcParentRef := *srcParentEnModel.RefCount - 1
			srcParentEnModel.RefCount = &srcParentRef
		}

		srcParentEnModel.ChangedAt = nowTime
		srcParentEnModel.ModifiedAt = nowTime
		if updateErr = updateEntryWithVersion(tx, srcParentEnModel); updateErr != nil {
			s.logger.Errorw("update src parent entry failed when change parent",
				"entry", targetEntryId, "srcParent", srcParentEntryID, "dstParent", newParentId, "err", updateErr)
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

	res := s.WithNamespace(ctx).Where("ref_type = ? and ref_id = ?", "object", id).Find(&labelMods)
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
		res := namespaceQuery(ctx, tx).Where("ref_type = ? AND ref_id = ?", "object", id).Find(&labelModels)
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

func (s *sqlMetaStore) ListEntryProperties(ctx context.Context, id int64) (types.Properties, error) {
	defer trace.StartRegion(ctx, "metastore.sql.ListEntryProperties").End()
	var (
		property = make([]db.ObjectProperty, 0)
		result   = types.Properties{Fields: map[string]types.PropertyItem{}}
	)

	res := s.WithNamespace(ctx).Where("oid = ?", id).Find(&property)
	if res.Error != nil {
		return result, res.Error
	}

	for _, p := range property {
		result.Fields[p.Name] = types.PropertyItem{Value: p.Value, Encoded: p.Encoded}
	}
	return result, nil
}

func (s *sqlMetaStore) GetEntryProperty(ctx context.Context, id int64, key string) (types.PropertyItem, error) {
	defer trace.StartRegion(ctx, "metastore.sql.GetEntryProperty").End()
	var (
		itemModel = &db.ObjectProperty{}
		result    types.PropertyItem
	)
	res := s.WithContext(ctx).Where("oid = ? AND key = ?", id, key).First(&itemModel)
	if res.Error != nil {
		return result, db.SqlError2Error(res.Error)
	}
	result.Value = itemModel.Value
	result.Encoded = itemModel.Encoded
	return result, nil
}

func (s *sqlMetaStore) AddEntryProperty(ctx context.Context, id int64, key string, item types.PropertyItem) error {
	defer trace.StartRegion(ctx, "metastore.sql.AddEntryProperty").End()
	var (
		entryModel   = &db.Object{ID: id}
		propertyItem = &db.ObjectProperty{}
	)
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := namespaceQuery(ctx, tx).Where("id = ?", id).First(entryModel)
		if res.Error != nil {
			return res.Error
		}

		res = namespaceQuery(ctx, tx).Where("oid = ? AND key = ?", id, key).First(propertyItem)
		if res.Error != nil && !errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return res.Error
		}

		propertyItem.OID = id
		propertyItem.Namespace = entryModel.Namespace
		propertyItem.Name = key
		propertyItem.Value = item.Value
		propertyItem.Encoded = item.Encoded

		if propertyItem.ID == 0 {
			res = tx.Create(propertyItem)
			return res.Error
		}
		res = tx.Updates(propertyItem)
		return res.Error
	})
	if err != nil {
		s.logger.Errorw("add entry property failed", "entry", id, "err", err)
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) RemoveEntryProperty(ctx context.Context, id int64, key string) error {
	defer trace.StartRegion(ctx, "metastore.sql.RemoveEntryProperty").End()
	res := s.WithContext(ctx).Where("oid = ? AND key = ?", id, key).Delete(&db.ObjectProperty{})
	if res.Error != nil {
		return db.SqlError2Error(res.Error)
	}
	return nil
}

func (s *sqlMetaStore) UpdateEntryProperties(ctx context.Context, id int64, properties types.Properties) error {
	defer trace.StartRegion(ctx, "metastore.sql.UpdateEntryProperties").End()
	var (
		entryModel                 = &db.Object{ID: id}
		objectProperties           = make([]db.ObjectProperty, 0)
		needCreatePropertiesModels = make([]db.ObjectProperty, 0)
		needUpdatePropertiesModels = make([]db.ObjectProperty, 0)
		needDeletePropertiesModels = make([]db.ObjectProperty, 0)
	)
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := namespaceQuery(ctx, tx).Where("id = ?", id).First(entryModel)
		if res.Error != nil {
			return res.Error
		}

		res = namespaceQuery(ctx, tx).Where("oid = ?", id).Find(&objectProperties)
		if res.Error != nil {
			return res.Error
		}

		propertiesMap := map[string]types.PropertyItem{}
		for k, v := range properties.Fields {
			propertiesMap[k] = v
		}

		for i := range objectProperties {
			oldKv := objectProperties[i]
			if newV, ok := propertiesMap[oldKv.Name]; ok {
				if oldKv.Value != newV.Value {
					oldKv.Value = newV.Value
					oldKv.Encoded = newV.Encoded
					needUpdatePropertiesModels = append(needUpdatePropertiesModels, oldKv)
				}
				delete(propertiesMap, oldKv.Name)
				continue
			}
			needDeletePropertiesModels = append(needDeletePropertiesModels, oldKv)
		}

		for k, v := range propertiesMap {
			needCreatePropertiesModels = append(needCreatePropertiesModels,
				db.ObjectProperty{OID: id, Name: k, Namespace: entryModel.Namespace, Value: v.Value, Encoded: v.Encoded})
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
		s.logger.Errorw("save entry properties failed", "entry", id, "err", err)
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
			ID:      seg.ID,
			ChunkID: seg.ChunkID,
			EntryID: seg.OID,
			Off:     seg.Off,
			Len:     seg.Len,
			State:   seg.State,
		}

	}
	return result, nil
}

func (s *sqlMetaStore) AppendSegments(ctx context.Context, seg types.ChunkSeg) (*types.Metadata, error) {
	defer trace.StartRegion(ctx, "metastore.sql.AppendSegments").End()
	var (
		enMod   = &db.Object{ID: seg.EntryID}
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
			OID:      seg.EntryID,
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
		_ = json.Unmarshal([]byte(t.Event), &evt)
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
	t.Event = string(rawEvt)
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

func (s *sqlMetaStore) GetWorkflow(ctx context.Context, wfID string) (*types.Workflow, error) {
	defer trace.StartRegion(ctx, "metastore.sql.GetWorkflow").End()
	wf := &db.Workflow{}
	res := s.WithContext(ctx).Where("id = ?", wfID).First(wf)
	if res.Error != nil {
		return nil, db.SqlError2Error(res.Error)
	}
	result, err := wf.To()
	if err != nil {
		return nil, err
	}
	result.HealthScore = workflowScore(ctx, wf.ID, s.DB)
	return result, nil
}

func (s *sqlMetaStore) ListGlobalWorkflow(ctx context.Context) ([]*types.Workflow, error) {
	return s.listWorkflow(ctx, types.GlobalNamespaceValue)
}

func (s *sqlMetaStore) ListWorkflow(ctx context.Context) ([]*types.Workflow, error) {
	ns := types.GetNamespace(ctx)
	if ns.String() == types.DefaultNamespaceValue {
		return s.listWorkflow(ctx, "")
	}
	return s.listWorkflow(ctx, ns.String())
}

func (s *sqlMetaStore) listWorkflow(ctx context.Context, namespace string) ([]*types.Workflow, error) {
	defer trace.StartRegion(ctx, "metastore.sql.ListWorkflow").End()
	wfList := make([]db.Workflow, 0)
	res := s.WithContext(ctx).Order("created_at desc")
	if namespace != "" {
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
		result[i].HealthScore = workflowScore(ctx, wf.ID, s.DB)
	}
	return result, nil
}

func (s *sqlMetaStore) DeleteWorkflow(ctx context.Context, wfID string) error {
	defer trace.StartRegion(ctx, "metastore.sql.DeleteWorkflow").End()
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := namespaceQuery(ctx, tx).Where("id = ?", wfID).Delete(&db.Workflow{})
		if res.Error != nil {
			return res.Error
		}
		res = namespaceQuery(ctx, tx).Where("workflow = ?", wfID).Delete(&db.WorkflowJob{})
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

func (s *sqlMetaStore) GetWorkflowJob(ctx context.Context, jobID string) (*types.WorkflowJob, error) {
	defer trace.StartRegion(ctx, "metastore.sql.GetWorkflowJob").End()
	jobMod := &db.WorkflowJob{}
	res := s.WithNamespace(ctx).Where("id = ?", jobID).First(jobMod)
	if res.Error != nil {
		return nil, db.SqlError2Error(res.Error)
	}
	return jobMod.To()
}

func (s *sqlMetaStore) ListWorkflowJob(ctx context.Context, filter types.JobFilter) ([]*types.WorkflowJob, error) {
	defer trace.StartRegion(ctx, "metastore.sql.ListWorkflowJob").End()
	jobList := make([]db.WorkflowJob, 0)
	query := s.WithNamespace(ctx)
	if page := types.GetPagination(ctx); page != nil {
		query = query.Offset(page.Offset()).Limit(page.Limit())
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

func (s *sqlMetaStore) SaveWorkflow(ctx context.Context, wf *types.Workflow) error {
	defer trace.StartRegion(ctx, "metastore.sql.SaveWorkflow").End()
	wf.UpdatedAt = time.Now()
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var loadErr error
		dbMod := &db.Workflow{}
		res := namespaceQuery(ctx, tx).Where("id = ?", wf.Id).First(dbMod)
		if res.Error != nil {
			if res.Error == gorm.ErrRecordNotFound {
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

func (s *sqlMetaStore) SaveWorkflowJob(ctx context.Context, job *types.WorkflowJob) error {
	defer trace.StartRegion(ctx, "metastore.sql.SaveWorkflowJob").End()
	job.UpdatedAt = time.Now()
	if job.Namespace == "" {
		job.Namespace = types.GetNamespace(ctx).String()
	}
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		jobModel := &db.WorkflowJob{}
		var loadErr error
		res := namespaceQuery(ctx, tx).Where("id = ?", job.Id).First(jobModel)
		if res.Error != nil {
			if res.Error == gorm.ErrRecordNotFound {
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
		return res.Error
	})
	if err != nil {
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) DeleteWorkflowJob(ctx context.Context, wfJobID ...string) error {
	defer trace.StartRegion(ctx, "metastore.sql.DeleteWorkflowJob").End()
	res := s.WithNamespace(ctx).Where("id IN ?", wfJobID).Delete(&db.WorkflowJob{})
	if res.Error != nil {
		return db.SqlError2Error(res.Error)
	}
	return nil
}

func (s *sqlMetaStore) ListNotifications(ctx context.Context) ([]types.Notification, error) {
	defer trace.StartRegion(ctx, "metastore.sql.ListNotifications").End()
	noList := make([]db.Notification, 0)
	res := s.WithNamespace(ctx).Order("time desc").Find(&noList)
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

func (s *sqlMetaStore) RecordNotification(ctx context.Context, nid string, no types.Notification) error {
	defer trace.StartRegion(ctx, "metastore.sql.RecordNotification").End()
	model := db.Notification{
		ID:        nid,
		Namespace: no.Namespace,
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

func (s *sqlMetaStore) UpdateNotificationStatus(ctx context.Context, nid, status string) error {
	defer trace.StartRegion(ctx, "metastore.sql.UpdateNotificationStatus").End()
	res := s.WithNamespace(ctx).Model(&db.Notification{}).Where("id = ?", nid).UpdateColumn("status", status)
	if res.Error != nil {
		return db.SqlError2Error(res.Error)
	}
	return nil
}

func (s *sqlMetaStore) RecordEvents(ctx context.Context, events []types.Event) error {
	defer trace.StartRegion(ctx, "metastore.sql.RecordEvents").End()

	if len(events) == 0 {
		return nil
	}

	eventModels := make([]db.Event, len(events))
	for i, evt := range events {
		eventModels[i].From(evt)
	}

	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, evt := range eventModels {
			if res := tx.Create(evt); res.Error != nil {
				s.logger.Errorw("insert event error", "event", evt.ID, "err", res.Error)
				return res.Error
			}
		}
		return nil
	})
	return err
}

func (s *sqlMetaStore) ListEvents(ctx context.Context, filter types.EventFilter) ([]types.Event, error) {
	defer trace.StartRegion(ctx, "metastore.sql.ListEvents").End()
	query := s.WithNamespace(ctx)
	if filter.StartSequence > 0 {
		query = query.Where("sequence > ?", filter.StartSequence)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.DESC {
		query = query.Order("sequence DESC")
	} else {
		query = query.Order("sequence ASC")
	}

	modelList := make([]db.Event, 0)
	res := query.Find(&modelList)
	if res.Error != nil {
		s.logger.Errorw("list event from db failed", "err", res.Error)
		return nil, res.Error
	}

	var (
		result = make([]types.Event, len(modelList))
		err    error
	)
	for i, m := range modelList {
		result[i], err = m.To()
		if err != nil {
			s.logger.Errorw("trans event data from db model error", "event", m.ID, "data", m.Data)
		}
	}
	return result, nil
}

func (s *sqlMetaStore) DeviceSync(ctx context.Context, deviceID string, syncedSequence int64) error {
	defer trace.StartRegion(ctx, "metastore.sql.DeviceSync").End()
	return s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		model := db.RegisteredDevice{}
		res := namespaceQuery(ctx, tx).Where("id = ?", deviceID).First(&model)
		if res.Error != nil {
			if errors.Is(res.Error, gorm.ErrRecordNotFound) {
				model.ID = deviceID
				model.SyncedSequence = syncedSequence
				model.Namespace = types.GetNamespace(ctx).String()
				model.LastSeenAt = time.Now()
				res = tx.Create(model)
				return res.Error
			}
			s.logger.Errorw("query device error", "deviceID", deviceID, "err", res.Error)
			return res.Error
		}

		model.SyncedSequence = syncedSequence
		model.LastSeenAt = time.Now()
		res = tx.Save(model)
		return res.Error
	})
}

func (s *sqlMetaStore) SaveDocument(ctx context.Context, doc *types.Document) error {
	defer trace.StartRegion(ctx, "metastore.sql.SaveDocument").End()
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		docMod := &db.Document{}
		res := tx.Where("id = ?", doc.ID).First(docMod)
		if res.Error != nil {
			if errors.Is(res.Error, gorm.ErrRecordNotFound) {
				docMod = docMod.From(doc)
				res = tx.Create(docMod)
				return res.Error
			}
			return res.Error
		}
		docMod = docMod.From(doc)
		res = tx.Save(docMod)
		return res.Error
	})
	if err != nil {
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) ListDocument(ctx context.Context, filter types.DocFilter, order *types.DocumentOrder) ([]*types.Document, error) {
	defer trace.StartRegion(ctx, "metastore.sql.ListDocument").End()
	docList := make([]db.Document, 0)
	query := s.WithNamespace(ctx)
	if page := types.GetPagination(ctx); page != nil {
		query = query.Offset(page.Offset()).Limit(page.Limit())
	}
	res := docOrder(docQueryFilter(query, filter), order).Find(&docList)
	if res.Error != nil {
		return nil, db.SqlError2Error(res.Error)
	}

	result := make([]*types.Document, len(docList))
	for i, doc := range docList {
		result[i] = doc.To()
	}
	return result, nil
}

func (s *sqlMetaStore) GetDocument(ctx context.Context, id int64) (*types.Document, error) {
	defer trace.StartRegion(ctx, "metastore.sql.GetDocument").End()
	doc := &db.Document{}
	res := s.WithNamespace(ctx).Where("id = ?", id).First(doc)
	if res.Error != nil {
		return nil, db.SqlError2Error(res.Error)
	}
	return doc.To(), nil
}

func (s *sqlMetaStore) GetDocumentByName(ctx context.Context, name string) (*types.Document, error) {
	defer trace.StartRegion(ctx, "metastore.sql.GetDocumentByName").End()
	doc := &db.Document{}
	res := s.WithNamespace(ctx).Where("name = ?", name).First(doc)
	if res.Error != nil {
		return nil, db.SqlError2Error(res.Error)
	}
	return doc.To(), nil
}

func (s *sqlMetaStore) GetDocumentByEntryId(ctx context.Context, oid int64) (*types.Document, error) {
	defer trace.StartRegion(ctx, "metastore.sql.GetDocumentByEntryId").End()
	doc := &db.Document{}
	res := s.WithNamespace(ctx).Where("oid = ?", oid).First(doc)
	if res.Error != nil {
		return nil, db.SqlError2Error(res.Error)
	}
	return doc.To(), nil
}

func (s *sqlMetaStore) DeleteDocument(ctx context.Context, id int64) error {
	defer trace.StartRegion(ctx, "metastore.sql.DeleteDocument").End()
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := namespaceQuery(ctx, tx).Where("id = ?", id).Delete(&db.Document{})
		return res.Error
	})
	return db.SqlError2Error(err)
}

func (s *sqlMetaStore) ListFridayAccount(ctx context.Context, refId int64) ([]*types.FridayAccount, error) {
	defer trace.StartRegion(ctx, "metastore.sql.ListFridayAccount").End()
	accountList := make([]db.FridayAccount, 0)
	query := s.WithNamespace(ctx).Where("ref_id = ?", refId)
	if page := types.GetPagination(ctx); page != nil {
		query = query.Offset(page.Offset()).Limit(page.Limit())
	}
	res := query.Order("created_at DESC").Find(&accountList)
	if res.Error != nil {
		return nil, db.SqlError2Error(res.Error)
	}

	result := make([]*types.FridayAccount, len(accountList))
	for i, account := range accountList {
		result[i] = account.To()
	}
	return result, nil
}

func (s *sqlMetaStore) CreateFridayAccount(ctx context.Context, account *types.FridayAccount) error {
	defer trace.StartRegion(ctx, "metastore.sql.CreateFridayAccount").End()
	accountMod := &db.FridayAccount{}
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		accountMod = accountMod.From(account)
		res := namespaceQuery(ctx, tx).Create(accountMod)
		return res.Error
	})
	return db.SqlError2Error(err)
}

func (s *sqlMetaStore) SaveRoom(ctx context.Context, room *types.Room) error {
	defer trace.StartRegion(ctx, "metastore.sql.CreateRoom").End()
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		roomMod := &db.Room{}
		var err error
		res := namespaceQuery(ctx, tx).Where("id = ?", room.ID).First(roomMod)
		if res.Error != nil {
			if errors.Is(res.Error, gorm.ErrRecordNotFound) {
				roomMod, err = roomMod.From(room)
				if err != nil {
					return err
				}
				res := tx.Create(roomMod)
				return res.Error
			}
			return res.Error
		}
		roomMod, err = roomMod.From(room)
		if err != nil {
			return err
		}
		res = tx.Save(roomMod)
		return res.Error
	})
	if err != nil {
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) GetRoom(ctx context.Context, id int64) (*types.Room, error) {
	defer trace.StartRegion(ctx, "metastore.sql.GetRoom").End()
	room := &db.Room{}
	res := s.WithNamespace(ctx).Where("id = ?", id).First(room)
	if res.Error != nil {
		return nil, db.SqlError2Error(res.Error)
	}
	return room.To()
}

func (s *sqlMetaStore) FindRoom(ctx context.Context, entryId int64) (*types.Room, error) {
	defer trace.StartRegion(ctx, "metastore.sql.FindRoom").End()
	room := &db.Room{}
	res := s.WithNamespace(ctx).Where("entry_id = ?", entryId).First(room)
	if res.Error != nil {
		return nil, db.SqlError2Error(res.Error)
	}
	return room.To()
}

func (s *sqlMetaStore) DeleteRoom(ctx context.Context, id int64) error {
	defer trace.StartRegion(ctx, "metastore.sql.DeleteRoom").End()
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := namespaceQuery(ctx, tx).Where("id = ?", id).Delete(&db.Room{})
		return res.Error
	})
	return db.SqlError2Error(err)
}

func (s *sqlMetaStore) ListRooms(ctx context.Context, entryId int64) ([]*types.Room, error) {
	defer trace.StartRegion(ctx, "metastore.sql.ListRoom").End()
	roomList := make([]db.Room, 0)
	query := s.WithNamespace(ctx)
	query = query.Where("entry_id = ?", entryId)
	if page := types.GetPagination(ctx); page != nil {
		query = query.Offset(page.Offset()).Limit(page.Limit())
	}
	res := query.Order("created_at DESC").Find(&roomList)
	if res.Error != nil {
		return nil, db.SqlError2Error(res.Error)
	}

	result := make([]*types.Room, len(roomList))
	for i, room := range roomList {
		r, err := room.To()
		if err != nil {
			return nil, err
		}
		result[i] = r
	}
	return result, nil
}

func (s *sqlMetaStore) ListRoomMessage(ctx context.Context, roomId int64) ([]*types.RoomMessage, error) {
	defer trace.StartRegion(ctx, "metastore.sql.ListRoomMessage").End()
	roomMsgList := make([]db.RoomMessage, 0)
	query := s.WithNamespace(ctx)
	query = query.Where("room_id = ?", roomId)
	if page := types.GetPagination(ctx); page != nil {
		query = query.Offset(page.Offset()).Limit(page.Limit())
	}
	res := query.Order("send_at").Find(&roomMsgList)
	if res.Error != nil {
		return nil, db.SqlError2Error(res.Error)
	}

	result := make([]*types.RoomMessage, len(roomMsgList))
	for i, msg := range roomMsgList {
		result[i] = msg.To()
	}
	return result, nil
}

func (s *sqlMetaStore) SaveRoomMessage(ctx context.Context, msg *types.RoomMessage) error {
	defer trace.StartRegion(ctx, "metastore.sql.SaveRoomMessage").End()
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		roomMsgMod := &db.RoomMessage{}
		res := namespaceQuery(ctx, tx).Where("id = ?", msg.ID).First(roomMsgMod)
		if res.Error != nil {
			if errors.Is(res.Error, gorm.ErrRecordNotFound) {
				roomMsgMod = roomMsgMod.From(msg)
				res := tx.Create(roomMsgMod)
				return res.Error
			}
			return res.Error
		}
		roomMsgMod = roomMsgMod.From(msg)
		res = tx.Save(roomMsgMod)
		return res.Error
	})
	if err != nil {
		return db.SqlError2Error(err)
	}
	return nil
}

func (s *sqlMetaStore) GetRoomMessage(ctx context.Context, msgId int64) (*types.RoomMessage, error) {
	defer trace.StartRegion(ctx, "metastore.sql.GetRoomMessage").End()
	roomMsg := &db.RoomMessage{}
	res := s.WithNamespace(ctx).Where("id = ?", msgId).First(roomMsg)
	if res.Error != nil {
		return nil, db.SqlError2Error(res.Error)
	}
	return roomMsg.To(), nil
}

func (s *sqlMetaStore) DeleteRoomMessages(ctx context.Context, roomId int64) error {
	defer trace.StartRegion(ctx, "metastore.sql.DeleteRoomMessage").End()
	err := s.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := namespaceQuery(ctx, tx).Where("room_id = ?", roomId).Delete(&db.RoomMessage{})
		return res.Error
	})
	return db.SqlError2Error(err)
}

func (s *sqlMetaStore) WithNamespaceOfTable(ctx context.Context, table string) *gorm.DB {
	ns := types.GetNamespace(ctx)
	if ns.String() == types.DefaultNamespaceValue {
		return s.WithContext(ctx)
	}
	return s.WithContext(ctx).Where(fmt.Sprintf("%s.namespace = ?", table), ns.String())
}

func (s *sqlMetaStore) WithNamespace(ctx context.Context) *gorm.DB {
	ns := types.GetNamespace(ctx)
	if ns.String() == types.DefaultNamespaceValue {
		return s.WithContext(ctx)
	}
	return s.WithContext(ctx).Where("namespace = ?", ns.String())
}

func (s *sqlMetaStore) WithPagination(ctx context.Context) *gorm.DB {
	page := types.GetPagination(ctx)
	return s.WithContext(ctx).Where("limit = ? AND offset = ?", page.PageSize, (page.Page-1)*page.PageSize)
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

func updateEntryAndCleanURICache(tx *gorm.DB, entryMod *db.Object) error {
	// clean uri cache
	uri := &db.ObjectURI{}
	res := tx.Where("oid = ?", entryMod.ID).First(uri)
	if res.Error != nil {
		if !errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return res.Error
		}
	} else {
		res = tx.Where("oid = ?", entryMod.ID).Delete(&db.ObjectURI{})
		if res.Error != nil {
			return res.Error
		}
		if entryMod.IsGroup {
			res = tx.Where("uri LIKE ?", uri.Uri+"%").Delete(&db.ObjectURI{})
			if res.Error != nil {
				return res.Error
			}
		}
	}

	return updateEntryWithVersion(tx, entryMod)
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

func listEntryIdsWithLabelMatcher(ctx context.Context, tx *gorm.DB, labelMatch types.LabelMatch) ([]int64, error) {
	tx = namespaceQuery(ctx, tx.WithContext(ctx))
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
	} else {
		tx = tx.Where("parent_id > 0")
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
	if filter.IsGroup != nil {
		tx = tx.Where("is_group = ?", *filter.IsGroup)
	}
	if filter.CreatedAtStart != nil {
		tx = tx.Where("created_at >= ?", *filter.CreatedAtStart)
	}
	if filter.CreatedAtEnd != nil {
		tx = tx.Where("created_at < ?", *filter.CreatedAtEnd)
	}
	if filter.ModifiedAtStart != nil {
		tx = tx.Where("modified_at >= ?", *filter.ModifiedAtStart)
	}
	if filter.ModifiedAtEnd != nil {
		tx = tx.Where("modified_at < ?", *filter.ModifiedAtEnd)
	}
	if filter.FuzzyName != "" {
		tx = tx.Where("name LIKE ?", "%"+filter.FuzzyName+"%")
	}
	return tx
}

func enOrder(tx *gorm.DB, order *types.EntryOrder) *gorm.DB {
	if order != nil {
		orderStr := order.Order.String()
		if order.Desc {
			orderStr += " DESC"
		}
		tx = tx.Order(orderStr)
	}
	return tx
}

func docQueryFilter(tx *gorm.DB, filter types.DocFilter) *gorm.DB {
	if filter.ParentID > 0 {
		tx = tx.Where("document.parent_entry_id = ?", filter.ParentID)
	}
	if filter.Marked != nil {
		tx = tx.Where("document.marked = ?", *filter.Marked)
	}
	if filter.Unread != nil {
		tx = tx.Where("document.unread = ?", *filter.Unread)
	}
	if filter.Source != "" {
		tx = tx.Where("document.source = ?", filter.Source)
	}
	if filter.CreatedAtStart != nil {
		tx = tx.Where("document.created_at >= ?", *filter.CreatedAtStart)
	}
	if filter.CreatedAtEnd != nil {
		tx = tx.Where("document.created_at < ?", *filter.CreatedAtEnd)
	}
	if filter.ChangedAtStart != nil {
		tx = tx.Where("document.changed_at >= ?", *filter.ChangedAtStart)
	}
	if filter.ChangedAtEnd != nil {
		tx = tx.Where("document.changed_at < ?", *filter.ChangedAtEnd)
	}
	if filter.FuzzyName != "" {
		tx = tx.Where("document.name LIKE ?", "%"+filter.FuzzyName+"%")
	}
	return tx
}

func docOrder(tx *gorm.DB, order *types.DocumentOrder) *gorm.DB {
	if order != nil {
		orderStr := order.Order.String()
		if order.Desc {
			orderStr += " DESC"
		}
		tx = tx.Order(orderStr)
	} else {
		tx = tx.Order("created_at DESC")
	}
	return tx
}

func namespaceQuery(ctx context.Context, tx *gorm.DB) *gorm.DB {
	ns := types.GetNamespace(ctx)
	if ns.String() == types.DefaultNamespaceValue {
		return tx
	}
	return tx.Where("namespace = ?", ns.String())
}

func workflowScore(ctx context.Context, workflow string, tx *gorm.DB) int {
	var (
		totalCount  int64
		totalFailed int64
		totalError  int64
	)

	err := namespaceQuery(ctx, tx).WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&db.WorkflowJob{}).Where("workflow = ? AND status = ?", workflow, "error").Order("created_at DESC").Limit(100).Count(&totalError)
		if res.Error != nil {
			return res.Error
		}
		res = tx.Model(&db.WorkflowJob{}).Where("workflow = ? AND status = ?", workflow, "failed").Order("created_at DESC").Limit(100).Count(&totalFailed)
		if res.Error != nil {
			return res.Error
		}
		res = tx.Model(&db.WorkflowJob{}).Where("workflow = ?", workflow).Order("created_at DESC").Limit(100).Count(&totalFailed)
		if res.Error != nil {
			return res.Error
		}
		return nil
	})
	if err != nil {
		return 0
	}
	if totalCount == 0 {
		return 100
	}

	totalFailed += totalError
	if totalFailed >= totalCount {
		return 0
	}

	return int((totalCount - totalFailed) / totalCount)
}

func labelSearchKey(k, v string) string {
	return fmt.Sprintf("%s=%s", k, v)
}
