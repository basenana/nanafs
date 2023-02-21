package storage

import (
	"context"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/storage/db"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"sync"
)

const (
	SqliteMeta = "sqlite"
)

type sqliteMetaStore struct {
	dbEntity  *db.Entity
	dbPath    string
	nextInode int64
	mux       sync.RWMutex
	logger    *zap.SugaredLogger
}

var _ Meta = &sqliteMetaStore{}

func (s *sqliteMetaStore) GetObject(ctx context.Context, id int64) (*types.Object, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	defer utils.TraceRegion(ctx, "sqlite.getobject")()
	obj, err := s.dbEntity.GetObjectByID(ctx, id)
	if err != nil {
		s.logger.Errorw("query object by id failed", "id", id, "err", err.Error())
		return nil, err
	}
	return obj, nil
}

func (s *sqliteMetaStore) ListObjects(ctx context.Context, filter types.Filter) ([]*types.Object, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	defer utils.TraceRegion(ctx, "sqlite.listobject")()
	objList, err := s.dbEntity.ListObjectChildren(ctx, filter)
	if err != nil {
		s.logger.Errorw("list object failed", "filter", filter, "err", err.Error())
		return nil, err
	}
	return objList, nil
}

func (s *sqliteMetaStore) SaveObject(ctx context.Context, parent, obj *types.Object) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	defer utils.TraceRegion(ctx, "sqlite.saveobject")()
	if err := s.dbEntity.SaveObject(ctx, parent, obj); err != nil {
		s.logger.Errorw("save object failed", "id", obj.ID, "err", err.Error())
		return err
	}
	return nil
}

func (s *sqliteMetaStore) DestroyObject(ctx context.Context, src, parent, obj *types.Object) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	defer utils.TraceRegion(ctx, "sqlite.destroyobject")()
	err := s.dbEntity.DeleteObject(ctx, src, parent, obj)
	if err != nil {
		s.logger.Errorw("destroy object failed", "id", obj.ID, "err", err.Error())
		return err
	}
	return nil
}

func (s *sqliteMetaStore) ListChildren(ctx context.Context, obj *types.Object) (Iterator, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	defer utils.TraceRegion(ctx, "sqlite.listchildren")()
	children, err := s.dbEntity.ListObjectChildren(ctx, types.Filter{ParentID: obj.ID})
	if err != nil {
		s.logger.Errorw("list object children failed", "id", obj.ID, "err", err.Error())
		return nil, err
	}
	return &iterator{objects: children}, nil
}

func (s *sqliteMetaStore) ChangeParent(ctx context.Context, srcParent, dstParent, obj *types.Object, opt types.ChangeParentOption) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	defer utils.TraceRegion(ctx, "sqlite.changeparent")()
	obj.ParentID = dstParent.ID
	err := s.dbEntity.SaveChangeParentObject(ctx, srcParent, dstParent, obj, opt)
	if err != nil {
		s.logger.Errorw("change object parent failed", "id", obj.ID, "err", err.Error())
		return err
	}
	return nil
}

func (s *sqliteMetaStore) MirrorObject(ctx context.Context, srcObj, dstParent, object *types.Object) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	defer utils.TraceRegion(ctx, "sqlite.mirrorobject")()
	err := s.dbEntity.SaveMirroredObject(ctx, srcObj, dstParent, object)
	if err != nil {
		s.logger.Errorw("mirror object failed", "id", object.ID, "err", err.Error())
		return err
	}
	return nil
}

func (s *sqliteMetaStore) PluginRecorder(plugin types.PlugScope) PluginRecorder {
	return &sqlitePluginRecorder{
		sqliteMetaStore: s,
		plugin:          plugin,
	}
}

func (s *sqliteMetaStore) getPluginRecord(ctx context.Context, plugin types.PlugScope, rid string, record interface{}) error {
	//TODO implement me
	panic("implement me")
}

func (s *sqliteMetaStore) listPluginRecords(ctx context.Context, plugin types.PlugScope, groupId string) ([]string, error) {
	//TODO implement me
	panic("implement me")
}

func (s *sqliteMetaStore) savePluginRecord(ctx context.Context, plugin types.PlugScope, groupId, rid string, record interface{}) error {
	//TODO implement me
	panic("implement me")
}

func (s *sqliteMetaStore) deletePluginRecord(ctx context.Context, plugin types.PlugScope, rid string) error {
	//TODO implement me
	panic("implement me")
}

func newSqliteMetaStore(meta config.Meta) (*sqliteMetaStore, error) {
	dbObj, err := gorm.Open(sqlite.Open(meta.Path), &gorm.Config{})
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

	return &sqliteMetaStore{
		dbEntity: dbEnt,
		dbPath:   meta.Path,
		logger:   logger.NewLogger("sqlite"),
	}, nil
}

type sqlitePluginRecorder struct {
	*sqliteMetaStore
	plugin types.PlugScope
}

func (s *sqlitePluginRecorder) GetRecord(ctx context.Context, rid string, record interface{}) error {
	return s.getPluginRecord(ctx, s.plugin, rid, record)
}

func (s *sqlitePluginRecorder) ListRecords(ctx context.Context, groupId string) ([]string, error) {
	return s.listPluginRecords(ctx, s.plugin, groupId)
}

func (s *sqlitePluginRecorder) SaveRecord(ctx context.Context, groupId, rid string, record interface{}) error {
	return s.savePluginRecord(ctx, s.plugin, groupId, rid, record)
}

func (s *sqlitePluginRecorder) DeleteRecord(ctx context.Context, rid string) error {
	return s.deletePluginRecord(ctx, s.plugin, rid)
}
