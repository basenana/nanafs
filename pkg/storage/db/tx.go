package db

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/jmoiron/sqlx"
)

func GetObjectByID(ctx context.Context, db *sqlx.DB, parent *types.Object, oid string) (*types.Object, error) {
	return nil, nil
}

func GetObjectByName(ctx context.Context, db *sqlx.DB, parent *types.Object, name string) (*types.Object, error) {
	return nil, nil
}

func ListObjectChildren(ctx context.Context, db *sqlx.DB, parent *types.Object, name string) (*types.Object, error) {
	return nil, nil
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

func saveRawObject(ctx context.Context, tx *sqlx.Tx, object *Object, execSql string) error {
	_, err := tx.NamedExec(execSql, object)
	if err != nil {
		return fmt.Errorf("save object record failed: %s", err.Error())
	}
	return nil
}

func updateObjectLabels(ctx context.Context, tx *sqlx.Tx, obj *types.Object) error {
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
