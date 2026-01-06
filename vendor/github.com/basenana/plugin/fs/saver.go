package fs

import (
	"context"
	"os"

	"github.com/basenana/plugin/api"
	"github.com/basenana/plugin/logger"
	"github.com/basenana/plugin/types"
	"github.com/basenana/plugin/utils"
	"go.uber.org/zap"
)

const (
	savePluginName    = "save"
	savePluginVersion = "1.0"
)

var SavePluginSpec = types.PluginSpec{
	Name:    savePluginName,
	Version: savePluginVersion,
	Type:    types.TypeProcess,
}

type Saver struct {
	logger *zap.SugaredLogger
}

func NewSaver(ps types.PluginCall) types.Plugin {
	return &Saver{
		logger: logger.NewPluginLogger(savePluginName, ps.JobID),
	}
}

func (p *Saver) Name() string           { return savePluginName }
func (p *Saver) Type() types.PluginType { return types.TypeProcess }
func (p *Saver) Version() string        { return savePluginVersion }

func (p *Saver) Run(ctx context.Context, request *api.Request) (*api.Response, error) {
	filePath := api.GetStringParameter("file_path", request, "")
	if filePath == "" {
		return api.NewFailedResponse("file_path is required"), nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return api.NewFailedResponse("failed to open file: " + err.Error()), nil
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return api.NewFailedResponse("failed to get file info: " + err.Error()), nil
	}

	name := api.GetStringParameter("name", request, fileInfo.Name())
	parentURI := api.GetStringParameter("parent_uri", request, "")
	properties := buildProperties(request)

	p.logger.Infow("save started", "file_path", filePath, "name", name, "parent_uri", parentURI)

	if request.FS == nil {
		return api.NewFailedResponse("file system is not available"), nil
	}
	if err := request.FS.SaveEntry(ctx, parentURI, name, properties, file); err != nil {
		p.logger.Warnw("save entry failed", "file_path", filePath, "error", err)
		return api.NewFailedResponse("failed to save entry: " + err.Error()), nil
	}

	p.logger.Infow("save completed", "file_path", filePath)
	return api.NewResponse(), nil
}

func buildProperties(request *api.Request) types.Properties {
	// Safely extract properties map (takes priority)
	propertiesMapRaw, propertiesOK := request.Parameter["properties"]
	if propertiesOK {
		if propertiesMap, ok := propertiesMapRaw.(map[string]interface{}); ok {
			properties := types.Properties{}
			utils.UnmarshalMap(propertiesMap, &properties)
			return properties
		}
	}

	// Safely extract document map
	documentMapRaw, documentOK := request.Parameter["document"]
	if documentOK {
		if documentMap, ok := documentMapRaw.(map[string]interface{}); ok {
			document := &types.Document{}
			utils.UnmarshalMap(documentMap, document)
			return document.Properties
		}
	}

	return types.Properties{}
}
