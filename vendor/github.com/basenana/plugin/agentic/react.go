package agentic

import (
	"context"

	"github.com/basenana/friday/core/agents/react"
	fridayapi "github.com/basenana/friday/core/api"
	"github.com/basenana/plugin/api"
	"github.com/basenana/plugin/types"
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
}

type ReactPlugin struct {
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
		return api.NewFailedResponse("message parameter is required"), nil
	}

	systemPrompt := api.GetStringParameter("system_prompt", request, "")

	llm, err := NewLLMClient(p.config)
	if err != nil {
		return api.NewFailedResponse(err.Error()), nil
	}

	tools := FileAccessTools(p.workingPath)

	agent := react.New("react", "ReAct Agent with file access", llm, react.Option{
		SystemPrompt: systemPrompt,
		Tools:        tools,
	})

	resp := agent.Chat(ctx, &fridayapi.Request{
		Session:     NewSession(p.jobID),
		UserMessage: message,
	})

	content, _, err := CollectResponse(ctx, resp)
	if err != nil {
		return api.NewFailedResponse(err.Error()), nil
	}

	return api.NewResponseWithResult(map[string]any{
		"result": content,
	}), nil
}

func NewReactPlugin(ps types.PluginCall) types.Plugin {
	return &ReactPlugin{
		workingPath: ps.WorkingPath,
		jobID:       ps.JobID,
		config:      ps.Config,
	}
}
