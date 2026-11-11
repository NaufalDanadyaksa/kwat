package mock

import (
	"context"

	"github.com/md101/kwat/internal/llm"
)

// MockProvider is a programmable mock implementing llm.Provider.
type MockProvider struct {
	Responses   []*llm.ChatResponse
	StreamDeltas [][]string
	CallIndex   int
}

// NewMockProvider creates a new mock provider.
func NewMockProvider(responses ...*llm.ChatResponse) *MockProvider {
	return &MockProvider{
		Responses: responses,
	}
}

func (m *MockProvider) Name() string {
	return "mock"
}

func (m *MockProvider) StreamChat(ctx context.Context, req llm.ChatRequest, chunkChan chan<- llm.StreamChunk) (*llm.ChatResponse, error) {
	if m.CallIndex >= len(m.Responses) {
		resp := &llm.ChatResponse{
			Content: "Default mock response",
		}
		if chunkChan != nil {
			chunkChan <- llm.StreamChunk{DeltaText: resp.Content, Done: true}
		}
		return resp, nil
	}

	resp := m.Responses[m.CallIndex]
	m.CallIndex++

	if chunkChan != nil {
		if resp.Content != "" {
			chunkChan <- llm.StreamChunk{DeltaText: resp.Content}
		}
		if len(resp.ToolCalls) > 0 {
			chunkChan <- llm.StreamChunk{ToolCalls: resp.ToolCalls}
		}
		chunkChan <- llm.StreamChunk{Done: true}
	}

	return resp, nil
}

