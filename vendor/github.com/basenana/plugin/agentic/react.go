package agentic

import (
	"context"
	"strings"

	"github.com/basenana/friday/core/agents"
	fridayapi "github.com/basenana/friday/core/api"
	"github.com/basenana/plugin/api"
	"github.com/basenana/plugin/logger"
	"github.com/basenana/plugin/types"
	"go.uber.org/zap"
)

const (
	pluginName    = "react"
	pluginVersion = "1.0.0"
)

var PluginSpec = types.PluginSpec{
	Name:           pluginName,
	Version:        pluginVersion,
	Type:           types.TypeProcess,
	RequiredConfig: LLMRequiredConfig(),
	InitParameters: []types.ParameterSpec{
		{
			Name:        "system_prompt",
			Required:    false,
			Description: "System prompt to override default",
		},
	},
	Parameters: []types.ParameterSpec{
		{
			Name:        "message",
			Required:    true,
			Description: "User message for the agent",
		},
	},
}

type ReactPlugin struct {
	logger      *zap.SugaredLogger
	workingPath string
	jobID       string
	config      map[string]string
}

func (p *ReactPlugin) Name() string           { return pluginName }
func (p *ReactPlugin) Type() types.PluginType { return types.TypeProcess }
func (p *ReactPlugin) Version() string        { return pluginVersion }

func (p *ReactPlugin) Run(ctx context.Context, request *api.Request) (*api.Response, error) {
	message := api.GetStringParameter("message", request, "")
	if message == "" {
		p.logger.Warnw("message parameter is required")
		return api.NewFailedResponse("message parameter is required"), nil
	}

	systemPrompt := api.GetStringParameter("system_prompt", request, "")

	p.logger.Infow("react plugin started", "message_len", len(message), "has_system_prompt", systemPrompt != "")

	llm, err := NewLLMClient(p.config)
	if err != nil {
		p.logger.Warnw("create LLM client failed", "error", err)
		return api.NewFailedResponse(err.Error()), nil
	}

	tools := FileAccessTools(p.workingPath, p.logger)
	agent := agents.New(llm, agents.Option{SystemPrompt: systemPrompt, Tools: tools})

	resp := agent.Chat(ctx, &fridayapi.Request{
		Session:     NewSession(p.jobID, llm),
		UserMessage: message,
	})

	content, err := fridayapi.ReadAllContent(ctx, resp)
	if err != nil {
		p.logger.Warnw("collect response failed", "error", err)
		return api.NewFailedResponse(err.Error()), nil
	}

	p.logger.Infow("react plugin completed", "result_len", len(content))
	return api.NewResponseWithResult(map[string]any{"result": strings.TrimSpace(content)}), nil
}

func NewReactPlugin(ps types.PluginCall) types.Plugin {
	return &ReactPlugin{
		logger:      logger.NewPluginLogger(pluginName, ps.JobID),
		workingPath: ps.WorkingPath,
		jobID:       ps.JobID,
		config:      ps.Config,
	}
}
