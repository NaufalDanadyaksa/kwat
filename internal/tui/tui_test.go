package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/md101/kwat/internal/agent"
	"github.com/md101/kwat/internal/config"
	"github.com/md101/kwat/internal/llm/mock"
	"github.com/md101/kwat/internal/permission"
	"github.com/md101/kwat/internal/tui"
)

func setupTestModel(t *testing.T) *tui.Model {
	cfg := config.DefaultConfig()
	mockProv := mock.NewMockProvider()
	perm := permission.NewManager(permission.ModeGranular, false)
	engine := agent.NewReActEngine(mockProv, "test-model", nil, perm, "sys")

	ag := &agent.Agent{
		Config:      cfg,
		Provider:    mockProv,
		PermManager: perm,
		Engine:      engine,
	}

	return tui.NewModel(ag, "KWAT.md", "Rule Content", "0.1.0-test", "testcommit")
}

func TestModelInitialization(t *testing.T) {
	m := setupTestModel(t)
	if m == nil {
		t.Fatalf("expected non-nil TUI model")
	}

	// Trigger WindowSizeMsg
	sizeMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updated, _ := m.Update(sizeMsg)
	newModel, ok := updated.(*tui.Model)
	if !ok {
		t.Fatalf("Update did not return *tui.Model")
	}

	view := newModel.View()
	if !strings.Contains(view, "KWAT") {
		t.Errorf("expected view to contain 'KWAT', got: %s", view)
	}
}

func TestFilterCommands(t *testing.T) {
	all := tui.FilterCommands("/")
	if len(all) != len(tui.AvailableCommands) {
		t.Errorf("expected %d commands for '/', got %d", len(tui.AvailableCommands), len(all))
	}

	modelMatches := tui.FilterCommands("/mod")
	if len(modelMatches) < 2 {
		t.Errorf("expected at least 2 matches for '/mod' (/model and /mode), got %d", len(modelMatches))
	}
}

func TestSlashCommands(t *testing.T) {
	m := setupTestModel(t)

	// /help
	handled, resp, quit := m.HandleSlashCommand("/help")
	if !handled || !strings.Contains(resp, "Available Slash Commands") || quit {
		t.Errorf("failed /help: handled=%v, quit=%v, resp=%s", handled, quit, resp)
	}

	// /version
	handled, resp, quit = m.HandleSlashCommand("/version")
	if !handled || !strings.Contains(resp, "0.1.0-test") {
		t.Errorf("failed /version: %s", resp)
	}

	// /rules
	handled, resp, quit = m.HandleSlashCommand("/rules")
	if !handled || !strings.Contains(resp, "Rule Content") {
		t.Errorf("failed /rules: %s", resp)
	}

	// /mode strict
	handled, resp, quit = m.HandleSlashCommand("/mode strict")
	if !handled || !strings.Contains(resp, "strict") {
		t.Errorf("failed /mode strict: %s", resp)
	}

	// /compact
	handled, resp, quit = m.HandleSlashCommand("/compact")
	if !handled || !strings.Contains(resp, "Compacted conversation history") {
		t.Errorf("failed /compact: %s", resp)
	}

	// /cost
	handled, resp, quit = m.HandleSlashCommand("/cost")
	if !handled || !strings.Contains(resp, "Total tokens used") {
		t.Errorf("failed /cost: %s", resp)
	}

	// /model (interactive transition)
	handled, resp, quit = m.HandleSlashCommand("/model")
	if !handled || !strings.Contains(resp, "Select a model") {
		t.Errorf("failed /model: %s", resp)
	}

	// /exit
	handled, _, quit = m.HandleSlashCommand("/exit")
	if !handled || !quit {
		t.Errorf("expected /exit to request quit")
	}
}
