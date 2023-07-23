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
	"context"
	"fmt"
	"github.com/basenana/go-flow/flow"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"io"
	"os"
	"path"
	"regexp"
	"time"
)

var (
	workflowIDPattern = "^[A-zA-Z][a-zA-Z0-9-_.]{5,31}$"
	wfIDRegexp        = regexp.MustCompile(workflowIDPattern)
)

func isValidID(idStr string) error {
	if wfIDRegexp.MatchString(idStr) {
		return nil
	}
	return fmt.Errorf("invalid ID, pattern: %s", workflowIDPattern)
}

func initWorkflow(wf *types.WorkflowSpec) *types.WorkflowSpec {
	if wf.Id == "" {
		wf.Id = uuid.New().String()
	}
	wf.CreatedAt = time.Now()
	wf.UpdatedAt = time.Now()
	return wf
}

type storageWrapper struct {
	recorder metastore.ScheduledTaskRecorder
	logger   *zap.SugaredLogger
}

var _ flow.Storage = &storageWrapper{}

func (s *storageWrapper) GetFlow(ctx context.Context, flowId string) (*flow.Flow, error) {
	wfJob, err := s.recorder.ListWorkflowJob(ctx, types.JobFilter{JobID: flowId})
	if err != nil {
		s.logger.Errorw("load job failed", "err", err)
		return nil, err
	}
	if len(wfJob) == 0 {
		return nil, types.ErrNotFound
	}
	wf := wfJob[0]

	return assembleFlow(wf)
}

func (s *storageWrapper) SaveFlow(ctx context.Context, flow *flow.Flow) error {
	wfJob, err := s.recorder.ListWorkflowJob(ctx, types.JobFilter{JobID: flow.ID})
	if err != nil {
		return fmt.Errorf("query workflow job failed: %s", err)
	}
	if len(wfJob) == 0 {
		return fmt.Errorf("query workflow job failed: %s not found", flow.ID)
	}
	wf := wfJob[0]

	wf.Status = flow.Status
	wf.Message = flow.Message
	for i, step := range wf.Steps {
		for _, task := range flow.Tasks {
			if step.StepName == task.Name {
				wf.Steps[i].Status = task.Status
				wf.Steps[i].Message = task.Message
				break
			}
		}
	}
	wf.UpdatedAt = time.Now()

	err = s.recorder.SaveWorkflowJob(ctx, wf)
	if err != nil {
		s.logger.Errorw("save job to metadb failed", "err", err)
		return err
	}

	return nil
}

func (s *storageWrapper) DeleteFlow(ctx context.Context, flowId string) error {
	err := s.recorder.DeleteWorkflowJob(ctx, flowId)
	if err != nil {
		s.logger.Errorw("delete job to metadb failed", "err", err)
		return err
	}
	return nil
}

func (s *storageWrapper) SaveTask(ctx context.Context, flowId string, task *flow.Task) error {
	flowJob, err := s.GetFlow(ctx, flowId)
	if err != nil {
		return err
	}

	for i, t := range flowJob.Tasks {
		if t.Name == task.Name {
			flowJob.Tasks[i] = *task
			break
		}
	}

	return s.SaveFlow(ctx, flowJob)
}

func copyEntryToJobWorkDir(ctx context.Context, workDir, entryPath string, entry dentry.File) (string, error) {
	if entryPath == "" {
		entryPath = entry.Metadata().Name
	}
	filePath := path.Join(workDir, entryPath)
	f, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if err = f.Truncate(0); err != nil {
		return "", err
	}

	var (
		buf = make([]byte, 1024)
		off int64
	)
	for {
		n, rErr := entry.ReadAt(ctx, buf, off)
		if n > 0 {
			_, wErr := f.Write(buf[:n])
			if wErr != nil {
				return "", wErr
			}
			off += n
		}
		if rErr != nil {
			if rErr == io.EOF {
				break
			}
			return "", rErr
		}
	}

	return filePath, nil
}
