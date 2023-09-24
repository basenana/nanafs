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
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/pkg/workflow/jobrun"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"os"
	"runtime"
	"time"
)

var (
	defaultLinuxWorkdir = "/var/lib/nanafs/workflow"
	wfLogger            *zap.SugaredLogger
)

func assembleWorkflowJob(spec *types.WorkflowSpec, entry *types.Metadata) (*types.WorkflowJob, error) {
	var globalParam = map[string]string{}
	j := &types.WorkflowJob{
		Id:        uuid.New().String(),
		Workflow:  spec.Id,
		Target:    types.WorkflowTarget{EntryID: entry.ID},
		Status:    jobrun.InitializingStatus,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	for _, stepSpec := range spec.Steps {
		if stepSpec.Plugin != nil {
			for k, v := range globalParam {
				stepSpec.Plugin.Parameters[k] = v
			}
		}
		j.Steps = append(j.Steps,
			types.WorkflowJobStep{
				StepName: stepSpec.Name,
				Status:   jobrun.InitializingStatus,
				Plugin:   stepSpec.Plugin,
			},
		)
	}

	return j, nil
}

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
		parent *types.Metadata
		err    error

		reversedNames []string
	)
	for {
		parent, err = mgr.GetEntry(ctx, entryID)
		if err != nil {
			return "", err
		}
		entryID = parent.ParentID
		if entryID == 0 {
			return "", types.ErrNotFound
		}
		if entryID == 1 {
			break
		}
		reversedNames = append(reversedNames, parent.Name)
	}

	buf := &bytes.Buffer{}
	buf.WriteString("/")
	for i := len(reversedNames) - 1; i >= 0; i -= 1 {
		buf.WriteString("/")
		buf.WriteString(reversedNames[i])
	}

	return buf.String(), nil
}
