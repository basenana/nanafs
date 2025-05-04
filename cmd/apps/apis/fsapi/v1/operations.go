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

package v1

import (
	"context"
	"errors"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/types"
	"google.golang.org/protobuf/types/known/timestamppb"
	"runtime/trace"
)

const (
	entryNameMaxLength = 255
)

func (s *servicesV1) getGroupTree(ctx context.Context, namespace string) (*GetGroupTreeResponse_GroupEntry, error) {
	nsRoot, err := s.core.NamespaceRoot(ctx, namespace)
	if err != nil {
		return nil, err
	}

	children, err := s.listEntryChildren(ctx, namespace, nsRoot.ID, &types.EntryOrder{Order: types.EntryName})
	if err != nil {
		return nil, err
	}
	root := &GetGroupTreeResponse_GroupEntry{
		Name:     nsRoot.Name,
		Children: make([]*GetGroupTreeResponse_GroupEntry, 0, len(children)),
	}
	for _, child := range children {
		if !child.IsGroup {
			continue
		}
		grp, err := s.listGroupEntry(ctx, namespace, child.Name, child.ID)
		if err != nil {
			return nil, err
		}
		root.Children = append(root.Children, grp)
	}
	return root, nil
}

func (s *servicesV1) listGroupEntry(ctx context.Context, namespace string, name string, parentID int64) (*GetGroupTreeResponse_GroupEntry, error) {
	children, err := s.listEntryChildren(ctx, namespace, parentID, &types.EntryOrder{Order: types.EntryName})
	if err != nil {
		return nil, err
	}
	result := &GetGroupTreeResponse_GroupEntry{
		Name:     name,
		Children: make([]*GetGroupTreeResponse_GroupEntry, 0, len(children)),
	}
	for _, child := range children {
		if child.IsGroup {
			continue
		}
		grp, err := s.listGroupEntry(ctx, namespace, child.Name, child.ID)
		if err != nil {
			return nil, err
		}
		result.Children = append(result.Children, grp)
	}
	return result, nil
}

func (s *servicesV1) getEntryDetails(ctx context.Context, namespace string, id int64) (*EntryDetail, []*Property, error) {
	en, err := s.core.GetEntry(ctx, namespace, id)
	if err != nil {
		return nil, nil, err
	}
	parent, err := s.core.GetEntry(ctx, namespace, en.ParentID)
	if err != nil {
		// TODO
		s.logger.Warnw("get entry parent detail failed", "entry", id, "err", err)
	}

	properties := make(map[string]types.PropertyItem)

	access := &EntryDetail_Access{Uid: en.Access.UID, Gid: en.Access.GID}
	for _, perm := range en.Access.Permissions {
		access.Permissions = append(access.Permissions, string(perm))
	}

	ed := &EntryDetail{
		Id:         en.ID,
		Name:       en.Name,
		Aliases:    en.Aliases,
		Kind:       string(en.Kind),
		IsGroup:    en.IsGroup,
		Size:       en.Size,
		Version:    en.Version,
		Namespace:  en.Namespace,
		Storage:    en.Storage,
		Access:     access,
		CreatedAt:  timestamppb.New(en.CreatedAt),
		ChangedAt:  timestamppb.New(en.ChangedAt),
		ModifiedAt: timestamppb.New(en.ModifiedAt),
		AccessAt:   timestamppb.New(en.AccessAt),
	}

	if parent != nil {
		baseProperties, err := s.meta.ListEntryProperties(ctx, namespace, en.ParentID)
		if err != nil {
			return nil, nil, err
		}
		for k, p := range baseProperties.Fields {
			if p.Encoded {
				continue
			}
			properties[k] = p
		}
		ed.Parent = toEntryInfo(parent)
	}

	ps, err := s.meta.ListEntryProperties(ctx, namespace, id)
	if err != nil {
		return nil, nil, err
	}

	for k, p := range ps.Fields {
		if p.Encoded {
			continue
		}
		properties[k] = p
	}

	pl := make([]*Property, 0)
	for k, item := range properties {
		pl = append(pl, &Property{
			Key:     k,
			Value:   item.Value,
			Encoded: item.Encoded,
		})
	}
	return ed, pl, nil
}

func (s *servicesV1) listEntryChildren(ctx context.Context, namespace string, entryId int64, order *types.EntryOrder, filters ...types.Filter) ([]*types.Entry, error) {
	grp, err := s.core.OpenGroup(ctx, namespace, entryId)
	if err != nil {
		return nil, err
	}
	children, err := grp.ListChildren(ctx, order, filters...)
	if err != nil {
		return nil, err
	}

	return children, nil
}

func (s *servicesV1) InitNamespace(ctx context.Context, namespace string) error {
	defer trace.StartRegion(ctx, "fs.commander.InitNamespace").End()
	s.logger.Infow("init entry of namespace", "namespace", namespace)

	if len(namespace) > entryNameMaxLength {
		return types.ErrNameTooLong
	}

	return s.core.CreateNamespace(ctx, namespace)
}

func (s *servicesV1) MirrorEntry(ctx context.Context, namespace string, srcEntryId, dstParentId int64, attr types.EntryAttr) (*types.Entry, error) {
	defer trace.StartRegion(ctx, "fs.commander.MirrorEntry").End()
	if len(attr.Name) > entryNameMaxLength {
		return nil, types.ErrNameTooLong
	}

	oldEntry, err := s.core.FindEntry(ctx, namespace, dstParentId, attr.Name)
	if err != nil && !errors.Is(err, types.ErrNotFound) {
		s.logger.Errorw("check entry error", "srcEntry", srcEntryId, "err", err.Error())
		return nil, err
	}
	if oldEntry != nil {
		return nil, types.ErrIsExist
	}

	entry, err := s.core.MirrorEntry(ctx, namespace, srcEntryId, dstParentId, attr)
	if err != nil {
		s.logger.Errorw("mirror entry failed", "src", srcEntryId, "err", err.Error())
		return nil, err
	}
	s.logger.Debugw("mirror entry", "src", srcEntryId, "dstParent", dstParentId, "entry", entry.ID)

	return entry, nil
}

func (s *servicesV1) ChangeEntryParent(ctx context.Context, namespace string, targetId, oldParentId, newParentId int64, newName string, opt types.ChangeParentAttr) error {
	defer trace.StartRegion(ctx, "fs.commander.ChangeEntryParent").End()
	if len(newName) > entryNameMaxLength {
		return types.ErrNameTooLong
	}

	// need source dir WRITE
	oldParent, err := s.core.GetEntry(ctx, namespace, oldParentId)
	if err != nil {
		return err
	}
	if err = core.IsAccess(oldParent.Access, opt.Uid, opt.Gid, 0x2); err != nil {
		return err
	}
	// need dst dir WRITE
	newParent, err := s.core.GetEntry(ctx, namespace, newParentId)
	if err != nil {
		return err
	}
	if err = core.IsAccess(newParent.Access, opt.Uid, opt.Gid, 0x2); err != nil {
		return err
	}

	target, err := s.core.GetEntry(ctx, namespace, targetId)
	if err != nil {
		return err
	}
	if opt.Uid != 0 && opt.Uid != oldParent.Access.UID && opt.Uid != target.Access.UID && oldParent.Access.HasPerm(types.PermSticky) {
		return types.ErrNoPerm
	}

	var existObjId *int64
	existObj, err := s.core.FindEntry(ctx, namespace, newParentId, newName)
	if err != nil {
		if !errors.Is(err, types.ErrNotFound) {
			s.logger.Errorw("new name verify failed", "old", targetId, "newParent", newParentId, "newName", newName, "err", err)
			return err
		}
	}

	if existObj != nil {
		if opt.Uid != 0 && opt.Uid != newParent.Access.UID && opt.Uid != existObj.Access.UID && newParent.Access.HasPerm(types.PermSticky) {
			return types.ErrNoPerm
		}
		eid := existObj.ID
		existObjId = &eid
	}

	s.logger.Debugw("change entry parent", "target", targetId, "existObj", existObjId, "oldParent", oldParentId, "newParent", newParentId, "newName", newName)
	err = s.core.ChangeEntryParent(ctx, namespace, targetId, existObjId, oldParentId, newParentId, newName, types.ChangeParentAttr{
		Replace:  opt.Replace,
		Exchange: opt.Exchange,
	})
	if err != nil {
		s.logger.Errorw("change object parent failed", "target", targetId, "newParent", newParentId, "newName", newName, "err", err)
		return err
	}
	return nil
}

func (s *servicesV1) queryEntryProperties(ctx context.Context, namespace string, entryID, parentID int64) ([]*Property, error) {
	var (
		properties types.Properties
		err        error
	)
	if parentID != core.RootEntryID {
		properties, err = s.meta.ListEntryProperties(ctx, namespace, parentID)
		if err != nil {
			return nil, err
		}
		s.logger.Infow("list entry properties", "entry", entryID, "parentID", parentID, "got", len(properties.Fields))
	}
	entryProperties, err := s.meta.ListEntryProperties(ctx, namespace, entryID)
	if err != nil {
		return nil, err
	}

	if properties.Fields == nil {
		properties.Fields = make(map[string]types.PropertyItem)
	}
	for k, p := range entryProperties.Fields {
		properties.Fields[k] = p
	}
	result := make([]*Property, 0, len(properties.Fields))
	for key, p := range properties.Fields {
		result = append(result, &Property{
			Key:     key,
			Value:   p.Value,
			Encoded: p.Encoded,
		})
	}

	return result, nil
}
