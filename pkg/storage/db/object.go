package db

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/jmoiron/sqlx"
)

func GetObjectByID(ctx context.Context, db *sqlx.DB, oid string) (result *types.Object, err error) {
	err = withTx(ctx, db, func(ctx context.Context, tx *sqlx.Tx) error {
		object := &Object{}
		if err := Query(tx, object).Where("id", "=", oid).Get(object); err != nil {
			return err
		}
		result, err = buildObject(ctx, tx, object)
		if err != nil {
			return err
		}
		return nil
	})
	return
}

func ListObjectChildren(ctx context.Context, db *sqlx.DB, filter types.Filter) ([]*types.Object, error) {
	if len(filter.Label.Include) > 0 || len(filter.Label.Exclude) > 0 {
		return listObjectWithLabelMatcher(ctx, db, filter.Label)
	}
	var (
		result []*types.Object
		fm     = types.FilterObjectMapper(filter)
	)
	err := withTx(ctx, db, func(ctx context.Context, tx *sqlx.Tx) error {
		q := Query(tx, &Object{})
		if len(fm) > 0 {
			for queryKey, val := range fm {
				q = q.Where(queryKey, "=", val)
			}
		}

		objList := make([]Object, 0)
		if err := q.List(&objList); err != nil {
			return err
		}

		for _, o := range objList {
			obj, err := buildObject(ctx, tx, &o)
			if err != nil {
				return err
			}
			result = append(result, obj)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func SaveObject(ctx context.Context, db *sqlx.DB, parent, object *types.Object) error {
	return withTx(ctx, db, func(ctx context.Context, tx *sqlx.Tx) error {
		var err error
		if parent != nil {
			if err = saveRawObject(ctx, tx, parent); err != nil {
				return err
			}
		}

		if err = saveRawObject(ctx, tx, object); err != nil {
			return err
		}

		if err = updateObjectRef(ctx, tx, object); err != nil {
			return err
		}
		return nil
	})
}

func SaveMirroredObject(ctx context.Context, db *sqlx.DB, srcObj, dstParent, object *types.Object) error {
	return withTx(ctx, db, func(ctx context.Context, tx *sqlx.Tx) error {
		if err := saveRawObject(ctx, tx, srcObj); err != nil {
			return err
		}

		if err := saveRawObject(ctx, tx, dstParent); err != nil {
			return err
		}

		if err := saveRawObject(ctx, tx, object); err != nil {
			return err
		}
		if err := updateObjectRef(ctx, tx, object); err != nil {
			return err
		}
		return nil
	})
}

func SaveChangeParentObject(ctx context.Context, db *sqlx.DB, srcParent, dstParent, obj *types.Object, opt types.ChangeParentOption) error {
	return withTx(ctx, db, func(ctx context.Context, tx *sqlx.Tx) error {
		if err := saveRawObject(ctx, tx, srcParent); err != nil {
			return err
		}

		if err := saveRawObject(ctx, tx, dstParent); err != nil {
			return err
		}

		if err := saveRawObject(ctx, tx, obj); err != nil {
			return err
		}
		return nil
	})
}

func DeleteObject(ctx context.Context, db *sqlx.DB, src, parent, object *types.Object) error {
	return withTx(ctx, db, func(ctx context.Context, tx *sqlx.Tx) error {
		if err := saveRawObject(ctx, tx, parent); err != nil {
			return err
		}

		if src != nil {
			if src.RefCount > 0 {
				if err := saveRawObject(ctx, tx, src); err != nil {
					return err
				}
			} else {
				if err := deleteObject(ctx, tx, src.ID); err != nil {
					return err
				}
			}
		}

		if !object.IsGroup() && object.RefCount > 0 {
			return saveRawObject(ctx, tx, object)
		}
		return deleteObject(ctx, tx, object.ID)
	})
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

func buildObject(ctx context.Context, tx *sqlx.Tx, objModel *Object) (*types.Object, error) {
	result := objModel.Object()

	oa := &ObjectAccess{}
	ol := make([]ObjectLabel, 0)

	if err := Query(tx, oa).Where("id", "=", objModel.ID).Get(oa); err != nil {
		return nil, fmt.Errorf("query object access model failed: %s", err.Error())
	}
	if err := Query(tx, &ObjectLabel{}).Where("id", "=", objModel.ID).List(&ol); err != nil {
		return nil, fmt.Errorf("query object access model failed: %s", err.Error())
	}

	result.Access = oa.ToAccess()

	for _, kv := range ol {
		result.Labels.Labels = append(result.Labels.Labels, types.Label{Key: kv.Key, Value: kv.Value})
	}
	return result, nil
}

func saveRawObject(ctx context.Context, tx *sqlx.Tx, obj *types.Object) error {
	var (
		objModel = &Object{}
		err      error
	)
	if err = Query(tx, objModel).Where("id", "=", obj.ID).Get(objModel); err != nil && err != sql.ErrNoRows {
		return err
	}
	objModel.Update(obj)
	if err == nil {
		return Exec(tx, objModel).Update()
	}
	return Exec(tx, objModel).Insert()
}

func deleteObject(ctx context.Context, tx *sqlx.Tx, oid string) error {
	if err := Exec(tx, &Object{}).Delete(oid); err != nil {
		return fmt.Errorf("delete object failed: %s", err.Error())
	}
	if err := Exec(tx, &ObjectAccess{}).Delete(oid); err != nil {
		return fmt.Errorf("delete object access failed: %s", err.Error())
	}
	if err := Exec(tx, &ObjectLabel{}).Delete(oid); err != nil {
		return fmt.Errorf("delete object label failed: %s", err.Error())
	}

	if err := deleteObjectContent(ctx, tx, oid); err != nil {
		return fmt.Errorf("clean object content failed: %s", err.Error())
	}
	return nil
}

func updateObjectRef(ctx context.Context, tx *sqlx.Tx, obj *types.Object) error {
	// FIXME: not rebuild
	if err := Exec(tx, &ObjectAccess{}).Delete(obj.ID); err != nil {
		return err
	}
	oa := &ObjectAccess{}
	oa.Update(obj)
	if err := Exec(tx, oa).Insert(); err != nil {
		return fmt.Errorf("insert object label failed: %s", err.Error())
	}

	if err := Exec(tx, &ObjectLabel{}).Delete(obj.ID); err != nil {
		return err
	}
	for _, kv := range obj.Labels.Labels {
		if err := Exec(tx, &ObjectLabel{
			ID:    obj.ID,
			Key:   kv.Key,
			Value: kv.Value,
		}).Insert(); err != nil {
			return fmt.Errorf("insert object label failed: %s", err.Error())
		}
	}
	return nil
}

func GetObjectContent(ctx context.Context, db *sqlx.DB, id, kind, version string) ([]byte, error) {
	cnt := ObjectContent{}
	if err := withTx(ctx, db, func(ctx context.Context, tx *sqlx.Tx) error {
		queryErr := Query(tx, cnt).Where("id", "=", id).Where("kind", "=", kind).Where("version", "=", version).Get(&cnt)
		if queryErr != nil {
			if queryErr == sql.ErrNoRows {
				return types.ErrNotFound
			}
			return queryErr
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return cnt.Data, nil
}

func UpdateObjectContent(ctx context.Context, db *sqlx.DB, obj *types.Object, kind, version string, raw []byte) error {
	return withTx(ctx, db, func(ctx context.Context, tx *sqlx.Tx) error {
		cnt := ObjectContent{
			ID:      obj.ID,
			Kind:    kind,
			Version: version,
		}
		if err := Query(tx, cnt).Where("id", "=", obj.ID).Where("kind", "=", kind).Where("version", "=", version).Get(&cnt); err != nil {
			if err == sql.ErrNoRows {
				cnt.Data = raw
				return Exec(tx, &cnt).Insert()
			}
			return err
		}
		cnt.Data = raw
		obj.Size = int64(len(raw))
		if err := saveRawObject(ctx, tx, obj); err != nil {
			return err
		}

		return Exec(tx, &cnt).Update()
	})
}

func DeleteObjectContent(ctx context.Context, db *sqlx.DB, id string) error {
	return withTx(ctx, db, func(ctx context.Context, tx *sqlx.Tx) error {
		return deleteObjectContent(ctx, tx, id)
	})
}

func deleteObjectContent(ctx context.Context, tx *sqlx.Tx, id string) error {
	cnt := ObjectContent{}
	if err := Query(tx, cnt).Where("id", "=", id).Get(&cnt); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}
	return Exec(tx, &cnt).Delete(cnt.ID)
}

type combo func(ctx context.Context, tx *sqlx.Tx) error

func withTx(ctx context.Context, db *sqlx.DB, fn combo) error {
	tx, err := db.Beginx()
	if err != nil {
		return err
	}

	if err = fn(ctx, tx); err != nil {
		_ = tx.Rollback()
		return dbError2Error(err)
	}
	if err = tx.Commit(); err != nil {
		_ = tx.Rollback()
		return err
	}
	return nil
}
