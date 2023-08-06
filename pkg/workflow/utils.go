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

package workflow

import (
	"bytes"
	"context"
	"github.com/basenana/go-flow/cfg"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"io"
	"os"
	"runtime"
	"time"
)

var (
	defaultLinuxWorkdir = "/var/lib/nanafs/workflow"
	wfLogger            *zap.SugaredLogger
)

func initWorkflowJobRootWorkdir(wfCfg *config.Workflow) error {
	if wfCfg.JobWorkdir == "" {
		switch runtime.GOOS {
		case "linux":
			wfCfg.JobWorkdir = defaultLinuxWorkdir
		default:
			wfCfg.JobWorkdir = os.TempDir()
		}
	}
	wfLogger.Infof("job root workdir: %s", wfCfg.JobWorkdir)
	cfg.LocalWorkdirBase = wfCfg.JobWorkdir
	return os.MkdirAll(wfCfg.JobWorkdir, 0755)
}

func initWorkflow(wf *types.WorkflowSpec) *types.WorkflowSpec {
	if wf.Id == "" {
		wf.Id = uuid.New().String()
	}
	wf.CreatedAt = time.Now()
	wf.UpdatedAt = time.Now()
	return wf
}

func entryID2FusePath(ctx context.Context, entryID int64, mgr dentry.Manager, fuseCfg config.FUSE) (string, error) {
	if !fuseCfg.Enable || fuseCfg.RootPath == "" {
		return "", nil
	}

	var (
		parent dentry.Entry
		err    error

		reversedNames []string
	)
	for {
		parent, err = mgr.GetEntry(ctx, entryID)
		if err != nil {
			return "", err
		}
		entryID = parent.Metadata().ParentID
		if entryID == 0 {
			return "", types.ErrNotFound
		}
		if entryID == 1 {
			break
		}
		reversedNames = append(reversedNames, parent.Metadata().Name)
	}

	buf := &bytes.Buffer{}
	buf.WriteString("/")
	for i := len(reversedNames) - 1; i >= 0; i -= 1 {
		buf.WriteString("/")
		buf.WriteString(reversedNames[i])
	}

	return buf.String(), nil
}

func copyEntryToJobWorkDir(ctx context.Context, entryPath string, entry dentry.File) error {
	if entryPath == "" {
		entryPath = entry.Metadata().Name
	}
	f, err := os.OpenFile(entryPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(utils.NewWriterWithContextWriter(ctx, entry), f)
	return err
}
