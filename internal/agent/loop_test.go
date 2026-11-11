package agent_test

import (
	"context"
	"testing"

	"github.com/md101/kwat/internal/agent"
	"github.com/md101/kwat/internal/llm"
	"github.com/md101/kwat/internal/llm/mock"
	"github.com/md101/kwat/internal/permission"
)

type mockToolExecutor struct {
	executedCalls []llm.ToolCall
}

func (m *mockToolExecutor) GetDefinitions() []llm.ToolDefinition {
	return []llm.ToolDefinition{
		{
			Name:        "read_file",
			Description: "Read file contents",
		},
		{
			Name:        "bash",
			Description: "Execute shell command",
		},
	}
}

func (m *mockToolExecutor) Execute(ctx context.Context, call llm.ToolCall) (*llm.ToolResult, error) {
	m.executedCalls = append(m.executedCalls, call)
	if call.Name == "read_file" {
		return &llm.ToolResult{
			ToolCallID: call.ID,
			ToolName:   call.Name,
			Content:    "file content: hello world",
		}, nil
	}
	return &llm.ToolResult{
		ToolCallID: call.ID,
		ToolName:   call.Name,
		Content:    "command output",
	}, nil
}

func TestReActLoop_DirectAnswer(t *testing.T) {
	mockProv := mock.NewMockProvider(&llm.ChatResponse{
		Content: "Hello! How can I help you code today?",
	})

	perm := permission.NewManager(permission.ModeGranular, false)
	engine := agent.NewReActEngine(mockProv, "test-model", nil, perm, "system instruction")

	events := make(chan agent.TurnEvent, 10)
	var capturedDeltas []string

	go func() {
		for ev := range events {
			if ev.Type == agent.EventTextDelta {
				capturedDeltas = append(capturedDeltas, ev.Text)
			}
		}
	}()

	result, err := engine.RunStep(context.Background(), "Hi", events)
	close(events)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalResponse != "Hello! How can I help you code today?" {
		t.Errorf("unexpected final response: %q", result.FinalResponse)
	}
	if result.TotalToolCalls != 0 {
		t.Errorf("expected 0 tool calls, got %d", result.TotalToolCalls)
	}
}

func TestReActLoop_WithToolCall(t *testing.T) {
	// First response: request to read file
	step1 := &llm.ChatResponse{
		Content: "Let me check that file first.",
		ToolCalls: []llm.ToolCall{
			{
				ID:        "call_123",
				Name:      "read_file",
				Arguments: `{"path": "hello.txt"}`,
			},
		},
	}
	// Second response: model answers based on file content
	step2 := &llm.ChatResponse{
		Content: "The file contains: hello world",
	}

	mockProv := mock.NewMockProvider(step1, step2)
	executor := &mockToolExecutor{}
	perm := permission.NewManager(permission.ModeGranular, false)

	engine := agent.NewReActEngine(mockProv, "test-model", executor, perm, "system prompt")

	events := make(chan agent.TurnEvent, 20)
	var capturedEvents []agent.TurnEvent
	done := make(chan struct{})

	go func() {
		for ev := range events {
			capturedEvents = append(capturedEvents, ev)
		}
		close(done)
	}()

	result, err := engine.RunStep(context.Background(), "what is in hello.txt?", events)
	close(events)
	<-done

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalToolCalls != 1 {
		t.Errorf("expected 1 tool call, got %d", result.TotalToolCalls)
	}
	if len(executor.executedCalls) != 1 {
		t.Fatalf("expected 1 executed call in mock executor, got %d", len(executor.executedCalls))
	}
	if executor.executedCalls[0].Name != "read_file" {
		t.Errorf("expected executed tool to be 'read_file', got %s", executor.executedCalls[0].Name)
	}
	if result.FinalResponse != "The file contains: hello world" {
		t.Errorf("unexpected final response: %q", result.FinalResponse)
	}
}

func TestReActLoop_PermissionDenied(t *testing.T) {
	// Tool call for bash in strict mode (requires confirmation)
	step1 := &llm.ChatResponse{
		ToolCalls: []llm.ToolCall{
			{
				ID:        "call_bash_1",
				Name:      "bash",
				Arguments: "rm -rf /",
			},
		},
	}
	step2 := &llm.ChatResponse{
		Content: "I understand the command was denied.",
	}

	mockProv := mock.NewMockProvider(step1, step2)
	executor := &mockToolExecutor{}
	// Strict mode requires confirmation even for read
	perm := permission.NewManager(permission.ModeStrict, false)

	engine := agent.NewReActEngine(mockProv, "test-model", executor, perm, "sys")

	result, err := engine.RunStep(context.Background(), "delete everything", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Tool should NOT have been executed by executor
	if len(executor.executedCalls) != 0 {
		t.Errorf("expected 0 executed calls due to denial, got %d", len(executor.executedCalls))
	}

	if result.FinalResponse != "I understand the command was denied." {
		t.Errorf("unexpected final response: %q", result.FinalResponse)
	}
}

