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
	"github.com/basenana/go-flow/fsm"
	flowstorage "github.com/basenana/go-flow/storage"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"io"
	"os"
	"path"
	"time"
)

func initWorkflow(wf *types.WorkflowSpec) *types.WorkflowSpec {
	wf.Id = uuid.New().String()
	wf.CreatedAt = time.Now()
	wf.UpdatedAt = time.Now()
	return wf
}

type storageWrapper struct {
	recorder metastore.ScheduledTaskRecorder
	logger   *zap.SugaredLogger
}

func (s *storageWrapper) GetFlow(flowId flow.FID) (flow.Flow, error) {
	wfJob, err := s.recorder.ListWorkflowJob(context.Background(), types.JobFilter{JobID: string(flowId)})
	if err != nil {
		s.logger.Errorw("load job failed", "err", err)
		return nil, err
	}
	if len(wfJob) == 0 {
		return nil, types.ErrNotFound
	}

	job := &Job{
		WorkflowJob: wfJob[0],
		logger:      s.logger.With(zap.String("job", string(flowId))),
	}
	return job, nil
}

func (s *storageWrapper) GetFlowMeta(flowId flow.FID) (*flowstorage.FlowMeta, error) {
	flowJob, err := s.GetFlow(flowId)
	if err != nil {
		return nil, err
	}

	job, ok := flowJob.(*Job)
	if !ok {
		return nil, ErrJobNotFound
	}

	result := &flowstorage.FlowMeta{
		Type:       job.Type(),
		Id:         job.ID(),
		Status:     job.GetStatus(),
		TaskStatus: map[flow.TName]fsm.Status{},
	}
	for _, step := range job.Steps {
		result.TaskStatus[flow.TName(step.StepName)] = fsm.Status(step.Status)
	}
	return result, nil
}

func (s *storageWrapper) SaveFlow(flow flow.Flow) error {
	job, ok := flow.(*Job)
	if !ok {
		return fmt.Errorf("flow %s not a Job object", flow.ID())
	}

	err := s.recorder.SaveWorkflowJob(context.Background(), job.WorkflowJob)
	if err != nil {
		s.logger.Errorw("save job to metadb failed", "err", err)
		return err
	}

	return nil
}

func (s *storageWrapper) DeleteFlow(flowId flow.FID) error {
	err := s.recorder.DeleteWorkflowJob(context.Background(), string(flowId))
	if err != nil {
		s.logger.Errorw("delete job to metadb failed", "err", err)
		return err
	}
	return nil
}

func (s *storageWrapper) SaveTask(flowId flow.FID, task flow.Task) error {
	flowJob, err := s.GetFlow(flowId)
	if err != nil {
		return err
	}

	job, ok := flowJob.(*Job)
	if !ok {
		return ErrJobNotFound
	}

	for i, step := range job.Steps {
		if step.StepName == string(task.Name()) {
			job.Steps[i].Status = string(task.GetStatus())
			job.Steps[i].Message = task.GetMessage()
			break
		}
	}

	return s.SaveFlow(job)
}

func (s *storageWrapper) DeleteTask(flowId flow.FID, taskName flow.TName) error {
	return nil
}

func copyEntryToJobWorkDir(ctx context.Context, base, jobID string, entry dentry.File) (string, error) {
	filePath := path.Join(jobWorkdir(base, jobID), entry.Metadata().Name)

	f, err := os.OpenFile(filePath, os.O_RDWR, 0755)
	if err != nil {
		return "", err
	}
	defer f.Close()

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

func initJobWorkDir(base, jobID string) error {
	dir := jobWorkdir(base, jobID)
	s, err := os.Stat(dir)
	if err != nil && err != os.ErrNotExist {
		return err
	}
	if err == nil {
		if s.IsDir() {
			return nil
		}
		return types.ErrNoGroup
	}
	return os.MkdirAll(dir, 0755)
}

func cleanUpJobWorkDir(base, jobID string) error {
	dir := jobWorkdir(base, jobID)
	s, err := os.Stat(dir)
	if err != nil {
		if err == os.ErrNotExist {
			return nil
		}
		return err
	}
	if !s.IsDir() {
		return types.ErrNoGroup
	}

	return os.RemoveAll(dir)
}

func jobWorkdir(base, jobID string) string {
	return path.Join(base, "jobs", jobID)
}
