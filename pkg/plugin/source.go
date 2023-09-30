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
	"bytes"
	"context"
	"fmt"
	"github.com/basenana/nanafs/pkg/plugin/pluginapi"
	"github.com/basenana/nanafs/pkg/types"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
	"time"
)

type SourcePlugin interface {
	Plugin
	Fresh(ctx context.Context, opt pluginapi.FreshOption) ([]*pluginapi.Entry, error)
	Open(ctx context.Context, entry *pluginapi.Entry) (io.ReadCloser, error)
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

func (d *ThreeBodyPlugin) Fresh(ctx context.Context, opt pluginapi.FreshOption) ([]*pluginapi.Entry, error) {
	crtAt := time.Now().Unix()
	result := make([]*pluginapi.Entry, 0)
	for i := crtAt - 60; i < crtAt; i += 60 {
		if i <= opt.LastFreshAt.Unix() {
			continue
		}
		result = append(result, &pluginapi.Entry{
			Name:    fmt.Sprintf("3_body_%d.txt", i),
			Kind:    types.RawKind,
			IsGroup: false,
		})
	}
	return result, nil
}

func (d *ThreeBodyPlugin) Open(ctx context.Context, entry *pluginapi.Entry) (io.ReadCloser, error) {
	fileNameParts := strings.Split(entry.Name, "_")
	sendAtStr := fileNameParts[len(fileNameParts)-1]

	sendAt, err := strconv.ParseInt(sendAtStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse send time failed: %s", err)
	}

	buf := &bytes.Buffer{}
	for i := 0; i < 3; i++ {
		buf.WriteString(fmt.Sprintf("%d - Do not answer!\n", sendAt))
	}
	return ioutil.NopCloser(buf), nil
}

func threeBodyBuilder(ctx context.Context, spec types.PluginSpec, scope types.PlugScope) (Plugin, error) {
	return &ThreeBodyPlugin{}, nil
}

func register3BodyPlugin(r *registry) {
	r.Register(the3BodyPluginName, types.PluginSpec{Name: the3BodyPluginName, Version: the3BodyPluginVersion,
		Type: types.TypeSource, Parameters: map[string]string{}}, threeBodyBuilder)
}
