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

package jobrun

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"github.com/basenana/nanafs/pkg/core"

	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
)

func initWorkdir(ctx context.Context, workdir string, job *types.WorkflowJob) error {
	enInfo, err := os.Stat(workdir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if enInfo != nil {
		if enInfo.IsDir() {
			return nil
		}
		return fmt.Errorf("job workdir is existed and not a dir")
	}

	if err = utils.Mkdir(workdir); err != nil {
		return err
	}

	// workdir info
	infoFile := path.Join(workdir, ".workflowinfo.json")
	info := map[string]string{
		"workflow": job.Workflow,
		"job":      job.Id,
		"init_at":  time.Now().Format(time.RFC3339),
	}
	infoRaw, _ := json.Marshal(info)
	_ = os.WriteFile(infoFile, infoRaw, 0655)

	return nil
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

func entryWorkdirInit(ctx context.Context, namespace, entryUri string, fsCore core.Core, workdir string) (string, error) {
	_, entry, err := fsCore.GetEntryByPath(ctx, namespace, entryUri)
	if err != nil {
		return "", fmt.Errorf("load entry failed: %s", err)
	}
	if entry.IsGroup {
		return "", fmt.Errorf("entry is a group")
	}

	entryName := path.Base(entryUri)
	entryPath := path.Join(workdir, entryName)

	enInfo, err := os.Stat(entryPath)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	if enInfo != nil {
		return "", nil
	}

	f, err := fsCore.Open(ctx, namespace, entry.ID, types.OpenAttr{Read: true})
	if err != nil {
		return "", fmt.Errorf("open entry failed: %s", err)
	}
	defer f.Close(ctx)

	if err = copyEntryToJobWorkDir(ctx, entryPath, entry, f); err != nil {
		return "", fmt.Errorf("copy entry file failed: %s", err)
	}
	return entryPath, nil
}

func copyEntryToJobWorkDir(ctx context.Context, entryPath string, entry *types.Entry, file core.RawFile) error {
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
