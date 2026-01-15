package agentic

import (
	"fmt"
	"path/filepath"

	"github.com/basenana/friday/core/providers/openai"
	"github.com/basenana/friday/core/types"
	"github.com/basenana/plugin/docloader"
)

const (
	ConfigHost   = "friday_llm_host"
	ConfigAPIKey = "friday_llm_api_key"
	ConfigModel  = "friday_llm_model"
)

func NewLLMClient(config map[string]string) (openai.Client, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	host := config[ConfigHost]
	if host == "" {
		return nil, fmt.Errorf("friday_llm_host is required")
	}

	apiKey := config[ConfigAPIKey]
	if apiKey == "" {
		return nil, fmt.Errorf("friday_llm_api_key is required")
	}

	model := config[ConfigModel]
	if model == "" {
		return nil, fmt.Errorf("friday_llm_model is required")
	}

	return openai.New(host, apiKey, openai.Model{Name: model}), nil
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

func formatSize(size int64) string {
	const (
		unit  = 1024
		units = "KMGTPE"
	)
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit && exp < len(units)-1; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), units[exp])
}
