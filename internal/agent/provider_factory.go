package agent

import (
	"fmt"
	"strings"

	"github.com/md101/kwat/internal/config"
	"github.com/md101/kwat/internal/llm"
	"github.com/md101/kwat/internal/llm/anthropic"
	"github.com/md101/kwat/internal/llm/gemini"
	"github.com/md101/kwat/internal/llm/openai"
)

// CreateProvider instantiates the appropriate LLM provider for the agent.
func CreateProvider(providerName string, cfg config.ProviderConfig) (llm.Provider, error) {
	name := strings.ToLower(strings.TrimSpace(providerName))

	switch name {
	case "anthropic", "claude":
		return anthropic.NewClient(cfg.APIKey, cfg.BaseURL), nil
	case "openai":
		return openai.NewClient("openai", cfg.APIKey, cfg.BaseURL), nil
	case "ollama":
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		if !strings.HasSuffix(baseURL, "/v1") {
			baseURL = strings.TrimRight(baseURL, "/") + "/v1"
		}
		return openai.NewClient("ollama", cfg.APIKey, baseURL), nil
	case "gemini", "google":
		return gemini.NewClient(cfg.APIKey, cfg.BaseURL), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %q (supported: anthropic, openai, gemini, ollama)", providerName)
	}
}

