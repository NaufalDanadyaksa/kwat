package anthropic

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

// Client implements llm.Provider for Anthropic Claude.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewClient creates an Anthropic provider instance.
func NewClient(apiKey, baseURL string) *Client {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	return &Client{
		apiKey:  apiKey,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (c *Client) Name() string {
	return "anthropic"
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	System      string             `json:"system,omitempty"`
	Tools       []anthropicTool    `json:"tools,omitempty"`
	Stream      bool               `json:"stream"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float32            `json:"temperature,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string or []anthropicContentBlock
}

type anthropicContentBlock struct {
	Type      string `json:"type"` // "text", "tool_use", "tool_result"
	Text      string `json:"text,omitempty"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Input     any    `json:"input,omitempty"`
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

// StreamChat executes a streaming request to Anthropic Messages API.
func (c *Client) StreamChat(ctx context.Context, req llm.ChatRequest, chunkChan chan<- llm.StreamChunk) (*llm.ChatResponse, error) {
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 8192
	}

	apiReq := anthropicRequest{
		Model:       req.Model,
		Stream:      true,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
	}

	for _, msg := range req.Messages {
		switch msg.Role {
		case llm.RoleSystem:
			if apiReq.System != "" {
				apiReq.System += "\n\n" + msg.Content
			} else {
				apiReq.System = msg.Content
			}
		case llm.RoleUser:
			apiReq.Messages = append(apiReq.Messages, anthropicMessage{
				Role:    "user",
				Content: msg.Content,
			})
		case llm.RoleAssistant:
			var blocks []anthropicContentBlock
			if msg.Content != "" {
				blocks = append(blocks, anthropicContentBlock{
					Type: "text",
					Text: msg.Content,
				})
			}
			for _, tc := range msg.ToolCalls {
				var parsedInput any
				_ = json.Unmarshal([]byte(tc.Arguments), &parsedInput)
				blocks = append(blocks, anthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: parsedInput,
				})
			}
			if len(blocks) > 0 {
				apiReq.Messages = append(apiReq.Messages, anthropicMessage{
					Role:    "assistant",
					Content: blocks,
				})
			}
		case llm.RoleTool:
			if msg.ToolResult != nil {
				// Tool results are sent in Anthropic as user messages with tool_result blocks
				block := anthropicContentBlock{
					Type:      "tool_result",
					ToolUseID: msg.ToolResult.ToolCallID,
					Content:   msg.ToolResult.Content,
					IsError:   msg.ToolResult.IsError,
				}
				apiReq.Messages = append(apiReq.Messages, anthropicMessage{
					Role:    "user",
					Content: []anthropicContentBlock{block},
				})
			}
		}
	}

	for _, t := range req.Tools {
		apiReq.Tools = append(apiReq.Tools, anthropicTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.Parameters,
		})
	}

	bodyBytes, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal anthropic request: %w", err)
	}

	endpoint := c.baseURL + "/messages"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic request failed: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("anthropic api error (status %d): %s", httpResp.StatusCode, string(respBody))
	}

	scanner := bufio.NewScanner(httpResp.Body)
	var fullContent strings.Builder
	toolCallsMap := make(map[int]*llm.ToolCall)
	var currentBlockIndex int

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		var event struct {
			Type         string `json:"type"`
			Index        int    `json:"index"`
			ContentBlock struct {
				Type string `json:"type"`
				ID   string `json:"id"`
				Name string `json:"name"`
				Text string `json:"text"`
			} `json:"content_block"`
			Delta struct {
				Type         string `json:"type"`
				Text         string `json:"text"`
				PartialJSON  string `json:"partial_json"`
				StopReason   string `json:"stop_reason"`
			} `json:"delta"`
		}

		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "content_block_start":
			currentBlockIndex = event.Index
			if event.ContentBlock.Type == "tool_use" {
				toolCallsMap[currentBlockIndex] = &llm.ToolCall{
					ID:   event.ContentBlock.ID,
					Name: event.ContentBlock.Name,
				}
			}
		case "content_block_delta":
			if event.Delta.Type == "text_delta" && event.Delta.Text != "" {
				fullContent.WriteString(event.Delta.Text)
				if chunkChan != nil {
					chunkChan <- llm.StreamChunk{DeltaText: event.Delta.Text}
				}
			} else if event.Delta.Type == "input_json_delta" {
				if tc, ok := toolCallsMap[event.Index]; ok {
					tc.Arguments += event.Delta.PartialJSON
				}
			}
		case "message_stop":
			break
		}
	}

	var finalizedToolCalls []llm.ToolCall
	for i := 0; i <= len(toolCallsMap)+1; i++ {
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

