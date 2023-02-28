package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/basenana/nanafs/pkg/plugin/adaptors"
	"github.com/basenana/nanafs/pkg/plugin/buildin"
	"github.com/basenana/nanafs/pkg/types"
	"os"
	"path/filepath"
)

func readPluginSpec(basePath, pluginSpecFile string) (types.PluginSpec, error) {
	pluginPath := filepath.Join(basePath, pluginSpecFile)
	f, err := os.Open(pluginPath)
	if err != nil {
		return types.PluginSpec{}, err
	}
	defer f.Close()

	spec := types.PluginSpec{}
	if err = json.NewDecoder(f).Decode(&spec); err != nil {
		return types.PluginSpec{}, err
	}

	if spec.Name == "" {
		return types.PluginSpec{}, fmt.Errorf("plugin name was empty")
	}
	switch spec.Type {
	case adaptors.ExecTypeGoPlugin:
	case adaptors.ExecTypeBin:
	case adaptors.ExecTypeScript:
	default:
		return types.PluginSpec{}, fmt.Errorf("plugin type %s no def", spec.Type)
	}

	if spec.Path != "" {
		_, err = os.Stat(spec.Path)
		if err != nil {
			return types.PluginSpec{}, fmt.Errorf("stat plugin failed: %s", err.Error())
		}
	}

	return spec, nil
}

func loadDummyPlugins(r *plugins) {
	// register dummy plugin
	dummyPlugins := []Plugin{
		buildin.InitDummySourcePlugin(),
		buildin.InitDummyProcessPlugin(),
		buildin.InitDummyMirrorPlugin(),
	}

	for i := range dummyPlugins {
		r.Register(context.Background(), dummyPlugins[i].Name(), dummyPlugins[i])
	}
}
