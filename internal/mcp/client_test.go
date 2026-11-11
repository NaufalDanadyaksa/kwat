package mcp_test

import (
	"context"
	"testing"

	"github.com/md101/kwat/internal/llm"
	"github.com/md101/kwat/internal/mcp"
	"github.com/md101/kwat/internal/tools"
)

func TestToolAdapter(t *testing.T) {
	def := llm.ToolDefinition{
		Name:        "mcp__db__query",
		Description: "Run SQL query",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"sql": map[string]any{"type": "string"},
			},
		},
	}

	adapter := mcp.NewToolAdapter(nil, "query", def)
	if adapter.Name() != "mcp__db__query" {
		t.Errorf("expected name 'mcp__db__query', got %s", adapter.Name())
	}
	if adapter.Description() != "Run SQL query" {
		t.Errorf("expected desc 'Run SQL query', got %s", adapter.Description())
	}

	// Register in tools.Registry
	reg := tools.NewRegistry()
	reg.Register(adapter)

	registeredDefs := reg.GetDefinitions()
	if len(registeredDefs) != 1 || registeredDefs[0].Name != "mcp__db__query" {
		t.Errorf("failed to register MCP adapter in registry")
	}
}

func TestStartClientValidation(t *testing.T) {
	_, err := mcp.StartClient(context.Background(), "invalid", mcp.ServerConfig{
		Command: "",
	})
	if err == nil {
		t.Errorf("expected error for empty command")
	}
}

