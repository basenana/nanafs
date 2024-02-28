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
	kind2PdMapping = map[types.Kind]EntryKind{
		types.RawKind:           EntryKind_RawKind,
		types.GroupKind:         EntryKind_GroupKind,
		types.SmartGroupKind:    EntryKind_SmartGroupKind,
		types.FIFOKind:          EntryKind_FIFOKind,
		types.SocketKind:        EntryKind_SocketKind,
		types.SymLinkKind:       EntryKind_SymLinkKind,
		types.BlkDevKind:        EntryKind_BlkDevKind,
		types.CharDevKind:       EntryKind_CharDevKind,
		types.ExternalGroupKind: EntryKind_ExternalGroupKind,
	}
	pdKind2KindMapping = map[EntryKind]types.Kind{
		EntryKind_RawKind:           types.RawKind,
		EntryKind_GroupKind:         types.GroupKind,
		EntryKind_SmartGroupKind:    types.SmartGroupKind,
		EntryKind_FIFOKind:          types.FIFOKind,
		EntryKind_SocketKind:        types.SocketKind,
		EntryKind_SymLinkKind:       types.SymLinkKind,
		EntryKind_BlkDevKind:        types.BlkDevKind,
		EntryKind_CharDevKind:       types.CharDevKind,
		EntryKind_ExternalGroupKind: types.ExternalGroupKind,
	}
)

func entryKind2Pd(k types.Kind) EntryKind {
	return kind2PdMapping[k]
}

func pdKind2EntryKind(k EntryKind) types.Kind {
	return pdKind2KindMapping[k]
}

func entryInfo(en *types.Metadata) *EntryInfo {
	return &EntryInfo{
		Id:         en.ID,
		Name:       en.Name,
		Kind:       entryKind2Pd(en.Kind),
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
		Kind:       entryKind2Pd(en.Kind),
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
