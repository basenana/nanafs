package storage

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/jmoiron/sqlx"
	"time"
)

const (
	createObjectTableSQL = `
CREATE TABLE object (
    id VARCHAR(32),
	name VARCHAR(512),
	aliases VARCHAR(512),
	parent_id VARCHAR(32),
	ref_id VARCHAR(32),
	kind VARCHAR(128),
	hash VARCHAR(512),
	size INTEGER(64),
	inode INTEGER(64),
	namespace VARCHAR(512),
	created_at DATETIME,
	changed_at DATETIME,
	modified_at DATETIME,
	access_at DATETIME,
	data BLOB,
	PRIMARY KEY (id)
);
`
	createObjectLabelTableSQL = `
CREATE TABLE object_label (
    id VARCHAR(32),
    key VARCHAR(128),
    value VARCHAR(512),
    CONSTRAINT pk_lid PRIMARY KEY (id,key)
);
`
	createObjectContentTableSQL = `
CREATE TABLE object_content (
    id VARCHAR(32),
    kind VARCHAR(128),
    version VARCHAR(512),
    data BLOB,
    CONSTRAINT pk_cid PRIMARY KEY (id,kind,version)
);
`
	insertObjectSQL = `
INSERT INTO object (
	id, name, aliases, parent_id, ref_id, kind, hash, size, inode, namespace,
	created_at, changed_at, modified_at, access_at, data
) VALUES (
	:id, :name, :aliases, :parent_id, :ref_id, :kind, :hash, :size, :inode,
	:namespace, :created_at, :changed_at, :modified_at, :access_at, :data
);
`
	updateObjectSQL = `
UPDATE object
SET
	name=:name,
	aliases=:aliases,
	parent_id=:parent_id,
	ref_id=:ref_id,
	kind=:kind,
	hash=:hash,
	size=:size,
	inode=:inode,
	namespace=:namespace,
	created_at=:created_at,
	changed_at=:changed_at,
	modified_at=:modified_at,
	access_at=:access_at,
	data=:data
WHERE
	id=:id
`
	deleteObjectSQL = `
DELETE FROM object WHERE id=:id;
`
	insertObjectLabelSQL = `
INSERT INTO object_label (
	id, key, value
) VALUES (
	:id, :key, :value
);
`
	deleteObjectLabelSQL = `
DELETE FROM object_label WHERE id=:id;
`
	insertObjectContentSQL = `
INSERT INTO object_content (
	id, kind, version, data
) VALUES (
	:id, :kind, :version, :data
);
`
	updateObjectContentSQL = `
UPDATE object_content
SET
	data=:data
WHERE
	id=:id AND kind=:kind AND version=:version
`
	deleteObjectContentSQL = `
DELETE FROM object_custom WHERE id=:id;
`
)

func initTables(db *sqlx.DB) error {
	tx, err := db.Beginx()
	if err != nil {
		return err
	}

	tx.MustExec(createObjectTableSQL)
	tx.MustExec(createObjectLabelTableSQL)
	tx.MustExec(createObjectContentTableSQL)

	return tx.Commit()
}

type Object struct {
	ID         string    `db:"id"`
	Name       string    `db:"name"`
	Aliases    string    `db:"aliases"`
	ParentID   string    `db:"parent_id"`
	RefID      string    `db:"ref_id"`
	Kind       string    `db:"kind"`
	Hash       string    `db:"hash"`
	Size       int64     `db:"size"`
	Inode      uint64    `db:"inode"`
	Namespace  string    `db:"namespace"`
	CreatedAt  time.Time `db:"created_at"`
	ChangedAt  time.Time `db:"changed_at"`
	ModifiedAt time.Time `db:"modified_at"`
	AccessAt   time.Time `db:"access_at"`
	Data       []byte    `db:"data"`
}

type ObjectLabel struct {
	ID    string `db:"id"`
	Key   string `db:"key"`
	Value string `db:"value"`
}

func queryRawObject(ctx context.Context, db *sqlx.DB, id string) (*Object, error) {
	object := &Object{}
	if err := db.Get(object, "SELECT * FROM object WHERE id=$1", id); err != nil {
		return nil, dbError2Error(err)
	}
	return object, nil
}

func queryObject(ctx context.Context, db *sqlx.DB, id string) (*types.Object, error) {
	object, err := queryRawObject(ctx, db, id)
	if err != nil {
		return nil, err
	}

	result := &types.Object{}
	if err := json.Unmarshal(object.Data, result); err != nil {
		return nil, err
	}

	return result, nil
}

func saveObject(ctx context.Context, db *sqlx.DB, obj *types.Object) error {
	object, err := queryRawObject(ctx, db, obj.ID)
	if err != nil && err != types.ErrNotFound {
		return fmt.Errorf("query object befor save got error: %s", err.Error())
	}

	execSql := updateObjectSQL
	if err == types.ErrNotFound {
		execSql = insertObjectSQL
		object = &Object{}
	}

	copyObject2DbModel(obj, object)

	tx, err := db.Beginx()
	if err != nil {
		return fmt.Errorf("save object failed, error tx begin: %s", err.Error())
	}

	_, err = tx.NamedExec(execSql, object)
	if err != nil {
		return fmt.Errorf("save object record failed: %s", err.Error())
	}

	// rebuild label
	if _, err = tx.NamedExec(deleteObjectLabelSQL, obj); err != nil {
		return fmt.Errorf("clean object label failed: %s", err.Error())
	}
	for _, kv := range obj.Labels.Labels {
		if _, err = tx.NamedExec(insertObjectLabelSQL, ObjectLabel{
			ID:    obj.ID,
			Key:   kv.Key,
			Value: kv.Value,
		}); err != nil {
			return fmt.Errorf("insert object label failed: %s", err.Error())
		}
	}
	return tx.Commit()
}

func deleteObject(ctx context.Context, db *sqlx.DB, id string) error {
	tx, err := db.Beginx()
	if err != nil {
		return err
	}
	attrs := map[string]interface{}{"id": id}
	if _, err = tx.NamedExec(deleteObjectSQL, attrs); err != nil {
		return fmt.Errorf("delete object failed: %s", err.Error())
	}
	if _, err = tx.NamedExec(deleteObjectLabelSQL, attrs); err != nil {
		return fmt.Errorf("clean object failed: %s", err.Error())
	}
	return dbError2Error(tx.Commit())
}

func listObject(ctx context.Context, db *sqlx.DB, filter types.Filter) ([]*types.Object, error) {
	if len(filter.Label.Include) > 0 || len(filter.Label.Exclude) > 0 {
		return listObjectWithLabelMatcher(ctx, db, filter.Label)
	}

	objList := make([]Object, 0)
	fm := types.FilterObjectMapper(filter)
	queryBuf := bytes.Buffer{}
	queryBuf.WriteString("SELECT * FROM object")
	if len(fm) > 0 {
		queryBuf.WriteString(" WHERE ")
		count := len(fm)
		for queryKey := range fm {
			queryBuf.WriteString(fmt.Sprintf("%s=:%s ", queryKey, queryKey))
			count -= 1
			if count > 0 {
				queryBuf.WriteString("AND ")
			}
		}
	}

	execSQL := queryBuf.String()
	nstmt, err := db.PrepareNamed(execSQL)
	if err != nil {
		return nil, fmt.Errorf("prepare sql failed, sql=%s, err=%s", execSQL, err.Error())
	}
	err = nstmt.Select(&objList, fm)
	if err != nil {
		return nil, fmt.Errorf("list object failed: %s", err.Error())
	}

	result := make([]*types.Object, len(objList))
	for i, o := range objList {
		obj := &types.Object{}
		if err = json.Unmarshal(o.Data, obj); err != nil {
			return nil, err
		}
		result[i] = obj
	}
	return result, nil
}

func currentMaxInode(ctx context.Context, db *sqlx.DB) (inode int64, err error) {
	var total int64
	err = db.Get(&total, `SELECT count(*) FROM "object"`)
	if err != nil {
		return
	}
	if total == 0 {
		return
	}
	query := `SELECT max(inode) FROM "object"`
	err = db.Get(&inode, query)
	return
}

func listObjectWithLabelMatcher(ctx context.Context, db *sqlx.DB, labelMatch types.LabelMatch) ([]*types.Object, error) {
	queryBuf := bytes.Buffer{}
	queryBuf.WriteString("SELECT * FROM object WHERE ")

	for i, inKey := range labelMatch.Include {
		queryBuf.WriteString(fmt.Sprintf("id IN (SELECT id FROM object_lable WHERE key='%s' AND value='%s')", inKey.Key, inKey.Value))
		if i < len(labelMatch.Include) {
			queryBuf.WriteString(" AND ")
		}
	}
	for i, exKey := range labelMatch.Exclude {
		queryBuf.WriteString(fmt.Sprintf("id NOT IN (SELECT id FROM object_lable WHERE key='%s')", exKey))
		if i < len(labelMatch.Exclude) {
			queryBuf.WriteString(" AND ")
		}
	}

	result := make([]*types.Object, 0)
	err := db.Select(&result, queryBuf.String())
	return result, err
}

type Content struct {
	ID      string `db:"id"`
	Kind    string `db:"kind"`
	Version string `db:"version"`
	Data    []byte `db:"data"`
}

func queryObjectContent(ctx context.Context, db *sqlx.DB, id, kind, version string) (Content, error) {
	content := Content{}
	if err := db.Get(&content, "SELECT * FROM object_content WHERE id=$1 AND kind=$2 AND version=$3", id, kind, version); err != nil {
		return content, dbError2Error(err)
	}
	return content, nil
}

func updateObjectContent(ctx context.Context, db *sqlx.DB, content Content) error {
	old := Content{}
	execSql := updateObjectContentSQL
	err := db.Get(&old, "SELECT * FROM object_content WHERE id=$1 AND kind=$2 AND version=$3", content.ID, content.Kind, content.Version)
	if err == sql.ErrNoRows {
		execSql = insertObjectContentSQL
	} else {
		if bytes.Equal(old.Data, content.Data) {
			return nil
		}
	}
	if _, err := db.NamedExec(execSql, content); err != nil {
		return dbError2Error(err)
	}
	return nil
}

func deleteObjectContent(ctx context.Context, db *sqlx.DB, id string) error {
	if _, err := db.NamedExec(deleteObjectContentSQL, map[string]interface{}{"id": id}); err != nil {
		return dbError2Error(err)
	}
	return nil
}

func copyObject2DbModel(obj *types.Object, dbObj *Object) {
	dbObj.ID = obj.ID
	dbObj.Name = obj.Name
	dbObj.Aliases = obj.Aliases
	dbObj.ParentID = obj.ParentID
	dbObj.RefID = obj.RefID
	dbObj.Kind = string(obj.Kind)
	dbObj.Hash = obj.Hash
	dbObj.Size = obj.Size
	dbObj.Inode = obj.Inode
	dbObj.Namespace = obj.Namespace
	dbObj.CreatedAt = obj.CreatedAt
	dbObj.ChangedAt = obj.ChangedAt
	dbObj.ModifiedAt = obj.ModifiedAt
	dbObj.AccessAt = obj.AccessAt

	dbObj.Data, _ = json.Marshal(obj)
}
