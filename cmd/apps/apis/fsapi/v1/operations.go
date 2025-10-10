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
	"path"
	"runtime/trace"
	"sort"
)

const (
	entryNameMaxLength = 255
)

func (s *servicesV1) getGroupTree(ctx context.Context, namespace string) (*GetGroupTreeResponse_GroupEntry, error) {
	nsRoot, err := s.core.NamespaceRoot(ctx, namespace)
	if err != nil {
		return nil, err
	}

	children, err := s.listEntryChildren(ctx, namespace, nsRoot.ID)
	if err != nil {
		return nil, err
	}
	root := &GetGroupTreeResponse_GroupEntry{
		Uri:      "/",
		Name:     "/",
		Children: make([]*GetGroupTreeResponse_GroupEntry, 0, len(children)),
	}
	for _, child := range children {
		if !child.IsGroup {
			continue
		}
		grp, err := s.listGroupEntry(ctx, namespace, child.Name, path.Join(root.Uri, child.Name), child.ID)
		if err != nil {
			return nil, err
		}
		root.Children = append(root.Children, grp)
	}
	return root, nil
}

func (s *servicesV1) listGroupEntry(ctx context.Context, namespace string, name, groupUri string, groupID int64) (*GetGroupTreeResponse_GroupEntry, error) {
	children, err := s.listEntryChildren(ctx, namespace, groupID)
	if err != nil {
		return nil, err
	}
	result := &GetGroupTreeResponse_GroupEntry{
		Name:     name,
		Uri:      groupUri,
		Children: nil,
	}

	if len(children) > 0 {
		result.Children = make([]*GetGroupTreeResponse_GroupEntry, 0, len(children))
		for _, child := range children {
			if !child.IsGroup {
				continue
			}
			grp, err := s.listGroupEntry(ctx, namespace, child.Name, path.Join(groupUri, child.Name), child.ID)
			if err != nil {
				return nil, err
			}
			result.Children = append(result.Children, grp)
		}
	}
	return result, nil
}

func (s *servicesV1) getEntryDetails(ctx context.Context, namespace, uri, name string, id int64) (*EntryDetail, *Property, error) {
	en, err := s.core.GetEntry(ctx, namespace, id)
	if err != nil {
		return nil, nil, err
	}

	doc := types.DocumentProperties{}
	err = s.meta.GetEntryProperties(ctx, namespace, types.PropertyTypeDocument, id, &doc)
	if err != nil {
		return nil, nil, err
	}
	details := toEntryDetail(uri, name, en, doc)

	properties := &types.Properties{}
	err = s.meta.GetEntryProperties(ctx, namespace, types.PropertyTypeProperty, id, &properties)
	if err != nil {
		return nil, nil, err
	}
	return details, &Property{
		Tags:       properties.Tags,
		Properties: properties.Properties,
	}, nil
}

func (s *servicesV1) listEntryChildren(ctx context.Context, namespace string, entryId int64) ([]*types.Entry, error) {
	grp, err := s.core.OpenGroup(ctx, namespace, entryId)
	if err != nil {
		return nil, err
	}
	children, err := grp.ListChildren(ctx)
	if err != nil {
		return nil, err
	}

	sort.Slice(children, func(i, j int) bool {
		return children[i].Name < children[j].Name
	})

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
	existCh, err := s.core.FindEntry(ctx, namespace, newParentId, newName)
	if err != nil {
		if !errors.Is(err, types.ErrNotFound) {
			s.logger.Errorw("new name verify failed", "old", targetId, "newParent", newParentId, "newName", newName, "err", err)
			return err
		}
	}

	if existCh != nil {
		existObj, err := s.core.GetEntry(ctx, namespace, existCh.ChildID)
		if err != nil {
			s.logger.Errorw("get exited entry for verify failed", "old", targetId, "newParent", newParentId, "newName", newName, "err", err)
			return err
		}
		if opt.Uid != 0 && opt.Uid != newParent.Access.UID && opt.Uid != existObj.Access.UID && newParent.Access.HasPerm(types.PermSticky) {
			return types.ErrNoPerm
		}
		eid := existObj.ID
		existObjId = &eid
	}

	s.logger.Debugw("change entry parent", "target", targetId, "existObj", existObjId, "oldParent", oldParentId, "newParent", newParentId, "newName", newName)
	err = s.core.ChangeEntryParent(ctx, namespace, targetId, existObjId, oldParentId, newParentId, target.Name, newName, types.ChangeParentAttr{
		Replace:  opt.Replace,
		Exchange: opt.Exchange,
	})
	if err != nil {
		s.logger.Errorw("change entry parent failed", "target", targetId, "newParent", newParentId, "newName", newName, "err", err)
		return err
	}
	return nil
}
