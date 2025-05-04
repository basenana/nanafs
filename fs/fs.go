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

package fs

import (
	"context"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"go.uber.org/zap"
	"runtime/trace"
)

type FS struct {
	core   core.Core
	store  metastore.EntryStore
	dentry dentry.Cache
	logger *zap.SugaredLogger
}

func (f *FS) SetXAttr(ctx context.Context, namespace string, id int64, fKey string, fVal []byte) error {
	defer trace.StartRegion(ctx, "fs.commander.SetEntryEncodedProperty").End()
	f.logger.Debugw("set entry extend filed", "entry", id, "key", fKey)

	if err := f.store.AddEntryProperty(ctx, id, fKey, types.PropertyItem{Value: utils.EncodeBase64(fVal), Encoded: true}); err != nil {
		return err
	}
	return nil
}
