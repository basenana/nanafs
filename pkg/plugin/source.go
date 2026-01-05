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
	"os"
	"path"
	"time"

	"github.com/basenana/nanafs/pkg/plugin/buildin/web"
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
)

type SourcePlugin interface {
	ProcessPlugin
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
	if request.WorkingPath == "" {
		return nil, fmt.Errorf("workdir is empty")
	}

	result, err := d.fileGenerate(request.WorkingPath)
	if err != nil {
		resp := pluginapi.NewFailedResponse(fmt.Sprintf("file generate failed: %s", err))
		return resp, nil
	}
	resp := pluginapi.NewResponseWithResult(result)
	return resp, nil
}

func (d *ThreeBodyPlugin) fileGenerate(workdir string) (map[string]any, error) {
	var (
		crtAt    = time.Now().Unix()
		filePath = path.Join(workdir, fmt.Sprintf("3_body_%d.txt", crtAt))
		fileData = []byte(fmt.Sprintf("%d - Do not answer!\n", crtAt))
	)
	err := os.WriteFile(filePath, fileData, 0655)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"file_path": path.Base(filePath),
		"size":      int64(len(fileData)),
	}, nil
}

func registerBuildInSourcePlugin(r *registry) {
	r.Register(the3BodyPluginName, types.PluginSpec{Name: the3BodyPluginName, Version: the3BodyPluginVersion,
		Type: types.TypeSource}, &ThreeBodyPlugin{})

	r.Register(web.RssSourcePluginName, web.RssSourcePluginSpec, web.BuildRssSourcePlugin())
}
