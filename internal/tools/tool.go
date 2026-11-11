package tools

import (
	"context"
	"fmt"
	"sync"

	"github.com/md101/kwat/internal/llm"
)

// Tool represents a native executable capability exposed to the LLM agent.
type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]any
	Execute(ctx context.Context, argsJSON string) (string, error)
}

// Registry manages the collection of tools and implements agent.ToolExecutor.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry initializes an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry.
func (r *Registry) Register(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name()] = tool
}

// Get finds a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// GetDefinitions returns the JSON schema definitions for all registered tools.
func (r *Registry) GetDefinitions() []llm.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]llm.ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, llm.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		})
	}
	return defs
}

// Execute executes the requested tool call and returns the result.
func (r *Registry) Execute(ctx context.Context, call llm.ToolCall) (*llm.ToolResult, error) {
	tool, ok := r.Get(call.Name)
	if !ok {
		return &llm.ToolResult{
			ToolCallID: call.ID,
			ToolName:   call.Name,
			Content:    fmt.Sprintf("unknown tool: %s", call.Name),
			IsError:    true,
		}, nil
	}

	output, err := tool.Execute(ctx, call.Arguments)
	if err != nil {
		return &llm.ToolResult{
			ToolCallID: call.ID,
			ToolName:   call.Name,
			Content:    fmt.Sprintf("error executing %s: %v", call.Name, err),
			IsError:    true,
		}, nil
	}

	return &llm.ToolResult{
		ToolCallID: call.ID,
		ToolName:   call.Name,
		Content:    output,
		IsError:    false,
	}, nil
}

// DefaultRegistry creates a registry populated with all core tools.
func DefaultRegistry(workDir string) *Registry {
	r := NewRegistry()
	r.Register(NewReadFileTool(workDir))
	r.Register(NewWriteFileTool(workDir))
	r.Register(NewEditFileTool(workDir))
	r.Register(NewGlobTool(workDir))
	r.Register(NewGrepTool(workDir))
	r.Register(NewBashTool(workDir))
	return r
}

