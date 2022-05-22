package db

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/jmoiron/sqlx"
)

func GetObjectByID(ctx context.Context, db *sqlx.DB, oid string) (*types.Object, error) {
	object := &Object{}
	if err := db.Get(object, getObjectByIDSQL, oid); err != nil {
		return nil, dbError2Error(err)
	}

	result := &types.Object{}
	if err := json.Unmarshal(object.Data, result); err != nil {
		return nil, err
	}
	return result, nil
}

func GetObjectByName(ctx context.Context, db *sqlx.DB, parent *types.Object, name string) (*types.Object, error) {
	object := &Object{}
	if err := db.Get(object, getObjectByNameSQL, parent.ParentID, name); err != nil {
		return nil, dbError2Error(err)
	}

	result := &types.Object{}
	if err := json.Unmarshal(object.Data, result); err != nil {
		return nil, err
	}
	return result, nil
}

func ListObjectChildren(ctx context.Context, db *sqlx.DB, parent *types.Object, filter types.Filter) ([]*types.Object, error) {
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

func SaveObject(ctx context.Context, db *sqlx.DB, parent, object *types.Object) error {
	return withTx(ctx, db, func(ctx context.Context, tx *sqlx.Tx) error {
		parentModel, err := queryRawObject(ctx, tx, parent.ID)
		if err != nil {
			return err
		}
		parentModel.Update(parent)
		if err = saveRawObject(ctx, tx, parentModel, updateObjectSQL); err != nil {
			return err
		}

		objectModel, err := queryRawObject(ctx, tx, object.ID)
		if err != nil && err != types.ErrNotFound {
			return err
		}
		execSql := updateObjectSQL
		if err == types.ErrNotFound {
			execSql = insertObjectSQL
		}
		objectModel.Update(object)
		if err = saveRawObject(ctx, tx, objectModel, execSql); err != nil {
			return err
		}

		if err = updateObjectLabels(ctx, tx, object); err != nil {
			return err
		}

		return nil
	})
}

func SaveMirroredObject(ctx context.Context, db *sqlx.DB, srcObj, dstParent, object *types.Object) error {
	return withTx(ctx, db, func(ctx context.Context, tx *sqlx.Tx) error {
		return nil
	})
}

func SaveMovedObject(ctx context.Context, db *sqlx.DB, srcParent, dstParent, object *types.Object) error {
	return withTx(ctx, db, func(ctx context.Context, tx *sqlx.Tx) error {
		return nil
	})
}

func DeleteObject(ctx context.Context, db *sqlx.DB, parent, object *types.Object) error {
	return withTx(ctx, db, func(ctx context.Context, tx *sqlx.Tx) error {
		attrs := map[string]interface{}{"id": object.ID}
		if _, err := tx.NamedExec(deleteObjectSQL, attrs); err != nil {
			return fmt.Errorf("delete object failed: %s", err.Error())
		}
		if _, err := tx.NamedExec(deleteObjectLabelSQL, attrs); err != nil {
			return fmt.Errorf("clean object failed: %s", err.Error())
		}
		return nil
	})
}

func queryRawObject(ctx context.Context, tx *sqlx.Tx, id string) (*Object, error) {
	object := &Object{}
	if err := tx.Get(object, getObjectByIDSQL, id); err != nil {
		return nil, dbError2Error(err)
	}
	return object, nil
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

func saveRawObject(ctx context.Context, tx *sqlx.Tx, object *Object, execSql string) error {
	_, err := tx.NamedExec(execSql, object)
	if err != nil {
		return fmt.Errorf("save object record failed: %s", err.Error())
	}
	return nil
}

func updateObjectLabels(ctx context.Context, tx *sqlx.Tx, obj *types.Object) error {
	// FIXME: rebuild label
	if _, err := tx.NamedExec(deleteObjectLabelSQL, obj); err != nil {
		return fmt.Errorf("clean object label failed: %s", err.Error())
	}
	for _, kv := range obj.Labels.Labels {
		if _, err := tx.NamedExec(insertObjectLabelSQL, ObjectLabel{
			ID:    obj.ID,
			Key:   kv.Key,
			Value: kv.Value,
		}); err != nil {
			return fmt.Errorf("insert object label failed: %s", err.Error())
		}
	}
	return nil
}

type combo func(ctx context.Context, tx *sqlx.Tx) error

func withTx(ctx context.Context, db *sqlx.DB, fn combo) error {
	tx, err := db.Beginx()
	if err != nil {
		return err
	}

	if err = fn(ctx, tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
