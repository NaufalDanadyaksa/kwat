package session_test

import (
	"strings"
	"testing"
	"time"

	"github.com/md101/kwat/internal/llm"
	"github.com/md101/kwat/internal/session"
)

func TestSessionStore(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := session.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// 1. Create & Save Session
	sess := session.NewSession("anthropic", "claude-3-5-sonnet", "/workspace")
	sess.Messages = append(sess.Messages, llm.Message{
		Role:    llm.RoleUser,
		Content: "Hello KWAT",
	})
	sess.TotalTokens = 150

	if err := store.Save(sess); err != nil {
		t.Fatalf("failed to save session: %v", err)
	}

	// 2. Load Session
	loaded, err := store.Load(sess.ID)
	if err != nil {
		t.Fatalf("failed to load session: %v", err)
	}
	if loaded.ID != sess.ID || loaded.Model != sess.Model {
		t.Errorf("loaded session mismatch: got %+v, want %+v", loaded, sess)
	}
	if len(loaded.Messages) != 1 || loaded.Messages[0].Content != "Hello KWAT" {
		t.Errorf("messages mismatch in loaded session")
	}

	// 3. List & GetLatest
	time.Sleep(10 * time.Millisecond)
	sess2 := session.NewSession("openai", "gpt-4o", "/workspace")
	_ = store.Save(sess2)

	list, err := store.List()
	if err != nil {
		t.Fatalf("failed to list sessions: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(list))
	}
	if list[0].ID != sess2.ID {
		t.Errorf("expected latest session to be sess2, got %s", list[0].ID)
	}

	latest, err := store.GetLatest()
	if err != nil {
		t.Fatalf("failed to get latest: %v", err)
	}
	if latest.ID != sess2.ID {
		t.Errorf("expected GetLatest to be sess2, got %s", latest.ID)
	}
}

func TestCompactor(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: "System prompt"},
		{Role: llm.RoleUser, Content: "Turn 1: Fix bug"},
		{Role: llm.RoleAssistant, Content: "Turn 1: Fixing bug"},
		{Role: llm.RoleUser, Content: "Turn 2: Run tests"},
		{Role: llm.RoleAssistant, Content: "Turn 2: Tests passed"},
		{Role: llm.RoleUser, Content: "Turn 3: Commit code"},
		{Role: llm.RoleAssistant, Content: "Turn 3: Code committed"},
	}

	tokens := session.EstimateMessagesTokens(msgs)
	if tokens <= 0 {
		t.Errorf("expected positive token count, got %d", tokens)
	}

	// Test NeedsCompaction (tokens is ~28, so limit 30 has threshold 22)
	if !session.NeedsCompaction(msgs, 30) {
		t.Errorf("expected NeedsCompaction to be true for small limit 30, tokens=%d", tokens)
	}
	if session.NeedsCompaction(msgs, 10000) {
		t.Errorf("expected NeedsCompaction to be false for large limit 10000")
	}

	// Test Compact: keepRecent = 2 (Turn 3 user & assistant)
	compacted := session.Compact(msgs, 2)
	// Should contain: SystemPrompt (0), CheckpointMessage (1), Turn 3 User, Turn 3 Assistant
	if len(compacted) != 4 {
		t.Fatalf("expected 4 compacted messages, got %d", len(compacted))
	}

	if compacted[0].Content != "System prompt" {
		t.Errorf("expected first message to be preserved system prompt")
	}
	if !strings.Contains(compacted[1].Content, "Older turns summarized") {
		t.Errorf("expected second message to be checkpoint summary, got: %s", compacted[1].Content)
	}
	if compacted[2].Content != "Turn 3: Commit code" {
		t.Errorf("expected recent message preserved, got: %s", compacted[2].Content)
	}
	if compacted[3].Content != "Turn 3: Code committed" {
		t.Errorf("expected recent message preserved, got: %s", compacted[3].Content)
	}
}
