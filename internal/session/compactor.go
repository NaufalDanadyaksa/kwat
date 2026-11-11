package session

import (
	"fmt"
	"strings"

	"github.com/md101/kwat/internal/llm"
)

// EstimateTokens calculates an approximate token count based on string length (~4 chars per token).
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	tokens := len(text) / 4
	if tokens == 0 {
		return 1
	}
	return tokens
}

// EstimateMessagesTokens computes the estimated tokens across all messages in a conversation.
func EstimateMessagesTokens(messages []llm.Message) int {
	total := 0
	for _, m := range messages {
		total += EstimateTokens(m.Content)
		for _, tc := range m.ToolCalls {
			total += EstimateTokens(tc.Name) + EstimateTokens(tc.Arguments)
		}
		if m.ToolResult != nil {
			total += EstimateTokens(m.ToolResult.Content)
		}
	}
	return total
}

// NeedsCompaction determines if conversation history exceeds the compaction threshold (75% of maxTokens).
func NeedsCompaction(messages []llm.Message, maxTokens int) bool {
	if maxTokens <= 0 {
		return false
	}
	currentTokens := EstimateMessagesTokens(messages)
	threshold := int(float64(maxTokens) * 0.75)
	return currentTokens >= threshold
}

// Compact summarizes and prunes earlier messages while preserving the system prompt and recent turns.
func Compact(messages []llm.Message, keepRecent int) []llm.Message {
	if len(messages) <= keepRecent+2 {
		return messages
	}

	hasSystem := len(messages) > 0 && messages[0].Role == llm.RoleSystem
	startIdx := 0
	if hasSystem {
		startIdx = 1
	}

	splitIdx := len(messages) - keepRecent
	if splitIdx <= startIdx {
		return messages
	}

	// Generate summary of pruned messages
	var summaryBuilder strings.Builder
	summaryBuilder.WriteString("[Session History Checkpoint - Older turns summarized for token compaction]:\n")

	for i := startIdx; i < splitIdx; i++ {
		msg := messages[i]
		switch msg.Role {
		case llm.RoleUser:
			summaryBuilder.WriteString(fmt.Sprintf("- User asked: %s\n", truncateText(msg.Content, 100)))
		case llm.RoleAssistant:
			if len(msg.ToolCalls) > 0 {
				var toolNames []string
				for _, tc := range msg.ToolCalls {
					toolNames = append(toolNames, tc.Name)
				}
				summaryBuilder.WriteString(fmt.Sprintf("- KWAT invoked tools: %s\n", strings.Join(toolNames, ", ")))
			} else {
				summaryBuilder.WriteString(fmt.Sprintf("- KWAT answered: %s\n", truncateText(msg.Content, 100)))
			}
		case llm.RoleTool:
			if msg.ToolResult != nil {
				summaryBuilder.WriteString(fmt.Sprintf("  (Tool result for %s recorded)\n", msg.ToolResult.ToolName))
			}
		}
	}

	checkpointMessage := llm.Message{
		Role:    llm.RoleSystem,
		Content: summaryBuilder.String(),
	}

	compacted := make([]llm.Message, 0, 2+keepRecent)
	if hasSystem {
		compacted = append(compacted, messages[0])
	}
	compacted = append(compacted, checkpointMessage)
	compacted = append(compacted, messages[splitIdx:]...)

	return compacted
}

func truncateText(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

