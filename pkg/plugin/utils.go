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
	"encoding/json"
	"fmt"
	"github.com/basenana/nanafs/pkg/plugin/adaptors"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/prometheus/client_golang/prometheus"
	"os"
	"path/filepath"
)

var (
	processCallTimeUsage = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "plugin_process_call_time_usage_seconds",
			Help:    "The time usage of process plugin call.",
			Buckets: prometheus.ExponentialBuckets(0.1, 5, 5),
		},
		[]string{"plugin_name"},
	)
)

func init() {
	prometheus.MustRegister(
		processCallTimeUsage,
	)
}

func readPluginSpec(basePath, pluginSpecFile string) (types.PluginSpec, Builder, error) {
	pluginPath := filepath.Join(basePath, pluginSpecFile)
	info, err := os.Stat(pluginPath)
	if err != nil {
		return types.PluginSpec{}, nil, err
	}

	if info.IsDir() {
		return types.PluginSpec{}, nil, fmt.Errorf("%s was dir", pluginPath)
	}

	f, err := os.Open(pluginPath)
	if err != nil {
		return types.PluginSpec{}, nil, err
	}
	defer f.Close()

	spec := types.PluginSpec{}
	if err = json.NewDecoder(f).Decode(&spec); err != nil {
		return types.PluginSpec{}, nil, err
	}

	if spec.Name == "" {
		return types.PluginSpec{}, nil, fmt.Errorf("plugin name was empty")
	}

	var builder Builder
	switch spec.Adaptor {
	case adaptors.AdaptorTypeScriptPlugin:
		builder = scriptAdaptorBuilder()
	case adaptors.AdaptorTypeGoPlugin:
		builder = gopluginAdaptorBuilder()
	default:
		return types.PluginSpec{}, nil, fmt.Errorf("unknow adaptor %s", spec.Adaptor)
	}

	return spec, builder, nil
}

func scriptAdaptorBuilder() Builder {
	return func(ctx context.Context, spec types.PluginSpec, scope types.PlugScope) (Plugin, error) {
		return adaptors.NewScriptPluginAdaptor(spec, scope)
	}
}

func gopluginAdaptorBuilder() Builder {
	return func(ctx context.Context, spec types.PluginSpec, scope types.PlugScope) (Plugin, error) {
		return adaptors.NewGoPluginAdaptor(spec, scope)
	}
}
