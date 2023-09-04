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

package exec

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/pkg/workflow/jobrun"
	"github.com/basenana/nanafs/utils"
	"go.uber.org/zap"
	"io"
	"os"
	"path"
	"strings"
)

type entryInitOperator struct {
	entryID   int64
	entryMgr  dentry.Manager
	entryPath string
	logger    *zap.SugaredLogger
}

var _ jobrun.Operator = &entryInitOperator{}

func (e *entryInitOperator) Do(ctx context.Context, param *jobrun.Parameter) error {
	entryPath := e.entryPath
	if !path.IsAbs(e.entryPath) {
		entryPath = path.Join(param.Workdir, path.Base(e.entryPath))
	}

	enInfo, err := os.Stat(entryPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if enInfo != nil {
		return nil
	}

	if !strings.HasPrefix(entryPath, param.Workdir) {
		e.logger.Warnf("init entry unexpected, entryPath=%s, workdir=%s", entryPath, param.Workdir)
		return types.ErrNotFound
	}

	entry, err := e.entryMgr.GetEntry(ctx, e.entryID)
	if err != nil {
		return fmt.Errorf("load entry failed: %s", err)
	}
	f, err := e.entryMgr.Open(ctx, entry.ID, dentry.Attr{Read: true})
	if err != nil {
		return fmt.Errorf("open entry failed: %s", err)
	}
	defer f.Close(ctx)

	if err = copyEntryToJobWorkDir(ctx, entryPath, entry, f); err != nil {
		return fmt.Errorf("copy entry file failed: %s", err)
	}
	return nil
}

func copyEntryToJobWorkDir(ctx context.Context, entryPath string, entry *types.Metadata, file dentry.File) error {
	if entryPath == "" {
		entryPath = entry.Name
	}
	f, err := os.OpenFile(entryPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(utils.NewWriterWithContextWriter(ctx, file), f)
	return err
}
