package agent

import (
	"context"

	"github.com/md101/kwat/internal/llm"
)

// ToolExecutor defines how an agent invokes tools and retrieves their schemas.
type ToolExecutor interface {
	GetDefinitions() []llm.ToolDefinition
	Execute(ctx context.Context, call llm.ToolCall) (*llm.ToolResult, error)
}

// EventType describes the nature of an event emitted during an agent turn.
type EventType string

const (
	EventTextDelta      EventType = "text_delta"
	EventToolCall       EventType = "tool_call"
	EventToolResult     EventType = "tool_result"
	EventTurnComplete   EventType = "turn_complete"
	EventError          EventType = "error"
)

// TurnEvent is emitted over channels for real-time UI/CLI feedback.
type TurnEvent struct {
	Type       EventType       `json:"type"`
	Text       string          `json:"text,omitempty"`
	ToolCall   *llm.ToolCall   `json:"tool_call,omitempty"`
	ToolResult *llm.ToolResult `json:"tool_result,omitempty"`
	Error      error           `json:"error,omitempty"`
}

// TurnResult aggregates the outcome of an agent turn.
type TurnResult struct {
	FinalResponse  string        `json:"final_response"`
	TotalToolCalls int           `json:"total_tool_calls"`
	Messages       []llm.Message `json:"messages"`
}

