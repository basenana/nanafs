package agentic

import (
	"context"

	"github.com/basenana/friday/core/agents/research"
	fridayapi "github.com/basenana/friday/core/api"
	"github.com/basenana/plugin/api"
	"github.com/basenana/plugin/types"
)

const (
	researchPluginName    = "research"
	researchPluginVersion = "1.0.0"
)

var ResearchPluginSpec = types.PluginSpec{
	Name:    researchPluginName,
	Version: researchPluginVersion,
	Type:    types.TypeProcess,
	RequiredConfig: append(LLMRequiredConfig(),
		"websearch_type", // WebSearch type: pse (Google Programmable Search Engine)
		"pse_engine_id",  // Google PSE Engine ID (required when websearch_type=pse)
		"pse_api_key",    // Google PSE API Key (required when websearch_type=pse)
	),
}

type ResearchPlugin struct {
	workingPath string
	jobID       string
	config      map[string]string
}

func (p *ResearchPlugin) Name() string           { return researchPluginName }
func (p *ResearchPlugin) Type() types.PluginType { return types.TypeProcess }
func (p *ResearchPlugin) Version() string        { return researchPluginVersion }

func (p *ResearchPlugin) Run(ctx context.Context, request *api.Request) (*api.Response, error) {
	message := api.GetStringParameter("message", request, "")
	if message == "" {
		return api.NewFailedResponse("message parameter is required"), nil
	}

	systemPrompt := api.GetStringParameter("system_prompt", request, "")

	llm, err := NewLLMClient(p.config)
	if err != nil {
		return api.NewFailedResponse(err.Error()), nil
	}

	rsTools := FileAccessTools(p.workingPath)

	// Check for websearch_type config and add corresponding tools
	switch p.config["websearch_type"] {
	case "pse":
		engineID := p.config["pse_engine_id"]
		apiKey := p.config["pse_api_key"]
		if engineID != "" && apiKey != "" {
			rsTools = append(rsTools, NewPSEWebSearchTool(engineID, apiKey)...)
		}
	}

	agent := research.New("research", "Research Agent", llm, research.Option{
		SystemPrompt: systemPrompt,
		Tools:        rsTools,
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

func NewResearchPlugin(ps types.PluginCall) types.Plugin {
	return &ResearchPlugin{
		workingPath: ps.WorkingPath,
		jobID:       ps.JobID,
		config:      ps.Config,
	}
}
