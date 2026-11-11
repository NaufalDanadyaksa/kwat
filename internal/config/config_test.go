package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.DefaultProvider == "" {
		t.Errorf("expected non-empty default provider")
	}
	if cfg.PermissionMode != "granular" {
		t.Errorf("expected default permission mode 'granular', got %s", cfg.PermissionMode)
	}
	if _, ok := cfg.Providers["anthropic"]; !ok {
		t.Errorf("expected anthropic provider configured in defaults")
	}
}

func TestLoadProjectRules(t *testing.T) {
	tmpDir := t.TempDir()

	// Initially no rules
	rules, path, err := LoadProjectRules(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rules != "" || path != "" {
		t.Errorf("expected empty rules for empty dir, got %q", rules)
	}

	// Write KWAT.md
	ruleContent := "Always write clean idiomatic Go code with unit tests."
	rulePath := filepath.Join(tmpDir, "KWAT.md")
	if err := os.WriteFile(rulePath, []byte(ruleContent), 0644); err != nil {
		t.Fatalf("failed to write rule file: %v", err)
	}

	rules, loadedPath, err := LoadProjectRules(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rules != ruleContent {
		t.Errorf("expected %q, got %q", ruleContent, rules)
	}
	if loadedPath != rulePath {
		t.Errorf("expected loadedPath %q, got %q", rulePath, loadedPath)
	}
}

func TestLoadConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	sampleYAML := `
default_provider: "openai"
default_model: "gpt-4o-mini"
permission_mode: "strict"
max_context_tokens: 64000
`
	if err := os.WriteFile(configFile, []byte(sampleYAML), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("failed to load custom config: %v", err)
	}

	if cfg.DefaultProvider != "openai" {
		t.Errorf("expected default_provider 'openai', got %s", cfg.DefaultProvider)
	}
	if cfg.DefaultModel != "gpt-4o-mini" {
		t.Errorf("expected default_model 'gpt-4o-mini', got %s", cfg.DefaultModel)
	}
	if cfg.PermissionMode != "strict" {
		t.Errorf("expected permission_mode 'strict', got %s", cfg.PermissionMode)
	}
}

