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
	"errors"
	"fmt"
	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/pkg/workflow/jobrun"
	"github.com/basenana/nanafs/utils"
	"github.com/goccy/go-yaml"
	"io"
	"path"
	"strings"
	"time"
)

const (
	MirrorPluginName    = "workflow"
	MirrorPluginVersion = "1.0"
	MirrorDirRoot       = "root"
	MirrorDirWorkflows  = "workflows"
	MirrorDirJobs       = "jobs"
	MirrorFileType      = ".yaml"
	MirrorRootDirName   = ".workflow"
)

var mirrorPlugin = types.PluginSpec{
	Name:       MirrorPluginName,
	Version:    MirrorPluginVersion,
	Type:       types.TypeMirror,
	Parameters: map[string]string{},
}

/*
MirrorPlugin is an implementation of plugin.MirrorPlugin,
which supports managing workflows using POSIX operations.

virtual directory structure as follows:

	.
	|--workflows
	  |--<workflow_id>.yaml
	|--jobs
	  |--<workflow_id>
	    |--<job_id>.yaml
*/
type MirrorPlugin struct {
	fs  *plugin.MemFS
	mgr Manager
}

var _ plugin.MirrorPlugin = &MirrorPlugin{}

func (m *MirrorPlugin) Name() string { return MirrorPluginName }

func (m *MirrorPlugin) Type() types.PluginType { return types.TypeMirror }

func (m *MirrorPlugin) Version() string { return MirrorPluginVersion }

func (m *MirrorPlugin) GetEntry(ctx context.Context, entryPath string) (*pluginapi.Entry, error) {
	// memfs cached entry
	en, err := m.fs.GetEntry(ctx, entryPath)
	if err != nil && !errors.Is(err, types.ErrNotFound) {
		return nil, err
	}
	if en != nil {
		return en, nil
	}

	dirKind, wfID, jobID, err := parseFilePath(entryPath)
	if err != nil {
		return nil, err
	}

	switch dirKind {
	case MirrorDirRoot:
		return m.fs.GetEntry(ctx, entryPath)
	case MirrorDirWorkflows:
		wf, err := m.mgr.GetWorkflow(ctx, wfID)
		if err != nil {
			return nil, err
		}
		return m.reloadWorkflowEntry(ctx, wf)
	case MirrorDirJobs:
		job, err := m.mgr.GetJob(ctx, wfID, jobID)
		if err != nil {
			return nil, err
		}
		return m.reloadWorkflowJobEntry(ctx, job)
	}

	return nil, types.ErrNotFound
}

func (m *MirrorPlugin) CreateEntry(ctx context.Context, parentPath string, attr pluginapi.EntryAttr) (*pluginapi.Entry, error) {
	entryPath := path.Join(parentPath, attr.Name)
	dirKind, wfID, jobID, err := parseFilePath(entryPath)
	if err != nil {
		return nil, err
	}

	if dirKind == MirrorDirRoot {
		return nil, types.ErrNoAccess
	}
	if dirKind == MirrorDirJobs && wfID == "" {
		return nil, types.ErrNoAccess
	}

	if types.IsGroup(attr.Kind) {
		return nil, types.ErrNoAccess
	}

	en, err := m.fs.CreateEntry(ctx, parentPath, attr)
	if err != nil {
		return nil, err
	}
	if en.Parameters == nil {
		en.Parameters = map[string]string{}
	}
	en.Parameters[types.PlugScopeWorkflowID] = wfID
	en.Parameters[types.PlugScopeWorkflowJobID] = jobID
	return en, nil
}

func (m *MirrorPlugin) UpdateEntry(ctx context.Context, entryPath string, en *pluginapi.Entry) error {
	return m.fs.UpdateEntry(ctx, entryPath, en)
}

func (m *MirrorPlugin) RemoveEntry(ctx context.Context, entryPath string) error {
	dirKind, wfID, jobID, err := parseFilePath(entryPath)
	if err != nil {
		return err
	}

	if dirKind == MirrorDirRoot {
		return types.ErrNoAccess
	}

	en, err := m.fs.GetEntry(ctx, entryPath)
	if err != nil {
		return err
	}
	if en.IsGroup {
		return types.ErrNotEmpty
	}

	switch dirKind {
	case MirrorDirWorkflows:
		wf, err := m.mgr.GetWorkflow(ctx, wfID)
		if err != nil {
			if errors.Is(err, types.ErrNotFound) {
				return m.fs.RemoveEntry(ctx, entryPath)
			}
			return err
		}
		if err = m.mgr.DeleteWorkflow(ctx, wf.Id); err != nil {
			return err
		}
	case MirrorDirJobs:
		_, err = m.mgr.GetJob(ctx, wfID, jobID)
		if err != nil {
			if errors.Is(err, types.ErrNotFound) {
				return m.fs.RemoveEntry(ctx, entryPath)
			}
			return err
		}
		return types.ErrNoAccess
	}

	return m.fs.RemoveEntry(ctx, entryPath)
}

func (m *MirrorPlugin) ListChildren(ctx context.Context, parentPath string) ([]*pluginapi.Entry, error) {
	dirKind, wfID, _, err := parseFilePath(parentPath)
	if err != nil {
		return nil, err
	}

	cachedChild, err := m.fs.ListChildren(ctx, parentPath)
	if err != nil {
		return nil, err
	}

	var (
		children         = make([]*pluginapi.Entry, 0)
		cachedChildMap   = make(map[string]struct{})
		expectedChildMap = make(map[string]struct{})
	)

	for i, ch := range cachedChild {
		cachedChildMap[ch.Name] = struct{}{}
		children = append(children, cachedChild[i])
	}

	switch {
	case dirKind == MirrorDirRoot:

		if _, ok := cachedChildMap[MirrorDirJobs]; !ok {
			child, err := m.fs.CreateEntry(ctx, parentPath, pluginapi.EntryAttr{Name: MirrorDirJobs, Kind: types.ExternalGroupKind})
			if err != nil {
				wfLogger.Errorf("init mirror dir %s error: %s", MirrorDirJobs, err)
				return nil, err
			}
			children = append(children, child)
		}

		if _, ok := cachedChildMap[MirrorDirWorkflows]; !ok {
			child, err := m.fs.CreateEntry(ctx, parentPath, pluginapi.EntryAttr{Name: MirrorDirWorkflows, Kind: types.ExternalGroupKind})
			if err != nil {
				wfLogger.Errorf("init mirror dir %s error: %s", MirrorDirWorkflows, err)
				return nil, err
			}
			children = append(children, child)
		}

		expectedChildMap[MirrorDirJobs] = struct{}{}
		expectedChildMap[MirrorDirWorkflows] = struct{}{}
	case dirKind == MirrorDirWorkflows:
		wfList, err := m.mgr.ListWorkflows(ctx)
		if err != nil {
			return children, err
		}
		for _, wf := range wfList {
			wfFile := id2MirrorFile(wf.Id)
			expectedChildMap[wfFile] = struct{}{}
			if _, ok := cachedChildMap[wfFile]; !ok {
				child, err := m.reloadWorkflowEntry(ctx, wf)
				if err != nil {
					wfLogger.Errorf("init mirror workflow file %s error: %s", wfFile, err)
					return nil, err
				}
				children = append(children, child)
			}
		}
	case dirKind == MirrorDirJobs && wfID == "":
		wfList, err := m.mgr.ListWorkflows(ctx)
		if err != nil {
			return children, err
		}
		for _, wf := range wfList {
			expectedChildMap[wf.Id] = struct{}{}
			if _, ok := cachedChildMap[wf.Id]; !ok {
				child, err := m.fs.CreateEntry(ctx, parentPath, pluginapi.EntryAttr{Name: wf.Id, Kind: types.ExternalGroupKind})
				if err != nil {
					wfLogger.Errorf("init mirror jobs workflow group %s error: %s", wf.Id, err)
					return nil, err
				}
				children = append(children, child)
			}
		}
	case dirKind == MirrorDirJobs && wfID != "":
		jobList, err := m.mgr.ListJobs(ctx, wfID)
		if err != nil {
			return children, err
		}
		for _, j := range jobList {
			jobFile := id2MirrorFile(j.Id)
			expectedChildMap[jobFile] = struct{}{}
			if _, ok := cachedChildMap[jobFile]; !ok {
				child, err := m.reloadWorkflowJobEntry(ctx, j)
				if err != nil {
					wfLogger.Errorf("init mirror job file %s error: %s", jobFile, err)
					return nil, err
				}
				children = append(children, child)
			}
		}
	}

	for c := range cachedChildMap {
		if _, ok := expectedChildMap[c]; ok {
			continue
		}
		_ = m.fs.RemoveEntry(ctx, path.Join(parentPath, c))
	}

	return children, nil
}

func (m *MirrorPlugin) Open(ctx context.Context, entryPath string) (pluginapi.File, error) {
	dirKind, wfID, jobID, err := parseFilePath(entryPath)
	if dirKind == MirrorDirRoot {
		return nil, types.ErrNoAccess
	}
	if err != nil {
		return nil, err
	}

	f, err := m.fs.Open(ctx, entryPath)
	if err != nil {
		return nil, err
	}
	return &yamlFile{File: f, wfID: wfID, jobID: jobID, p: entryPath, mgr: m.mgr}, nil
}

func (m *MirrorPlugin) reloadWorkflowEntry(ctx context.Context, wf *types.WorkflowSpec) (*pluginapi.Entry, error) {
	parentPath := path.Join("/", MirrorDirWorkflows)
	filename := id2MirrorFile(wf.Id)

	en, err := m.fs.GetEntry(ctx, path.Join(parentPath, filename))
	if err != nil && !errors.Is(err, types.ErrNotFound) {
		return nil, err
	}
	if errors.Is(err, types.ErrNotFound) {
		en, err = m.fs.CreateEntry(ctx, parentPath, pluginapi.EntryAttr{Name: filename, Kind: types.RawKind})
		if err != nil {
			return nil, err
		}

		f, err := m.fs.Open(ctx, path.Join(parentPath, filename))
		if err != nil {
			return nil, err
		}
		_ = f.Trunc(ctx)

		err = yaml.NewEncoder(utils.NewWriterWithContextWriter(ctx, f)).Encode(wf)
		if err != nil {
			return nil, err
		}
	}
	return en, nil
}

func (m *MirrorPlugin) reloadWorkflowJobEntry(ctx context.Context, job *types.WorkflowJob) (*pluginapi.Entry, error) {
	parentPath := path.Join("/", MirrorDirJobs, job.Workflow)
	filename := id2MirrorFile(job.Id)

	en, err := m.fs.GetEntry(ctx, path.Join(parentPath, filename))
	if err != nil && !errors.Is(err, types.ErrNotFound) {
		return nil, err
	}
	if errors.Is(err, types.ErrNotFound) {
		en, err = m.fs.CreateEntry(ctx, parentPath, pluginapi.EntryAttr{Name: filename, Kind: types.RawKind})
		if err != nil {
			return nil, err
		}

		f, err := m.fs.Open(ctx, path.Join(parentPath, filename))
		if err != nil {
			return nil, err
		}
		_ = f.Trunc(ctx)

		err = yaml.NewEncoder(utils.NewWriterWithContextWriter(ctx, f)).Encode(job)
		if err != nil {
			return nil, err
		}
	}
	return en, nil
}

type yamlFile struct {
	pluginapi.File
	wfID  string
	jobID string
	p     string
	mgr   Manager
}

func (f *yamlFile) Close(ctx context.Context) error {
	var (
		op     = "unknown"
		rawObj interface{}
		err    error
	)
	switch {
	case f.wfID != "" && f.jobID == "":
		op = "create or update workflow"
		rawObj, err = f.createOrUpdateWorkflow(ctx)
	case f.wfID != "" && f.jobID != "":
		op = "update workflow job"
		rawObj, err = f.triggerOrUpdateWorkflowJob(ctx)
	}

	_ = f.Trunc(ctx)
	writer := utils.NewWriterWithContextWriter(ctx, f)
	if err != nil {
		wfLogger.Errorf("%s failed: %s", op, err)
		_, _ = f.WriteAt(ctx, []byte(fmt.Sprintf("# error: %s\n", err)), 0)
	}
	return yaml.NewEncoder(writer).Encode(rawObj)
}

func (f *yamlFile) createOrUpdateWorkflow(ctx context.Context) (interface{}, error) {
	wf := defaultWorkflow(f.wfID)
	if err := isValidID(f.wfID); err != nil {
		return wf, err
	}

	decodeErr := yaml.NewDecoder(utils.NewReaderWithContextReaderAt(ctx, f.File)).Decode(wf)
	if decodeErr != nil {
		wfLogger.Warnw("decode workflow yaml file failed",
			"workflow", f.wfID, "job", f.jobID, "err", decodeErr)
		return wf, decodeErr
	}

	wf.Id = f.wfID
	oldWf, err := f.mgr.GetWorkflow(ctx, f.wfID)
	if err != nil && !errors.Is(err, types.ErrNotFound) {
		return wf, err
	}

	// do create
	if errors.Is(err, types.ErrNotFound) {
		wf, err = f.mgr.CreateWorkflow(ctx, initWorkflow(wf))
		if err != nil {
			return wf, err
		}
		return wf, nil
	}

	// do update
	if wf.Name != "" {
		oldWf.Name = wf.Name
	}
	oldWf.Rule = wf.Rule
	oldWf.Steps = wf.Steps
	oldWf.UpdatedAt = time.Now()

	oldWf, err = f.mgr.UpdateWorkflow(ctx, oldWf)
	if err != nil {
		return wf, err
	}
	return oldWf, nil
}

func (f *yamlFile) triggerOrUpdateWorkflowJob(ctx context.Context) (interface{}, error) {
	wfJob := defaultWorkflowJob(f.wfID, f.jobID)
	yamlContent, err := io.ReadAll(utils.NewReaderWithContextReaderAt(ctx, f.File))
	if err != nil {
		return wfJob, err
	}
	if len(yamlContent) > 0 {
		decodeErr := yaml.Unmarshal(yamlContent, wfJob)
		if decodeErr != nil {
			wfLogger.Warnw("decode job file failed",
				"workflow", f.wfID, "job", f.jobID, "err", decodeErr)
			return wfJob, decodeErr
		}
	}

	if err = isValidID(f.jobID); err != nil {
		return wfJob, err
	}

	oldJob, err := f.mgr.GetJob(ctx, f.wfID, f.jobID)
	if err != nil && !errors.Is(err, types.ErrNotFound) {
		return wfJob, err
	}

	// do update
	if oldJob != nil {
		if wfJob.Status != oldJob.Status && oldJob.FinishAt.IsZero() {
			switch {
			case wfJob.Status == jobrun.PausedStatus && oldJob.Status == jobrun.RunningStatus:
				err = f.mgr.PauseWorkflowJob(ctx, f.jobID)
			case wfJob.Status == jobrun.RunningStatus && oldJob.Status == jobrun.PausedStatus:
				err = f.mgr.ResumeWorkflowJob(ctx, f.jobID)
			case wfJob.Status == jobrun.CanceledStatus:
				err = f.mgr.CancelWorkflowJob(ctx, f.jobID)
			default:
				err = fmt.Errorf("the current state is %s and cannot be changed to %s", oldJob.Status, wfJob.Status)
			}
		}
		return oldJob, err
	}

	// do create
	target := wfJob.Target
	wfJob, err = f.mgr.TriggerWorkflow(ctx, f.wfID, target, JobAttr{JobID: f.jobID})
	if err != nil {
		return wfJob, err
	}
	return wfJob, nil
}

func buildWorkflowMirrorPlugin(mgr Manager) plugin.Builder {
	mp := &MirrorPlugin{fs: plugin.NewMemFS(), mgr: mgr}

	_, _ = mp.fs.CreateEntry(context.Background(), "/", pluginapi.EntryAttr{
		Name: MirrorDirJobs,
		Kind: types.ExternalGroupKind,
	})

	_, _ = mp.fs.CreateEntry(context.Background(), "/", pluginapi.EntryAttr{
		Name: MirrorDirWorkflows,
		Kind: types.ExternalGroupKind,
	})
	return func(ctx context.Context, spec types.PluginSpec, scope types.PlugScope) (plugin.Plugin, error) {
		return mp, nil
	}
}

func initWorkflowMirrorDir(root *types.Metadata, entryMgr dentry.Manager) error {
	rootGrp, err := entryMgr.OpenGroup(context.TODO(), root.ID)
	if err != nil {
		return fmt.Errorf("open root group failed: %s", err)
	}
	oldDir, err := rootGrp.FindEntry(context.Background(), ".workflow")
	if err != nil && err != types.ErrNotFound {
		return err
	}
	if oldDir != nil {
		return nil
	}

	_, err = entryMgr.CreateEntry(context.Background(), root.ID, types.EntryAttr{
		Name: MirrorRootDirName,
		Kind: types.ExternalGroupKind,
		PlugScope: &types.PlugScope{
			PluginName: MirrorPluginName,
			Version:    MirrorPluginVersion,
			PluginType: types.TypeMirror,
			Parameters: map[string]string{},
		},
	})
	return err
}

func mirrorFile2ID(fileName string) string {
	if strings.HasSuffix(fileName, MirrorFileType) {
		return strings.TrimSuffix(fileName, MirrorFileType)
	}
	return fileName
}

func id2MirrorFile(idStr string) string {
	if strings.HasSuffix(idStr, MirrorFileType) {
		return idStr
	}
	return idStr + MirrorFileType
}

func parseFilePath(enPath string) (dirKind, wfID, jobID string, err error) {
	if enPath == "/" {
		dirKind = MirrorDirRoot
		return
	}
	enPath = strings.TrimPrefix(enPath, utils.PathSeparator)
	pathParts := strings.SplitN(enPath, utils.PathSeparator, 3)

	dirKind = pathParts[0]
	switch dirKind {
	case MirrorDirJobs:
		// path: /jobs/<workflow_id>/<job_id>.yaml
		if len(pathParts) == 3 {
			jobID = mirrorFile2ID(pathParts[2])
		}
		fallthrough
	case MirrorDirWorkflows:
		// path: /workflows/<workflow_id>.yaml
		if len(pathParts) > 1 {
			wfID = mirrorFile2ID(pathParts[1])
		}
	default:
		err = fmt.Errorf("unknown dir %s", dirKind)
		return
	}

	return
}

func defaultWorkflow(wfID string) *types.WorkflowSpec {
	nowAt := time.Now()
	return &types.WorkflowSpec{
		Id:   wfID,
		Name: wfID,
		Rule: types.Rule{},
		Steps: []types.WorkflowStepSpec{
			{
				Name: "step-1",
				Plugin: &types.PlugScope{
					PluginName: "plugin-1",
					Version:    "1.0",
					PluginType: types.TypeProcess,
				},
			},
		},
		Enable:          false,
		Executor:        "local",
		QueueName:       "default",
		CreatedAt:       nowAt,
		UpdatedAt:       nowAt,
		LastTriggeredAt: nowAt,
	}
}

func defaultWorkflowJob(wfID, jobID string) *types.WorkflowJob {
	nowAt := time.Now()
	return &types.WorkflowJob{
		Id:             jobID,
		Workflow:       wfID,
		TriggerReason:  "",
		Target:         types.WorkflowTarget{},
		Executor:       "local",
		QueueName:      "default",
		TimeoutSeconds: 3600,
		CreatedAt:      nowAt,
		UpdatedAt:      nowAt,
	}
}
