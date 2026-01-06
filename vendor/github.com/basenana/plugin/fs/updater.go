package fs

import (
	"context"
	"strconv"

	"github.com/basenana/plugin/api"
	"github.com/basenana/plugin/logger"
	"github.com/basenana/plugin/types"
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
}

type Updater struct {
	logger *zap.SugaredLogger
}

func NewUpdater(ps types.PluginCall) types.Plugin {
	return &Updater{
		logger: logger.NewPluginLogger(updatePluginName, ps.JobID),
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

	id, err := strconv.ParseInt(entryURI, 10, 64)
	if err != nil {
		return api.NewFailedResponse("invalid entry_uri: " + entryURI), nil
	}

	props := buildProperties(request)

	p.logger.Infow("update started", "entry_uri", entryURI)

	if request.FS == nil {
		return api.NewFailedResponse("file system is not available"), nil
	}
	if err := request.FS.UpdateEntry(ctx, id, props); err != nil {
		p.logger.Warnw("update entry failed", "entry_uri", entryURI, "error", err)
		return api.NewFailedResponse("failed to update entry: " + err.Error()), nil
	}

	p.logger.Infow("update completed", "entry_uri", entryURI)
	return api.NewResponse(), nil
}
