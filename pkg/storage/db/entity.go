package db

import (
	"context"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type Entity struct {
	*gorm.DB
}

func NewDbEntity(db *gorm.DB) (*Entity, error) {
	ent := &Entity{DB: db}

	ctx, canF := context.WithTimeout(context.TODO(), time.Second*10)
	defer canF()

	_, err := ent.SystemInfo(ctx)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	if err != nil {
		if err = db.AutoMigrate(dbModels...); err != nil {
			return nil, err
		}
		sysInfo := &SystemInfo{
			FsID:  uuid.New().String(),
			Inode: 1024,
		}
		if res := db.WithContext(ctx).Create(sysInfo); res.Error != nil {
			return nil, res.Error
		}
	}

	return ent, nil
}

func (e *Entity) GetObjectByID(ctx context.Context, oid int64) (*types.Object, error) {
	var (
		obj = &Object{ID: oid}
	)
	res := e.WithContext(ctx).First(obj)
	if res.Error != nil {
		return nil, res.Error
	}

	return assembleObject(e.DB, obj)
}

func (e *Entity) ListObjectChildren(ctx context.Context, filter types.Filter) ([]*types.Object, error) {
	if len(filter.Label.Include) > 0 || len(filter.Label.Exclude) > 0 {
		return listObjectWithLabelMatcher(ctx, e.DB, filter.Label)
	}

	objectList := make([]Object, 0)
	res := queryFilter(e.DB, filter).Find(objectList)
	if res.Error != nil {
		return nil, res.Error
	}

	result := make([]*types.Object, len(objectList))
	for i, obj := range objectList {
		o, err := assembleObject(e.DB, &obj)
		if err != nil {
			return nil, err
		}
		result[i] = o
	}

	return result, nil
}

func (e *Entity) SaveObject(ctx context.Context, parent, object *types.Object) error {
	return e.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if parent != nil {
			if err := saveRawObject(tx, parent); err != nil {
				return err
			}
		}
		if err := saveRawObject(tx, object); err != nil {
			return err
		}
		if err := updateObjectLabels(tx, object); err != nil {
			return err
		}
		return nil
	})
}

func (e *Entity) SaveMirroredObject(ctx context.Context, srcObj, dstParent, object *types.Object) error {
	return e.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := saveRawObject(tx, srcObj); err != nil {
			return err
		}
		if err := saveRawObject(tx, dstParent); err != nil {
			return err
		}
		if err := saveRawObject(tx, object); err != nil {
			return err
		}
		if err := updateObjectLabels(tx, object); err != nil {
			return err
		}
		return nil
	})
}

func (e *Entity) SaveChangeParentObject(ctx context.Context, srcParent, dstParent, obj *types.Object, opt types.ChangeParentOption) error {
	return e.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := saveRawObject(tx, srcParent); err != nil {
			return err
		}
		if err := saveRawObject(tx, dstParent); err != nil {
			return err
		}
		if err := saveRawObject(tx, obj); err != nil {
			return err
		}
		return nil
	})
}

func (e *Entity) DeleteObject(ctx context.Context, srcParent, dstParent, obj *types.Object) error {
	return e.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := saveRawObject(tx, srcParent); err != nil {
			return err
		}
		if err := saveRawObject(tx, dstParent); err != nil {
			return err
		}
		if err := saveRawObject(tx, obj); err != nil {
			return err
		}
		return nil
	})
}

func (e *Entity) SystemInfo(ctx context.Context) (*SystemInfo, error) {
	info := &SystemInfo{}
	res := e.WithContext(ctx).First(info)
	if res.Error != nil {
		return nil, res.Error
	}
	return info, nil
}

func assembleObject(db *gorm.DB, objModel *Object) (*types.Object, error) {
	obj := objModel.Object()

	oa := &ObjectAccess{}
	ol := make([]ObjectLabel, 0)

	res := db.Where("id", "=", objModel.ID).First(oa)
	if res.Error != nil {
		return nil, res.Error
	}

	res = db.Where("oid", "=", objModel.ID).Find(&ol)
	if res.Error != nil {
		return nil, res.Error
	}

	obj.Access = oa.ToAccess()

	for _, kv := range ol {
		obj.Labels.Labels = append(obj.Labels.Labels, types.Label{Key: kv.Key, Value: kv.Value})
	}
	return obj, nil
}

func saveRawObject(tx *gorm.DB, obj *types.Object) error {
	var (
		objModel = &Object{ID: obj.ID}
		oaModel  = &ObjectAccess{ID: obj.ID}
	)
	res := tx.First(objModel)
	if res.Error != nil && res.Error != gorm.ErrRecordNotFound {
		return res.Error
	}

	objModel.Update(obj)
	oaModel.Update(obj)
	if res.Error == gorm.ErrRecordNotFound {
		ino, err := availableInode(tx)
		if err != nil {
			return err
		}
		obj.Inode = ino
		objModel.Inode = ino

		res = tx.Create(objModel)
		if res.Error != nil {
			return res.Error
		}
		res = tx.Create(oaModel)
		if res.Error != nil {
			return res.Error
		}
	}
	res = tx.Updates(objModel)
	if res.Error != nil {
		return res.Error
	}
	res = tx.Updates(oaModel)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func availableInode(tx *gorm.DB) (uint64, error) {
	info := &SystemInfo{}
	res := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(info)
	if res.Error != nil {
		return 0, res.Error
	}

	info.Inode += 1
	res = tx.Updates(info)
	if res.Error != nil {
		return 0, res.Error
	}

	return info.Inode, nil
}

func updateObjectLabels(tx *gorm.DB, obj *types.Object) error {
	labelModels := make([]ObjectLabel, 0)
	res := tx.Where("oid = ?", obj.ID).Find(labelModels)
	if res.Error != nil {
		return res.Error
	}

	labelsMap := map[string]string{}
	for _, kv := range obj.Labels.Labels {
		labelsMap[kv.Key] = kv.Value
	}

	needCreateLabelModels := make([]ObjectLabel, 0)
	needUpdateLabelModels := make([]ObjectLabel, 0)
	needDeleteLabelModels := make([]ObjectLabel, 0)
	for i := range labelModels {
		oldKv := labelModels[i]
		if newV, ok := labelsMap[oldKv.Key]; ok {
			if oldKv.Value != newV {
				oldKv.Value = newV
				needUpdateLabelModels = append(needUpdateLabelModels, oldKv)
			}
			delete(labelsMap, oldKv.Key)
			continue
		}
		needDeleteLabelModels = append(needDeleteLabelModels, oldKv)
	}

	for k, v := range labelsMap {
		needCreateLabelModels = append(needCreateLabelModels, ObjectLabel{OID: obj.ID, Key: k, Value: v, SearchKey: labelSearchKey(k, v)})
	}

	if len(needCreateLabelModels) > 0 {
		for i := range needCreateLabelModels {
			label := needCreateLabelModels[i]
			res = tx.Create(label)
			if res.Error != nil {
				return res.Error
			}
		}
	}
	if len(needUpdateLabelModels) > 0 {
		for i := range needUpdateLabelModels {
			label := needUpdateLabelModels[i]
			res = tx.Updates(label)
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
}

func listObjectWithLabelMatcher(ctx context.Context, db *gorm.DB, labelMatch types.LabelMatch) ([]*types.Object, error) {
	db = db.WithContext(ctx)
	includeSearchKeys := make([]string, len(labelMatch.Include))
	for i, inKey := range labelMatch.Include {
		includeSearchKeys[i] = labelSearchKey(inKey.Key, inKey.Value)
	}

	var includeLabels []ObjectLabel
	res := db.Select("oid").Where("search_key IN ?", includeSearchKeys).Find(&includeLabels)
	if res.Error != nil {
		return nil, res.Error
	}

	var excludeLabels []ObjectLabel
	res = db.Select("oid").Where("key IN ?", labelMatch.Exclude).Find(&excludeLabels)
	if res.Error != nil {
		return nil, res.Error
	}

	targetObjects := make(map[int64]struct{})
	for _, in := range includeLabels {
		targetObjects[in.OID] = struct{}{}
	}
	for _, ex := range excludeLabels {
		delete(targetObjects, ex.OID)
	}

	idList := make([]int64, 0, len(targetObjects))
	for i := range targetObjects {
		idList = append(idList, i)
	}

	result := make([]*types.Object, 0, len(targetObjects))
	res = db.Where("id IN ?", idList).Find(&result)
	if res.Error != nil {
		return nil, res.Error
	}
	return result, nil
}