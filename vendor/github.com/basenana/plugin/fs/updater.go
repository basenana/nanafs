package fs

import (
	"context"

	"github.com/basenana/plugin/api"
	"github.com/basenana/plugin/logger"
	"github.com/basenana/plugin/types"
	"github.com/basenana/plugin/utils"
	"go.uber.org/zap"
)

const (
	updatePluginName    = "update"
	updatePluginVersion = "1.0"
)

var UpdatePluginSpec = types.PluginSpec{
	Name:    updatePluginName,
	Version: updatePluginVersion,
	Type:    types.TypeProcess,
	Parameters: []types.ParameterSpec{
		{
			Name:        "entry_uri",
			Required:    true,
			Description: "Entry URI to update",
		},
		{
			Name:        "properties",
			Required:    true,
			Description: "Entry properties to update (JSON object)",
		},
	},
}

type Updater struct {
	fileRoot *utils.FileAccess
	logger   *zap.SugaredLogger
}

func NewUpdater(ps types.PluginCall) types.Plugin {
	return &Updater{
		fileRoot: utils.NewFileAccess(ps.WorkingPath),
		logger:   logger.NewPluginLogger(updatePluginName, ps.JobID),
	}
}

func (p *Updater) Name() string           { return updatePluginName }
func (p *Updater) Type() types.PluginType { return types.TypeProcess }
func (p *Updater) Version() string        { return updatePluginVersion }

func (p *Updater) Run(ctx context.Context, request *api.Request) (*api.Response, error) {
	entryURI := api.GetStringParameter("entry_uri", request, "")
	if entryURI == "" {
		return api.NewFailedResponse("entry_uri is required"), nil
	}

	props := buildProperties(request)

	p.logger.Infow("update started", "entry_uri", entryURI)

	if request.FS == nil {
		return api.NewFailedResponse("file system is not available"), nil
	}
	if err := request.FS.UpdateEntry(ctx, entryURI, props); err != nil {
		p.logger.Warnw("update entry failed", "entry_uri", entryURI, "error", err)
		return api.NewFailedResponse("failed to update entry: " + err.Error()), nil
	}

	p.logger.Infow("update completed", "entry_uri", entryURI)
	return api.NewResponse(), nil
}
