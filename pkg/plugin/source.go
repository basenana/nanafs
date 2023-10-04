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

package plugin

import (
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/plugin/buildin"
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"io/ioutil"
	"path"
	"time"
)

type SourcePlugin interface {
	ProcessPlugin
	SourceInfo() (string, error)
}

func SourceInfo(ctx context.Context, ps types.PlugScope) (info string, err error) {
	defer func() {
		if rErr := utils.Recover(); rErr != nil {
			err = rErr
		}
	}()
	var plugin Plugin
	plugin, err = BuildPlugin(ctx, ps)
	if err != nil {
		return info, err
	}

	srcPlugin, ok := plugin.(SourcePlugin)
	if !ok {
		return info, fmt.Errorf("not process plugin")
	}

	return srcPlugin.SourceInfo()
}

const (
	the3BodyPluginName    = "three_body"
	the3BodyPluginVersion = "1.0"
)

type ThreeBodyPlugin struct{}

var _ SourcePlugin = &ThreeBodyPlugin{}

func (d *ThreeBodyPlugin) Name() string {
	return the3BodyPluginName
}

func (d *ThreeBodyPlugin) Type() types.PluginType {
	return types.TypeSource
}

func (d *ThreeBodyPlugin) Version() string {
	return the3BodyPluginVersion
}

func (d *ThreeBodyPlugin) SourceInfo() (string, error) {
	return "internal.FileGenerator", nil
}

func (d *ThreeBodyPlugin) Run(ctx context.Context, request *pluginapi.Request) (*pluginapi.Response, error) {
	if request.EntryId == 0 {
		return nil, fmt.Errorf("entry id is empty")
	}
	if request.WorkPath == "" {
		return nil, fmt.Errorf("workdir is empty")
	}

	result, err := d.fileGenerate(request.EntryId, request.WorkPath)
	if err != nil {
		resp := pluginapi.NewFailedResponse(fmt.Sprintf("file generate failed: %s", err))
		return resp, nil
	}
	results := []pluginapi.CollectManifest{result}
	return pluginapi.NewResponseWithResult(map[string]any{pluginapi.ResCollectManifests: results}), nil
}

func (d *ThreeBodyPlugin) fileGenerate(baseEntry int64, workdir string) (pluginapi.CollectManifest, error) {
	var (
		crtAt    = time.Now().Unix()
		filePath = path.Join(workdir, fmt.Sprintf("3_body_%d.txt", crtAt))
		fileData = []byte(fmt.Sprintf("%d - Do not answer!\n", crtAt))
	)
	err := ioutil.WriteFile(filePath, fileData, 0655)
	if err != nil {
		return pluginapi.CollectManifest{}, err
	}
	return pluginapi.CollectManifest{
		BaseEntry: baseEntry,
		NewFiles: []pluginapi.Entry{{
			Name: path.Base(filePath),
			Kind: types.RawKind,
			Size: int64(len(fileData)),
		}},
	}, nil
}

func threeBodyBuilder(ctx context.Context, spec types.PluginSpec, scope types.PlugScope) (Plugin, error) {
	return &ThreeBodyPlugin{}, nil
}

func registerBuildInSourcePlugin(r *registry, recorderGetter metastore.PluginRecorderGetter) {
	r.Register(the3BodyPluginName, types.PluginSpec{Name: the3BodyPluginName, Version: the3BodyPluginVersion,
		Type: types.TypeSource, Parameters: map[string]string{}}, threeBodyBuilder)

	r.Register(buildin.RssSourcePluginName,
		types.PluginSpec{Name: buildin.RssSourcePluginName, Version: buildin.RssSourcePluginVersion, Type: types.TypeSource, Parameters: map[string]string{}},
		func(ctx context.Context, spec types.PluginSpec, scope types.PlugScope) (Plugin, error) {
			return buildin.BuildRssSourcePlugin(ctx, recorderGetter.PluginRecorder(scope), spec, scope), nil
		})
}
