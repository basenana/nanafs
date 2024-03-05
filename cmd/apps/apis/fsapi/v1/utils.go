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
	"github.com/basenana/nanafs/pkg/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	kind2PdMapping = map[types.Kind]struct{}{
		types.RawKind:           {},
		types.GroupKind:         {},
		types.SmartGroupKind:    {},
		types.FIFOKind:          {},
		types.SocketKind:        {},
		types.SymLinkKind:       {},
		types.BlkDevKind:        {},
		types.CharDevKind:       {},
		types.ExternalGroupKind: {},
	}
)

func pdKind2EntryKind(k string) types.Kind {
	_, ok := kind2PdMapping[types.Kind(k)]
	if !ok {
		return types.RawKind
	}
	return types.Kind(k)
}

func entryInfo(en *types.Metadata) *EntryInfo {
	return &EntryInfo{
		Id:         en.ID,
		Name:       en.Name,
		Kind:       string(en.Kind),
		CreatedAt:  timestamppb.New(en.CreatedAt),
		ChangedAt:  timestamppb.New(en.ChangedAt),
		ModifiedAt: timestamppb.New(en.ModifiedAt),
		AccessAt:   timestamppb.New(en.AccessAt),
	}
}

func entryDetail(en, parent *types.Metadata) *EntryDetail {
	access := &EntryDetail_Access{Uid: en.Access.UID, Gid: en.Access.GID}
	for _, perm := range en.Access.Permissions {
		access.Permissions = append(access.Permissions, string(perm))
	}

	return &EntryDetail{
		Id:         en.ID,
		Name:       en.Name,
		Aliases:    en.Aliases,
		Parent:     entryInfo(parent),
		Kind:       string(en.Kind),
		KindMap:    en.KindMap,
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
}
