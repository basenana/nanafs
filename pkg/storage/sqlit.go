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
	return db.GetObjectByID(ctx, s.db, id)
}

func (s *sqliteMetaStore) ListObjects(ctx context.Context, filter types.Filter) ([]*types.Object, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	defer utils.TraceRegion(ctx, "sqlite.listobject")()
	return db.ListObjectChildren(ctx, s.db, filter)
}

func (s *sqliteMetaStore) SaveObject(ctx context.Context, parent, obj *types.Object) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	defer utils.TraceRegion(ctx, "sqlite.saveobject")()
	if obj.Inode == 0 {
		obj.Inode = uint64(s.nextInode)
		s.nextInode += 1
	}
	return db.SaveObject(ctx, s.db, parent, obj)
}

func (s *sqliteMetaStore) DestroyObject(ctx context.Context, src, parent, obj *types.Object) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	defer utils.TraceRegion(ctx, "sqlite.destroyobject")()
	return db.DeleteObject(ctx, s.db, src, parent, obj)
}

func (s *sqliteMetaStore) ListChildren(ctx context.Context, obj *types.Object) (Iterator, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	defer utils.TraceRegion(ctx, "sqlite.listchildren")()
	children, err := db.ListObjectChildren(ctx, s.db, types.Filter{ParentID: obj.ID})
	if err != nil {
		return nil, err
	}
	return &iterator{objects: children}, nil
}

func (s *sqliteMetaStore) ChangeParent(ctx context.Context, srcParent, dstParent, existObj, obj *types.Object, opt types.ChangeParentOption) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	defer utils.TraceRegion(ctx, "sqlite.changeparent")()
	obj.ParentID = dstParent.ID
	return db.SaveChangeParentObject(ctx, s.db, srcParent, dstParent, existObj, obj, opt)
}

func (s *sqliteMetaStore) MirrorObject(ctx context.Context, srcObj, dstParent, object *types.Object) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	defer utils.TraceRegion(ctx, "sqlite.mirrorobject")()
	return db.SaveMirroredObject(ctx, s.db, srcObj, dstParent, object)
}

func (s *sqliteMetaStore) SaveContent(ctx context.Context, obj *types.Object, cType types.Kind, version string, content interface{}) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	defer utils.TraceRegion(ctx, "sqlite.savecontent")()
	rawData, err := json.Marshal(content)
	if err != nil {
		return err
	}
	obj.Size = int64(len(rawData))
	err = updateObjectContent(ctx, s.db, Content{
		ID:      obj.ID,
		Kind:    string(cType),
		Version: version,
		Data:    rawData,
	})
	if err != nil {
		return err
	}
	return saveObject(ctx, s.db, obj)
}

func (s *sqliteMetaStore) LoadContent(ctx context.Context, obj *types.Object, cType types.Kind, version string, content interface{}) error {
	s.mux.RLock()
	defer s.mux.RUnlock()
	defer utils.TraceRegion(ctx, "sqlite.loadcontent")()
	contentModel, err := queryObjectContent(ctx, s.db, obj.ID, string(cType), version)
	if err != nil {
		return err
	}
	return json.Unmarshal(contentModel.Data, content)
}

func (s *sqliteMetaStore) DeleteContent(ctx context.Context, obj *types.Object, cType types.Kind, version string) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	defer utils.TraceRegion(ctx, "sqlite.deletecontent")()
	return deleteObjectContent(ctx, s.db, obj.ID)
}

func newSqliteMetaStore(meta config.Meta) (*sqliteMetaStore, error) {
	db, err := sqlx.Open("sqlite", meta.Path)
	if err != nil {
		return nil, err
	}

	mig := migrate.NewMigrateManager(db)
	if err = mig.UpgradeHead(); err != nil {
		return nil, fmt.Errorf("migrate db failed: %s", err.Error())
	}

	maxInode, err := currentMaxInode(context.Background(), db)
	if err != nil {
		return nil, fmt.Errorf("query inode failed: %s", err.Error())
	}

	return &sqliteMetaStore{
		db:        db,
		dbPath:    meta.Path,
		nextInode: maxInode + 1,
		logger:    logger.NewLogger("sqlite"),
	}, nil
}
