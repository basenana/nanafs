package agentic

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/basenana/friday/core/agents/summarize"
	fridayapi "github.com/basenana/friday/core/api"
	"github.com/basenana/friday/core/memory"
	"github.com/basenana/plugin/api"
	"github.com/basenana/plugin/logger"
	"github.com/basenana/plugin/types"
	"github.com/basenana/plugin/utils"
	"go.uber.org/zap"
)

const (
	summaryPluginName    = "summary"
	summaryPluginVersion = "1.0.0"
)

var SummaryPluginSpec = types.PluginSpec{
	Name:           summaryPluginName,
	Version:        summaryPluginVersion,
	Type:           types.TypeProcess,
	RequiredConfig: LLMRequiredConfig(),
}

type SummaryPlugin struct {
	logger     *zap.SugaredLogger
	fileAccess *utils.FileAccess
	jobID      string
	config     map[string]string
}

func (p *SummaryPlugin) Name() string           { return summaryPluginName }
func (p *SummaryPlugin) Type() types.PluginType { return types.TypeProcess }
func (p *SummaryPlugin) Version() string        { return summaryPluginVersion }

func (p *SummaryPlugin) Run(ctx context.Context, request *api.Request) (*api.Response, error) {
	filePath := api.GetStringParameter("file_path", request, "")
	if filePath == "" {
		p.logger.Warnw("file_path parameter is required")
		return api.NewFailedResponse("file_path parameter is required"), nil
	}

	absPath, err := p.fileAccess.GetAbsPath(filePath)
	if err != nil {
		p.logger.Warnw("invalid file path", "path", filePath, "error", err)
		return api.NewFailedResponse(fmt.Sprintf("invalid file_path: %s", err)), nil
	}

	parser := newParser(absPath)
	if parser == nil {
		p.logger.Warnw("unsupported file format", "path", filePath, "ext", filepath.Ext(filePath))
		return api.NewFailedResponse(fmt.Sprintf("unsupported file format: %s", filepath.Ext(filePath))), nil
	}

	doc, err := parser.Load(logger.IntoContext(ctx, p.logger))
	if err != nil {
		p.logger.Warnw("load file content failed", "path", filePath, "error", err)
		return api.NewFailedResponse(fmt.Sprintf("load file content failed: %s", filePath)), nil
	}

	message := doc.Content
	systemPrompt := api.GetStringParameter("system_prompt", request, "")
	p.logger.Infow("summary plugin started", "message_len", len(message), "has_system_prompt", systemPrompt != "")

	llm, err := NewLLMClient(p.config)
	if err != nil {
		p.logger.Warnw("create LLM client failed", "error", err)
		return api.NewFailedResponse(err.Error()), nil
	}

	agent := summarize.New("summary", "Summary Agent", llm, summarize.Option{
		SystemPrompt: systemPrompt,
	})

	resp := agent.Chat(ctx, &fridayapi.Request{
		Session:     NewSession(p.jobID),
		Memory:      memory.NewEmpty(p.jobID),
		UserMessage: message,
	})

	content, err := fridayapi.ReadAllContent(ctx, resp)
	if err != nil {
		p.logger.Warnw("collect response failed", "error", err)
		return api.NewFailedResponse(err.Error()), nil
	}

	p.logger.Infow("summary plugin completed", "result_len", len(content))
	return api.NewResponseWithResult(map[string]any{
		"file_path": filePath,
		"result":    strings.TrimSpace(content),
	}), nil
}

func NewSummaryPlugin(ps types.PluginCall) types.Plugin {
	return &SummaryPlugin{
		logger:     logger.NewPluginLogger(summaryPluginName, ps.JobID),
		fileAccess: utils.NewFileAccess(ps.WorkingPath),
		jobID:      ps.JobID,
		config:     ps.Config,
	}
}
