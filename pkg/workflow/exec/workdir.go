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
	"github.com/basenana/nanafs/utils"
	"io"
	"os"
	"path"
)

func initWorkdir(ctx context.Context, jobWorkdir string, job *types.WorkflowJob) (string, error) {
	if job.Id == "" {
		return "", fmt.Errorf("job id is empty")
	}
	_, err := os.Stat(jobWorkdir)
	if err != nil {
		return "", fmt.Errorf("base job workdir %s: %s", jobWorkdir, err)
	}

	workdir := path.Join(jobWorkdir, fmt.Sprintf("pob-%s", job.Id))
	enInfo, err := os.Stat(workdir)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	if enInfo != nil {
		if enInfo.IsDir() {
			return workdir, nil
		}
		return workdir, fmt.Errorf("job workdir is existed and not a dir")
	}

	return workdir, utils.Mkdir(workdir)
}

func cleanupWorkdir(ctx context.Context, workdir string) error {
	if workdir == "" {
		return nil
	}
	info, err := os.Stat(workdir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if !info.IsDir() {
		return fmt.Errorf("workdir %s not a dir", workdir)
	}
	return utils.Rmdir(workdir)
}

func entryWorkdirInit(ctx context.Context, entryID int64, entryMgr dentry.Manager, workdir string) (string, error) {
	entry, err := entryMgr.GetEntry(ctx, entryID)
	if err != nil {
		return "", fmt.Errorf("load entry failed: %s", err)
	}
	if types.IsGroup(entry.Kind) {
		return "", fmt.Errorf("entry is a group")
	}

	entryPath := path.Join(workdir, entry.Name)
	enInfo, err := os.Stat(entryPath)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	if enInfo != nil {
		return entryPath, nil
	}

	f, err := entryMgr.Open(ctx, entry.ID, dentry.Attr{Read: true})
	if err != nil {
		return "", fmt.Errorf("open entry failed: %s", err)
	}
	defer f.Close(ctx)

	if err = copyEntryToJobWorkDir(ctx, entryPath, entry, f); err != nil {
		return "", fmt.Errorf("copy entry file failed: %s", err)
	}
	return entryPath, nil
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

	n, err := io.Copy(f, utils.NewReaderWithContextReaderAt(ctx, file))
	fmt.Println(n)
	return err
}
