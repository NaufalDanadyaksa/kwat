package mcp

import (
	"context"
	"strings"

	"github.com/md101/kwat/internal/llm"
	"github.com/md101/kwat/internal/tools"
)

// ToolAdapter adapts an external MCP tool into the native tools.Tool interface.
type ToolAdapter struct {
	client     *Client
	rawName    string
	name       string
	desc       string
	parameters map[string]any
}

// NewToolAdapter wraps an external MCP tool definition.
func NewToolAdapter(client *Client, rawName string, def llm.ToolDefinition) *ToolAdapter {
	return &ToolAdapter{
		client:     client,
		rawName:    rawName,
		name:       def.Name,
		desc:       def.Description,
		parameters: def.Parameters,
	}
}

func (a *ToolAdapter) Name() string {
	return a.name
}

func (a *ToolAdapter) Description() string {
	return a.desc
}

func (a *ToolAdapter) Parameters() map[string]any {
	return a.parameters
}

func (a *ToolAdapter) Execute(ctx context.Context, argsJSON string) (string, error) {
	return a.client.CallTool(ctx, a.rawName, argsJSON)
}

// RegisterMCPServer connects to an MCP server, lists its tools, and adds them to the registry.
func RegisterMCPServer(ctx context.Context, registry *tools.Registry, serverName string, cfg ServerConfig) (*Client, error) {
	client, err := StartClient(ctx, serverName, cfg)
	if err != nil {
		return nil, err
	}

	defs, err := client.ListTools(ctx)
	if err != nil {
		client.Close()
		return nil, err
	}

	for _, def := range defs {
		// rawName is the tool name without server prefix
		prefix := "mcp__" + serverName + "__"
		rawName := strings.TrimPrefix(def.Name, prefix)
		adapter := NewToolAdapter(client, rawName, def)
		registry.Register(adapter)
	}

	return client, nil
}

