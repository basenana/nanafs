package agentic

import (
	"context"
	"fmt"
	"path/filepath"

	fridayapi "github.com/basenana/friday/core/api"
	"github.com/basenana/friday/core/providers/openai"
	"github.com/basenana/friday/core/types"
	"github.com/basenana/plugin/docloader"
)

const (
	ConfigHost   = "llm_host"
	ConfigAPIKey = "llm_api_key"
	ConfigModel  = "llm_model"
)

func NewLLMClient(config map[string]string) (openai.Client, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	host := config[ConfigHost]
	if host == "" {
		return nil, fmt.Errorf("llm_host is required")
	}

	apiKey := config[ConfigAPIKey]
	if apiKey == "" {
		return nil, fmt.Errorf("llm_api_key is required")
	}

	model := config[ConfigModel]
	if model == "" {
		return nil, fmt.Errorf("llm_model is required")
	}

	return openai.NewCompatible(host, apiKey, openai.Model{Name: model}), nil
}

func CollectResponse(ctx context.Context, resp *fridayapi.Response) (content string, reasoning string, err error) {
	content, err = fridayapi.ReadAllContent(ctx, resp)
	return
}

func NewSession(jobID string) *types.Session {
	return &types.Session{
		ID:   jobID,
		Type: types.SessionTypeAgentic,
	}
}

func LLMRequiredConfig() []string {
	return []string{ConfigHost, ConfigAPIKey, ConfigModel}
}

func newParser(docPath string) docloader.Parser {
	ext := filepath.Ext(docPath)
	switch ext {
	case ".pdf":
		return docloader.NewPDF(docPath, nil)
	case ".txt", ".md", ".markdown":
		return docloader.NewText(docPath, nil)
	case ".html", ".htm":
		return docloader.NewHTML(docPath, nil)
	case ".webarchive":
		return docloader.NewHTML(docPath, nil)
	case ".epub":
		return docloader.NewEPUB(docPath, nil)
	default:
		return nil
	}
}
