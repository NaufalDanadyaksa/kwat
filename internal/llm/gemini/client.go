package gemini

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

// Client implements llm.Provider for Google Gemini API.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a Gemini provider instance.
func NewClient(apiKey, baseURL string) *Client {
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
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
	return "gemini"
}

type geminiPart struct {
	Text             string              `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCall `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResp `json:"functionResponse,omitempty"`
}

type geminiFunctionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type geminiFunctionResp struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiRequest struct {
	Contents          []geminiContent `json:"contents"`
	SystemInstruction *geminiContent  `json:"systemInstruction,omitempty"`
	Tools             []geminiTool    `json:"tools,omitempty"`
}

type geminiTool struct {
	FunctionDeclarations []geminiFuncDecl `json:"functionDeclarations"`
}

type geminiFuncDecl struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// StreamChat executes a streaming request to Gemini API.
func (c *Client) StreamChat(ctx context.Context, req llm.ChatRequest, chunkChan chan<- llm.StreamChunk) (*llm.ChatResponse, error) {
	apiReq := geminiRequest{}

	for _, msg := range req.Messages {
		switch msg.Role {
		case llm.RoleSystem:
			apiReq.SystemInstruction = &geminiContent{
				Parts: []geminiPart{{Text: msg.Content}},
			}
		case llm.RoleUser:
			apiReq.Contents = append(apiReq.Contents, geminiContent{
				Role:  "user",
				Parts: []geminiPart{{Text: msg.Content}},
			})
		case llm.RoleAssistant:
			var parts []geminiPart
			if msg.Content != "" {
				parts = append(parts, geminiPart{Text: msg.Content})
			}
			for _, tc := range msg.ToolCalls {
				var args map[string]any
				_ = json.Unmarshal([]byte(tc.Arguments), &args)
				parts = append(parts, geminiPart{
					FunctionCall: &geminiFunctionCall{
						Name: tc.Name,
						Args: args,
					},
				})
			}
			if len(parts) > 0 {
				apiReq.Contents = append(apiReq.Contents, geminiContent{
					Role:  "model",
					Parts: parts,
				})
			}
		case llm.RoleTool:
			if msg.ToolResult != nil {
				var respMap map[string]any
				if err := json.Unmarshal([]byte(msg.ToolResult.Content), &respMap); err != nil {
					respMap = map[string]any{"output": msg.ToolResult.Content}
				}
				apiReq.Contents = append(apiReq.Contents, geminiContent{
					Role: "user",
					Parts: []geminiPart{
						{
							FunctionResponse: &geminiFunctionResp{
								Name:     msg.ToolResult.ToolName,
								Response: respMap,
							},
						},
					},
				})
			}
		}
	}

	if len(req.Tools) > 0 {
		var decls []geminiFuncDecl
		for _, t := range req.Tools {
			decls = append(decls, geminiFuncDecl{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			})
		}
		apiReq.Tools = []geminiTool{{FunctionDeclarations: decls}}
	}

	bodyBytes, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal gemini request: %w", err)
	}

	model := req.Model
	if model == "" {
		model = "gemini-1.5-pro-latest"
	}

	endpoint := fmt.Sprintf("%s/models/%s:streamGenerateContent?key=%s&alt=sse", c.baseURL, model, c.apiKey)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini request failed: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("gemini api error (status %d): %s", httpResp.StatusCode, string(respBody))
	}

	scanner := bufio.NewScanner(httpResp.Body)
	var fullContent strings.Builder
	var toolCalls []llm.ToolCall

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		var chunk struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text         string              `json:"text"`
						FunctionCall *geminiFunctionCall `json:"functionCall"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Candidates) == 0 {
			continue
		}

		for _, p := range chunk.Candidates[0].Content.Parts {
			if p.Text != "" {
				fullContent.WriteString(p.Text)
				if chunkChan != nil {
					chunkChan <- llm.StreamChunk{DeltaText: p.Text}
				}
			}
			if p.FunctionCall != nil {
				argsBytes, _ := json.Marshal(p.FunctionCall.Args)
				tc := llm.ToolCall{
					ID:        fmt.Sprintf("call_%d", time.Now().UnixNano()),
					Name:      p.FunctionCall.Name,
					Arguments: string(argsBytes),
				}
				toolCalls = append(toolCalls, tc)
			}
		}
	}

	finalResp := &llm.ChatResponse{
		Content:   fullContent.String(),
		ToolCalls: toolCalls,
	}

	if chunkChan != nil {
		if len(toolCalls) > 0 {
			chunkChan <- llm.StreamChunk{ToolCalls: toolCalls}
		}
		chunkChan <- llm.StreamChunk{Done: true}
	}

	return finalResp, nil
}

