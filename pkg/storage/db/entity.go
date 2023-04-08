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

package db

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/basenana/nanafs/pkg/types"
)

type Entity struct {
	*gorm.DB
}

func NewDbEntity(db *gorm.DB) (*Entity, error) {
	ent := &Entity{DB: db}

	ctx, canF := context.WithTimeout(context.TODO(), time.Second*10)
	defer canF()

	if err := Migrate(db); err != nil {
		return nil, err
	}

	_, err := ent.SystemInfo(ctx)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	if err != nil {
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
	res := queryFilter(e.DB, filter).Find(&objectList)
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

func (e *Entity) DeleteObject(ctx context.Context, srcObj, dstParent, obj *types.Object) error {
	return e.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if srcObj != nil {
			if srcObj.RefCount == 0 {
				if err := deleteRawObject(tx, srcObj); err != nil {
					return err
				}
			} else {
				if err := saveRawObject(tx, srcObj); err != nil {
					return err
				}
			}
		}
		if err := saveRawObject(tx, dstParent); err != nil {
			return err
		}
		if err := deleteRawObject(tx, obj); err != nil {
			return err
		}
		return nil
	})
}

func (e *Entity) GetPluginRecord(ctx context.Context, plugin types.PlugScope, rid string, record interface{}) error {
	pd := PluginData{
		PluginName: plugin.PluginName,
		RecordId:   rid,
	}
	res := e.DB.WithContext(ctx).Where("plugin_name = ? AND record_id = ?", plugin.PluginName, rid).First(&pd)
	if res.Error != nil {
		return res.Error
	}
	return json.Unmarshal([]byte(pd.Content), record)
}

func (e *Entity) ListPluginRecords(ctx context.Context, plugin types.PlugScope, groupId string) ([]string, error) {
	result := make([]string, 0)
	res := e.DB.WithContext(ctx).Model(&PluginData{}).Select("record_id").Where("plugin_name = ? AND group_id = ?", plugin.PluginName, groupId).Find(&result)
	return result, res.Error
}

func (e *Entity) SavePluginRecord(ctx context.Context, plugin types.PlugScope, groupId, rid string, record interface{}) error {
	pd := PluginData{
		PluginName: plugin.PluginName,
		GroupId:    groupId,
		RecordId:   rid,
	}

	needCreate := false
	res := e.DB.WithContext(ctx).Where("plugin_name = ? AND record_id = ?", plugin.PluginName, rid).First(&pd)
	if res.Error != nil {
		if res.Error != gorm.ErrRecordNotFound {
			return res.Error
		}
		needCreate = true
	}

	rawContent, err := json.Marshal(record)
	if err != nil {
		return err
	}

	pd.Version = plugin.Version
	pd.Type = plugin.PluginType
	pd.Content = string(rawContent)

	if needCreate {
		res = e.DB.WithContext(ctx).Create(&pd)
		if res.Error != nil {
			return res.Error
		}
		return nil
	}

	res = e.DB.WithContext(ctx).Save(pd)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func (e *Entity) DeletePluginRecord(ctx context.Context, plugin types.PlugScope, rid string) error {
	pd := PluginData{
		PluginName: plugin.PluginName,
		RecordId:   rid,
	}

	res := e.DB.WithContext(ctx).Where("plugin_name = ? AND record_id = ?", plugin.PluginName, rid).First(&pd)
	if res.Error != nil {
		return res.Error
	}

	res = e.DB.WithContext(ctx).Delete(pd)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func (e *Entity) NextSegmentID(ctx context.Context) (int64, error) {
	return availableChunkSegID(e.DB.WithContext(ctx))
}

func (e *Entity) InsertChunkSegment(ctx context.Context, obj *types.Object, seg types.ChunkSeg) error {
	return e.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tx.Create(&ObjectChunk{
			ID:       seg.ID,
			OID:      obj.ID,
			ChunkID:  seg.ChunkID,
			Off:      seg.Off,
			Len:      seg.Len,
			State:    seg.State,
			AppendAt: time.Now().UnixNano(),
		})
		if seg.Off+seg.Len > obj.Size {
			obj.Size = seg.Off + seg.Len
		}
		obj.ModifiedAt = time.Now()
		return saveRawObject(tx, obj)
	})
}

func (e *Entity) ListChunkSegments(ctx context.Context, oid, chunkID int64) ([]types.ChunkSeg, error) {
	segments := make([]ObjectChunk, 0)
	res := e.DB.WithContext(ctx).Where("oid = ? AND chunk_id = ?", oid, chunkID).Order("append_at").Find(&segments)
	if res.Error != nil {
		return nil, res.Error
	}
	result := make([]types.ChunkSeg, len(segments))
	for i, seg := range segments {
		result[i] = types.ChunkSeg{
			ID:       seg.ID,
			ChunkID:  seg.ChunkID,
			ObjectID: seg.OID,
			Off:      seg.Off,
			Len:      seg.Len,
			State:    seg.State,
		}

	}
	return result, nil
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

	oa := &ObjectPermission{}
	ext := &ObjectExtend{}
	ol := make([]Label, 0)

	res := db.Where("oid = ?", objModel.ID).First(oa)
	if res.Error != nil {
		return nil, res.Error
	}

	res = db.Where("id = ?", objModel.ID).First(ext)
	if res.Error != nil {
		return nil, res.Error
	}

	res = db.Where("oid = ?", objModel.ID).Find(&ol)
	if res.Error != nil {
		return nil, res.Error
	}

	obj.Access = oa.ToAccess()
	obj.ExtendData = ext.ToExtData()

	for _, kv := range ol {
		obj.Labels.Labels = append(obj.Labels.Labels, types.Label{Key: kv.Key, Value: kv.Value})
	}
	return obj, nil
}

func saveRawObject(tx *gorm.DB, obj *types.Object) error {
	var (
		objModel = &Object{ID: obj.ID}
		oaModel  = &ObjectPermission{OID: obj.ID}
		extModel = &ObjectExtend{ID: obj.ID}
	)
	res := tx.First(objModel)
	if res.Error != nil && res.Error != gorm.ErrRecordNotFound {
		return res.Error
	}

	objModel.Update(obj)
	oaModel.Update(obj)
	extModel.Update(obj)
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
		res = tx.Create(extModel)
		if res.Error != nil {
			return res.Error
		}
		return nil
	}
	res = tx.Save(objModel)
	if res.Error != nil {
		return res.Error
	}
	res = tx.Save(oaModel)
	if res.Error != nil {
		return res.Error
	}
	res = tx.Save(extModel)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func deleteRawObject(tx *gorm.DB, obj *types.Object) error {
	var (
		objModel = &Object{ID: obj.ID}
		extModel = &ObjectExtend{ID: obj.ID}
	)
	res := tx.First(objModel)
	if res.Error != nil && res.Error != gorm.ErrRecordNotFound {
		return res.Error
	}

	res = tx.Delete(objModel)
	if res.Error != nil {
		return res.Error
	}
	res = tx.Delete(extModel)
	if res.Error != nil {
		return res.Error
	}
	res = tx.Where("oid = ?", obj.ID).Delete(&ObjectPermission{})
	if res.Error != nil {
		return res.Error
	}
	res = tx.Where("oid = ?", obj.ID).Delete(&Label{})
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
	res = tx.Save(info)
	if res.Error != nil {
		return 0, res.Error
	}

	return info.Inode, nil
}

func availableChunkSegID(tx *gorm.DB) (int64, error) {
	info := &SystemInfo{}
	res := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(info)
	if res.Error != nil {
		return 0, res.Error
	}

	info.ChunkSeg += 1
	res = tx.Save(info)
	if res.Error != nil {
		return 0, res.Error
	}

	return info.ChunkSeg, nil
}

func updateObjectLabels(tx *gorm.DB, obj *types.Object) error {
	labelModels := make([]Label, 0)
	res := tx.Where("oid = ?", obj.ID).Find(&labelModels)
	if res.Error != nil {
		return res.Error
	}

	labelsMap := map[string]string{}
	for _, kv := range obj.Labels.Labels {
		labelsMap[kv.Key] = kv.Value
	}

	needCreateLabelModels := make([]Label, 0)
	needUpdateLabelModels := make([]Label, 0)
	needDeleteLabelModels := make([]Label, 0)
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
		needCreateLabelModels = append(needCreateLabelModels, Label{RefType: "object", RefID: obj.ID, Key: k, Value: v, SearchKey: labelSearchKey(k, v)})
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
}

func listObjectWithLabelMatcher(ctx context.Context, db *gorm.DB, labelMatch types.LabelMatch) ([]*types.Object, error) {
	db = db.WithContext(ctx)
	includeSearchKeys := make([]string, len(labelMatch.Include))
	for i, inKey := range labelMatch.Include {
		includeSearchKeys[i] = labelSearchKey(inKey.Key, inKey.Value)
	}

	var includeLabels []Label
	res := db.Select("oid").Where("search_key IN ?", includeSearchKeys).Find(&includeLabels)
	if res.Error != nil {
		return nil, res.Error
	}

	var excludeLabels []Label
	res = db.Select("oid").Where("key IN ?", labelMatch.Exclude).Find(&excludeLabels)
	if res.Error != nil {
		return nil, res.Error
	}

	targetObjects := make(map[int64]struct{})
	for _, in := range includeLabels {
		targetObjects[in.RefID] = struct{}{}
	}
	for _, ex := range excludeLabels {
		delete(targetObjects, ex.RefID)
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
