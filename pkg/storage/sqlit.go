package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/storage/db"
	"github.com/basenana/nanafs/pkg/storage/db/migrate"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
	"sync"

	_ "modernc.org/sqlite"
)

const (
	SqliteMeta = "sqlite"
)

type sqliteMetaStore struct {
	db        *sqlx.DB
	dbPath    string
	nextInode int64
	mux       sync.RWMutex
	logger    *zap.SugaredLogger
}

var _ MetaStore = &sqliteMetaStore{}

func (s *sqliteMetaStore) GetObject(ctx context.Context, id string) (*types.Object, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	defer utils.TraceRegion(ctx, "sqlite.getobject")()
	obj, err := db.GetObjectByID(ctx, s.db, id)
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
	objList, err := db.ListObjectChildren(ctx, s.db, filter)
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
	if obj.Inode == 0 {
		obj.Inode = uint64(s.nextInode)
		s.nextInode += 1
		s.logger.Debugw("set object inode", "id", obj.ID, "inode", obj.Inode)
	}
	if err := db.SaveObject(ctx, s.db, parent, obj); err != nil {
		s.logger.Errorw("save object failed", "id", obj.ID, "err", err.Error())
		return err
	}
	return nil
}

func (s *sqliteMetaStore) DestroyObject(ctx context.Context, src, parent, obj *types.Object) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	defer utils.TraceRegion(ctx, "sqlite.destroyobject")()
	err := db.DeleteObject(ctx, s.db, src, parent, obj)
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
	children, err := db.ListObjectChildren(ctx, s.db, types.Filter{ParentID: obj.ID})
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
	err := db.SaveChangeParentObject(ctx, s.db, srcParent, dstParent, obj, opt)
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
	err := db.SaveMirroredObject(ctx, s.db, srcObj, dstParent, object)
	if err != nil {
		s.logger.Errorw("mirror object failed", "id", object.ID, "err", err.Error())
		return err
	}
	return nil
}

func (s *sqliteMetaStore) SaveContent(ctx context.Context, obj *types.Object, cType types.Kind, version string, content interface{}) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	defer utils.TraceRegion(ctx, "sqlite.savecontent")()
	rawData, err := json.Marshal(content)
	if err != nil {
		s.logger.Errorw("marshal object content failed", "id", obj.ID, "err", err.Error())
		return err
	}
	obj.Size = int64(len(rawData))
	err = db.UpdateObjectContent(ctx, s.db, obj, string(cType), version, rawData)
	if err != nil {
		s.logger.Errorw("save object content failed", "id", obj.ID, "err", err.Error())
		return err
	}
	return nil
}

func (s *sqliteMetaStore) LoadContent(ctx context.Context, obj *types.Object, cType types.Kind, version string, content interface{}) error {
	s.mux.RLock()
	defer s.mux.RUnlock()
	defer utils.TraceRegion(ctx, "sqlite.loadcontent")()
	contentRaw, err := db.GetObjectContent(ctx, s.db, obj.ID, string(cType), version)
	if err != nil {
		s.logger.Errorw("get object content failed", "id", obj.ID, "err", err.Error())
		return err
	}
	return json.Unmarshal(contentRaw, content)
}

func (s *sqliteMetaStore) DeleteContent(ctx context.Context, obj *types.Object, cType types.Kind, version string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	defer utils.TraceRegion(ctx, "sqlite.deletecontent")()
	err := db.DeleteObjectContent(ctx, s.db, obj.ID)
	if err != nil {
		s.logger.Errorw("delete object content failed", "id", obj.ID, "err", err.Error())
		return err
	}
	return err
}

func newSqliteMetaStore(meta config.Meta) (*sqliteMetaStore, error) {
	dbConn, err := sqlx.Open("sqlite", meta.Path)
	if err != nil {
		return nil, err
	}

	if err = dbConn.Ping(); err != nil {
		return nil, err
	}

	mig := migrate.NewMigrateManager(dbConn)
	if err = mig.UpgradeHead(); err != nil {
		return nil, fmt.Errorf("migrate db failed: %s", err.Error())
	}

	maxInode, err := db.CurrentMaxInode(context.Background(), dbConn)
	if err != nil {
		return nil, fmt.Errorf("query inode failed: %s", err.Error())
	}

	return &sqliteMetaStore{
		db:        dbConn,
		dbPath:    meta.Path,
		nextInode: maxInode + 1,
		logger:    logger.NewLogger("sqlite"),
	}, nil
}
