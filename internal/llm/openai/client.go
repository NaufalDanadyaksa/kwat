package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/md101/kwat/internal/llm"
)

// Client implements llm.Provider for OpenAI and OpenAI-compatible endpoints (Ollama, vLLM, OpenRouter, etc.).
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	name       string
}

// NewClient creates an OpenAI provider instance.
func NewClient(name, apiKey, baseURL string) *Client {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	return &Client{
		name:    name,
		apiKey:  apiKey,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (c *Client) Name() string {
	if c.name != "" {
		return c.name
	}
	return "openai"
}

type openAIMessage struct {
	Role       string            `json:"role"`
	Content    string            `json:"content,omitempty"`
	ToolCalls  []openAIToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
}

type openAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openAITool struct {
	Type     string             `json:"type"`
	Function openAIToolFunction `json:"function"`
}

type openAIToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type openAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Tools       []openAITool    `json:"tools,omitempty"`
	Stream      bool            `json:"stream"`
	Temperature float32         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
}

// StreamChat executes a streaming request to OpenAI / OpenAI-compatible endpoint.
func (c *Client) StreamChat(ctx context.Context, req llm.ChatRequest, chunkChan chan<- llm.StreamChunk) (*llm.ChatResponse, error) {
	apiReq := openAIChatRequest{
		Model:       req.Model,
		Stream:      true,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}

	for _, msg := range req.Messages {
		switch msg.Role {
		case llm.RoleSystem:
			apiReq.Messages = append(apiReq.Messages, openAIMessage{
				Role:    "system",
				Content: msg.Content,
			})
		case llm.RoleUser:
			apiReq.Messages = append(apiReq.Messages, openAIMessage{
				Role:    "user",
				Content: msg.Content,
			})
		case llm.RoleAssistant:
			m := openAIMessage{
				Role:    "assistant",
				Content: msg.Content,
			}
			for _, tc := range msg.ToolCalls {
				var otc openAIToolCall
				otc.ID = tc.ID
				otc.Type = "function"
				otc.Function.Name = tc.Name
				otc.Function.Arguments = tc.Arguments
				m.ToolCalls = append(m.ToolCalls, otc)
			}
			apiReq.Messages = append(apiReq.Messages, m)
		case llm.RoleTool:
			if msg.ToolResult != nil {
				apiReq.Messages = append(apiReq.Messages, openAIMessage{
					Role:       "tool",
					Content:    msg.ToolResult.Content,
					ToolCallID: msg.ToolResult.ToolCallID,
				})
			}
		}
	}

	for _, t := range req.Tools {
		apiReq.Tools = append(apiReq.Tools, openAITool{
			Type: "function",
			Function: openAIToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}

	bodyBytes, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal openai request: %w", err)
	}

	endpoint := c.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai request failed: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("openai api error (status %d): %s", httpResp.StatusCode, string(respBody))
	}

	// Parse SSE stream
	scanner := bufio.NewScanner(httpResp.Body)
	var fullContent strings.Builder
	toolCallsMap := make(map[int]*llm.ToolCall)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if strings.TrimSpace(data) == "[DONE]" {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content   string `json:"content"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta
		if delta.Content != "" {
			fullContent.WriteString(delta.Content)
			if chunkChan != nil {
				chunkChan <- llm.StreamChunk{DeltaText: delta.Content}
			}
		}

		for _, tc := range delta.ToolCalls {
			existing, ok := toolCallsMap[tc.Index]
			if !ok {
				existing = &llm.ToolCall{
					ID:   tc.ID,
					Name: tc.Function.Name,
				}
				toolCallsMap[tc.Index] = existing
			}
			if tc.ID != "" {
				existing.ID = tc.ID
			}
			if tc.Function.Name != "" {
				existing.Name = tc.Function.Name
			}
			existing.Arguments += tc.Function.Arguments
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading sse stream: %w", err)
	}

	var finalizedToolCalls []llm.ToolCall
	for i := 0; i < len(toolCallsMap); i++ {
		if tc, ok := toolCallsMap[i]; ok {
			finalizedToolCalls = append(finalizedToolCalls, *tc)
		}
	}

	finalResp := &llm.ChatResponse{
		Content:   fullContent.String(),
		ToolCalls: finalizedToolCalls,
	}

	if chunkChan != nil {
		if len(finalizedToolCalls) > 0 {
			chunkChan <- llm.StreamChunk{ToolCalls: finalizedToolCalls}
		}
		chunkChan <- llm.StreamChunk{Done: true}
	}

	return finalResp, nil
}

