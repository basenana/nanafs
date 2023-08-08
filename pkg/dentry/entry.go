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

package dentry

import (
	"context"
	"github.com/basenana/nanafs/pkg/types"
)

const (
	RootEntryID     = 1
	RootEntryName   = "root"
	externalStorage = "[ext]"
)

type Entry interface {
	ID() int64
	GetExtendData(ctx context.Context) (types.ExtendData, error)
	UpdateExtendData(ctx context.Context, ed types.ExtendData) error
	GetExtendField(ctx context.Context, fKey string) (*string, error)
	SetExtendField(ctx context.Context, fKey, fVal string) error
	RemoveExtendField(ctx context.Context, fKey string) error
	RuleMatched(ctx context.Context, ruleSpec types.Rule) bool
	IsGroup() bool
	IsMirror() bool
	Group() Group

	// Deprecated: remove this
	Metadata() *types.Metadata
}
