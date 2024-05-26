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

package controller

import (
	"context"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
	"strings"
)

func buildGroupEntry(ctx context.Context, entryMgr dentry.Manager, entry *types.Metadata, showHidden bool) (*types.GroupEntry, error) {
	ge := &types.GroupEntry{
		Entry:    entry,
		Children: make([]*types.GroupEntry, 0),
	}
	parent, err := entryMgr.OpenGroup(ctx, entry.ID)
	if err != nil {
		return nil, err
	}

	children, err := parent.ListChildren(ctx, types.Filter{IsGroup: true})
	if err != nil {
		return nil, err
	}

	for _, ch := range children {
		if strings.HasPrefix(ch.Name, ".") && !showHidden {
			continue
		}
		nextGe, err := buildGroupEntry(ctx, entryMgr, ch, showHidden)
		if err != nil {
			return nil, err
		}
		ge.Children = append(ge.Children, nextGe)
	}
	return ge, nil
}
