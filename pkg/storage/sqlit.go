package storage

import (
	"context"
	"encoding/json"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

const (
	SqliteMeta = "sqlite"
)

type sqliteMetaStore struct {
	db     *sqlx.DB
	dbPath string
	logger *zap.SugaredLogger
}

var _ MetaStore = &sqliteMetaStore{}

func (s *sqliteMetaStore) GetObject(ctx context.Context, id string) (*types.Object, error) {
	defer utils.TraceRegion(ctx, "sqlite.getobject")()
	return queryObject(ctx, s.db, id)
}

func (s *sqliteMetaStore) ListObjects(ctx context.Context, filter Filter) ([]*types.Object, error) {
	defer utils.TraceRegion(ctx, "sqlite.listobject")()
	return listObject(ctx, s.db, filter)
}

func (s *sqliteMetaStore) SaveObject(ctx context.Context, obj *types.Object) error {
	defer utils.TraceRegion(ctx, "sqlite.saveobject")()
	return saveObject(ctx, s.db, obj)
}

func (s *sqliteMetaStore) DestroyObject(ctx context.Context, obj *types.Object) error {
	defer utils.TraceRegion(ctx, "sqlite.destroyobject")()
	return deleteObject(ctx, s.db, obj.ID)
}

func (s *sqliteMetaStore) ListChildren(ctx context.Context, obj *types.Object) (Iterator, error) {
	defer utils.TraceRegion(ctx, "sqlite.listchildren")()
	children, err := listObject(ctx, s.db, Filter{ParentID: obj.ID})
	if err != nil {
		return nil, err
	}
	return &iterator{objects: children}, nil
}

func (s *sqliteMetaStore) ChangeParent(ctx context.Context, old *types.Object, parent *types.Object) error {
	defer utils.TraceRegion(ctx, "sqlite.changeparent")()
	old.ParentID = parent.ID
	return saveObject(ctx, s.db, old)
}

func (s *sqliteMetaStore) SaveContent(ctx context.Context, obj *types.Object, cType types.Kind, version string, content interface{}) error {
	defer utils.TraceRegion(ctx, "sqlite.savecontent")()
	rawData, err := json.Marshal(content)
	if err != nil {
		return err
	}
	return updateObjectContent(ctx, s.db, Content{
		ID:      obj.ID,
		Kind:    string(cType),
		Version: version,
		Data:    rawData,
	})
}

func (s *sqliteMetaStore) LoadContent(ctx context.Context, obj *types.Object, cType types.Kind, version string, content interface{}) error {
	defer utils.TraceRegion(ctx, "sqlite.loadcontent")()
	contentModel, err := queryObjectContent(ctx, s.db, obj.ID, string(cType), version)
	if err != nil {
		return err
	}
	return json.Unmarshal(contentModel.Data, content)
}

func (s *sqliteMetaStore) DeleteContent(ctx context.Context, obj *types.Object, cType types.Kind, version string) error {
	defer utils.TraceRegion(ctx, "sqlite.deletecontent")()
	return deleteObjectContent(ctx, s.db, obj.ID)
}

func newSqliteMetaStore(meta config.Meta) (*sqliteMetaStore, error) {
	db, err := sqlx.Open("sqlite3", meta.Path)
	if err != nil {
		return nil, err
	}

	total := 0
	err = db.Get(&total, "SELECT count(*) FROM object")
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			if err = initTables(db); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	return &sqliteMetaStore{
		db:     db,
		dbPath: meta.Path,
		logger: logger.NewLogger("sqlite"),
	}, nil
}
