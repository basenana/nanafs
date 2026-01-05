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
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"github.com/basenana/nanafs/pkg/core"

	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
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

func entryWorkdirInit(ctx context.Context, namespace, entryUri string, fsCore core.Core, workdir string) (*pluginapi.Entry, error) {
	_, entry, err := fsCore.GetEntryByPath(ctx, namespace, entryUri)
	if err != nil {
		return nil, fmt.Errorf("load entry failed: %s", err)
	}
	if entry.IsGroup {
		return nil, fmt.Errorf("entry is a group")
	}
	entryName := path.Base(entryUri)

	result := &pluginapi.Entry{
		ID:         entry.ID,
		Name:       entryName,
		Kind:       entry.Kind,
		Size:       entry.Size,
		IsGroup:    entry.IsGroup,
		Properties: nil,
		Document:   nil,
	}
	result.Path = path.Join(workdir, entryName)

	enInfo, err := os.Stat(result.Path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	if enInfo != nil {
		return result, nil
	}

	f, err := fsCore.Open(ctx, namespace, entry.ID, types.OpenAttr{Read: true})
	if err != nil {
		return nil, fmt.Errorf("open entry failed: %s", err)
	}
	defer f.Close(ctx)

	if err = copyEntryToJobWorkDir(ctx, result.Path, entry, f); err != nil {
		return nil, fmt.Errorf("copy entry file failed: %s", err)
	}
	return result, nil
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

func collectAndModifyEntry(ctx context.Context, namespace string, fsCore core.Core, workdir string, entry *pluginapi.Entry) error {
	isNeedCreate := entry.ID == 0

	if !isNeedCreate && !entry.Overwrite {
		return fmt.Errorf("file %s already exists", entry.Name)
	}

	var (
		result *types.Entry
		err    error
	)
	_, err = fsCore.FindEntry(ctx, namespace, entry.Parent, entry.Name)
	if err != nil && !errors.Is(err, types.ErrNotFound) {
		return fmt.Errorf("check new file existed error: %s", err)
	}

	if isNeedCreate && err == nil {
		// file already existed
		return nil
	}

	tmpFile, err := os.Open(path.Join(workdir, entry.Name))
	if err != nil {
		return fmt.Errorf("read temporary file failed: %s", err)
	}
	defer tmpFile.Close()

	if isNeedCreate {
		result, err = fsCore.CreateEntry(ctx, namespace, entry.Parent, types.EntryAttr{Name: entry.Name, Kind: entry.Kind, Properties: nil})
		if err != nil {
			return fmt.Errorf("create new entry failed: %s", err)
		}
		entry.ID = result.ID
	}

	f, err := fsCore.Open(ctx, namespace, entry.ID, types.OpenAttr{Write: true, Trunc: true})
	if err != nil {
		return fmt.Errorf("open entry file failed: %s", err)
	}
	defer f.Close(ctx)

	_, err = io.Copy(utils.NewWriterWithContextWriter(ctx, f), tmpFile)
	if err != nil {
		return fmt.Errorf("copy temporary file to entry file failed: %s", err)
	}

	return nil
}
