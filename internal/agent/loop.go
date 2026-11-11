package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/md101/kwat/internal/llm"
	"github.com/md101/kwat/internal/permission"
)

// ApproverFunc defines an interactive callback to request user approval.
// Returns (approved bool, alwaysAllow bool).
type ApproverFunc func(action permission.ActionType, toolName string, target string) (bool, bool)

// ReActEngine coordinates multi-turn reasoning and tool execution.
type ReActEngine struct {
	Provider       llm.Provider
	Model          string
	ToolExecutor   ToolExecutor
	PermManager    *permission.Manager
	Approver       ApproverFunc
	MaxSteps       int
	History        []llm.Message
	SystemPrompt   string
}

// NewReActEngine initializes a ReAct execution engine.
func NewReActEngine(provider llm.Provider, model string, executor ToolExecutor, perm *permission.Manager, systemPrompt string) *ReActEngine {
	if perm == nil {
		perm = permission.NewManager(permission.ModeGranular, false)
	}
	return &ReActEngine{
		Provider:     provider,
		Model:        model,
		ToolExecutor: executor,
		PermManager:  perm,
		MaxSteps:     15,
		SystemPrompt: systemPrompt,
		History:      make([]llm.Message, 0),
	}
}

// RunStep runs a single turn with user prompt through the ReAct loop until completion or MaxSteps.
func (e *ReActEngine) RunStep(ctx context.Context, userPrompt string, eventChan chan<- TurnEvent) (*TurnResult, error) {
	// Initialize system prompt on first turn
	if len(e.History) == 0 && e.SystemPrompt != "" {
		e.History = append(e.History, llm.Message{
			Role:    llm.RoleSystem,
			Content: e.SystemPrompt,
		})
	}

	if strings.TrimSpace(userPrompt) != "" {
		e.History = append(e.History, llm.Message{
			Role:    llm.RoleUser,
			Content: userPrompt,
		})
	}

	totalToolCalls := 0
	var lastAssistantText string

	for step := 1; step <= e.MaxSteps; step++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Retrieve active tools if an executor is provided
		var tools []llm.ToolDefinition
		if e.ToolExecutor != nil {
			tools = e.ToolExecutor.GetDefinitions()
		}

		req := llm.ChatRequest{
			Model:    e.Model,
			Messages: e.History,
			Tools:    tools,
		}

		// Channel for chunk streaming
		chunkChan := make(chan llm.StreamChunk, 64)
		var streamErr error
		var resp *llm.ChatResponse

		go func() {
			resp, streamErr = e.Provider.StreamChat(ctx, req, chunkChan)
			close(chunkChan)
		}()

		// Forward streaming text to caller
		for chunk := range chunkChan {
			if chunk.DeltaText != "" && eventChan != nil {
				eventChan <- TurnEvent{
					Type: EventTextDelta,
					Text: chunk.DeltaText,
				}
			}
			if chunk.Error != nil {
				streamErr = chunk.Error
			}
		}

		if streamErr != nil {
			if eventChan != nil {
				eventChan <- TurnEvent{Type: EventError, Error: streamErr}
			}
			return nil, fmt.Errorf("llm stream error at step %d: %w", step, streamErr)
		}

		if resp == nil {
			return nil, fmt.Errorf("received nil response from provider at step %d", step)
		}

		lastAssistantText = resp.Content

		// Add assistant response to history
		e.History = append(e.History, llm.Message{
			Role:      llm.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// If no tools were called, the turn is complete!
		if len(resp.ToolCalls) == 0 {
			if eventChan != nil {
				eventChan <- TurnEvent{
					Type: EventTurnComplete,
					Text: resp.Content,
				}
			}
			return &TurnResult{
				FinalResponse:  resp.Content,
				TotalToolCalls: totalToolCalls,
				Messages:       e.History,
			}, nil
		}

		// Execute tool calls sequentially
		for _, tc := range resp.ToolCalls {
			totalToolCalls++

			if eventChan != nil {
				eventChan <- TurnEvent{
					Type:     EventToolCall,
					ToolCall: &tc,
				}
			}

			// Determine permission check
			action := inferActionType(tc.Name)
			if e.PermManager.RequiresConfirmation(action, tc.Name, tc.Arguments) {
				if e.PermManager.CurrentMode() == permission.ModeSandbox {
					// In Sandbox mode, prevent execution and return preview
					toolResult := &llm.ToolResult{
						ToolCallID: tc.ID,
						ToolName:   tc.Name,
						Content:    fmt.Sprintf("[Sandbox Mode]: Tool %s(%s) previewed. Execution safely skipped.", tc.Name, tc.Arguments),
						IsError:    false,
					}
					e.History = append(e.History, llm.Message{
						Role:       llm.RoleTool,
						ToolResult: toolResult,
					})
					if eventChan != nil {
						eventChan <- TurnEvent{
							Type:       EventToolResult,
							ToolResult: toolResult,
						}
					}
					continue
				}

				approved := false
				if e.Approver != nil {
					var always bool
					approved, always = e.Approver(action, tc.Name, tc.Arguments)
					if always {
						e.PermManager.Whitelist(tc.Name, tc.Arguments)
					}
				}

				if !approved {
					toolResult := &llm.ToolResult{
						ToolCallID: tc.ID,
						ToolName:   tc.Name,
						Content:    fmt.Sprintf("Tool %s execution denied: permission not granted.", tc.Name),
						IsError:    true,
					}
					e.History = append(e.History, llm.Message{
						Role:       llm.RoleTool,
						ToolResult: toolResult,
					})
					if eventChan != nil {
						eventChan <- TurnEvent{
							Type:       EventToolResult,
							ToolResult: toolResult,
						}
					}
					continue
				}
			}

			var toolResult *llm.ToolResult
			if e.ToolExecutor != nil {
				var err error
				toolResult, err = e.ToolExecutor.Execute(ctx, tc)
				if err != nil {
					toolResult = &llm.ToolResult{
						ToolCallID: tc.ID,
						ToolName:   tc.Name,
						Content:    fmt.Sprintf("Tool execution error: %v", err),
						IsError:    true,
					}
				}
			} else {
				toolResult = &llm.ToolResult{
					ToolCallID: tc.ID,
					ToolName:   tc.Name,
					Content:    fmt.Sprintf("No tool executor configured for tool %s", tc.Name),
					IsError:    true,
				}
			}

			e.History = append(e.History, llm.Message{
				Role:       llm.RoleTool,
				ToolResult: toolResult,
			})

			if eventChan != nil {
				eventChan <- TurnEvent{
					Type:       EventToolResult,
					ToolResult: toolResult,
				}
			}
		}
	}

	return &TurnResult{
		FinalResponse:  lastAssistantText,
		TotalToolCalls: totalToolCalls,
		Messages:       e.History,
	}, nil
}

func inferActionType(toolName string) permission.ActionType {
	switch strings.ToLower(toolName) {
	case "read_file", "grep", "glob", "file_search", "list_dir":
		return permission.ActionRead
	case "write_file", "edit_file", "patch_file", "delete_file":
		return permission.ActionWrite
	case "bash", "exec", "run_command":
		return permission.ActionExecute
	default:
		return permission.ActionExecute
	}
}

