package llm

import (
	"context"
)

// Role defines the role of a message sender in the conversation.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ToolCall represents a model's request to invoke a specific tool.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON-encoded arguments
}

// ToolResult represents the output of an executed tool call.
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	ToolName   string `json:"tool_name,omitempty"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error"`
}

// Message represents a single message in the conversation history.
type Message struct {
	Role       Role        `json:"role"`
	Content    string      `json:"content"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolResult *ToolResult `json:"tool_result,omitempty"`
}

// ToolDefinition describes a tool schema in standard JSON Schema format.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ChatRequest encapsulates all parameters needed to query an LLM.
type ChatRequest struct {
	Model       string           `json:"model"`
	Messages    []Message        `json:"messages"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	Temperature float32          `json:"temperature,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
}

// StreamChunk conveys a streaming response delta from the provider.
type StreamChunk struct {
	DeltaText string
	ToolCalls []ToolCall
	Done      bool
	Error     error
}

// ChatResponse contains the finalized response from an LLM call.
type ChatResponse struct {
	Content      string     `json:"content"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	InputTokens  int        `json:"input_tokens,omitempty"`
	OutputTokens int        `json:"output_tokens,omitempty"`
	StopReason   string     `json:"stop_reason,omitempty"`
}

// Provider defines the common interface implemented by all model adapters.
type Provider interface {
	Name() string
	StreamChat(ctx context.Context, req ChatRequest, chunkChan chan<- StreamChunk) (*ChatResponse, error)
}

