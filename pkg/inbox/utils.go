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

package inbox

import (
	"context"
	"errors"
	"fmt"

	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
)

const (
	inboxInternalGroupName = ".inbox"
)

func InitInboxInternalGroup(ctx context.Context, entryMgr dentry.Manager) error {
	root, err := entryMgr.Root(ctx)
	if err != nil {
		return err
	}
	rootGrp, err := entryMgr.OpenGroup(ctx, root.ID)
	if err != nil {
		return fmt.Errorf("open root group failed: %w", err)
	}
	group, err := rootGrp.FindEntry(ctx, inboxInternalGroupName)
	if err != nil && !errors.Is(err, types.ErrNotFound) {
		return err
	}
	if group != nil {
		return nil
	}

	group, err = entryMgr.CreateEntry(ctx, root.ID,
		types.EntryAttr{Name: inboxInternalGroupName, Kind: types.GroupKind})
	return nil
}

func FindInboxInternalGroup(ctx context.Context, entryMgr dentry.Manager) (*types.Entry, error) {
	root, err := entryMgr.Root(ctx)
	if err != nil {
		return nil, err
	}
	rootGrp, err := entryMgr.OpenGroup(ctx, root.ID)
	if err != nil {
		return nil, fmt.Errorf("open root group failed: %w", err)
	}
	group, err := rootGrp.FindEntry(ctx, inboxInternalGroupName)
	if err != nil && !errors.Is(err, types.ErrNotFound) {
		return nil, err
	}
	if group != nil {
		return group, nil
	}

	return group, nil
}
