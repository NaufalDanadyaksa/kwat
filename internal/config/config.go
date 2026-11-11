package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// ProviderConfig stores provider-specific configurations.
type ProviderConfig struct {
	APIKey  string `mapstructure:"api_key" json:"api_key"`
	BaseURL string `mapstructure:"base_url" json:"base_url"`
	Model   string `mapstructure:"model" json:"model"`
}

// Config represents the complete runtime configuration for KWAT.
type Config struct {
	DefaultProvider  string                    `mapstructure:"default_provider" json:"default_provider"`
	DefaultModel     string                    `mapstructure:"default_model" json:"default_model"`
	PermissionMode   string                    `mapstructure:"permission_mode" json:"permission_mode"`
	MaxContextTokens int                       `mapstructure:"max_context_tokens" json:"max_context_tokens"`
	AutoCompact      bool                      `mapstructure:"auto_compact" json:"auto_compact"`
	Providers        map[string]ProviderConfig `mapstructure:"providers" json:"providers"`
}

// DefaultConfig returns reasonable default settings for KWAT.
func DefaultConfig() *Config {
	return &Config{
		DefaultProvider:  "anthropic",
		DefaultModel:     "claude-3-5-sonnet-20241022",
		PermissionMode:   "granular",
		MaxContextTokens: 128000,
		AutoCompact:      true,
		Providers: map[string]ProviderConfig{
			"anthropic": {
				Model: "claude-3-5-sonnet-20241022",
			},
			"openai": {
				Model: "gpt-4o",
			},
			"gemini": {
				Model: "gemini-1.5-pro-latest",
			},
			"ollama": {
				BaseURL: "http://localhost:11434",
				Model:   "llama3.1",
			},
		},
	}
}

// LoadConfig loads configuration from file and environment variables.
func LoadConfig(customPath string) (*Config, error) {
	v := viper.New()

	v.SetConfigName("config")
	v.SetConfigType("yaml")

	if customPath != "" {
		v.SetConfigFile(customPath)
	} else {
		// Priority 1: Current workspace .kwat directory
		v.AddConfigPath("./.kwat")
		// Priority 2: User home directory ~/.kwat
		if home, err := os.UserHomeDir(); err == nil {
			v.AddConfigPath(filepath.Join(home, ".kwat"))
		}
	}

	// Environment variable overrides (e.g. KWAT_DEFAULT_MODEL, KWAT_PERMISSION_MODE)
	v.SetEnvPrefix("KWAT")
	v.AutomaticEnv()

	cfg := DefaultConfig()

	// If config file exists, read it
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok && customPath != "" {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	} else {
		if err := v.Unmarshal(cfg); err != nil {
			return nil, fmt.Errorf("error unmarshaling config: %w", err)
		}
	}

	// Fallback environment variable bindings for standard API keys
	if p, ok := cfg.Providers["anthropic"]; ok && p.APIKey == "" {
		p.APIKey = os.Getenv("ANTHROPIC_API_KEY")
		cfg.Providers["anthropic"] = p
	}
	if p, ok := cfg.Providers["openai"]; ok && p.APIKey == "" {
		p.APIKey = os.Getenv("OPENAI_API_KEY")
		cfg.Providers["openai"] = p
	}
	if p, ok := cfg.Providers["gemini"]; ok && p.APIKey == "" {
		p.APIKey = os.Getenv("GEMINI_API_KEY")
		cfg.Providers["gemini"] = p
	}

	return cfg, nil
}

