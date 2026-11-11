package permission

import (
	"testing"
)

func TestParseMode(t *testing.T) {
	tests := []struct {
		input    string
		expected Mode
	}{
		{"granular", ModeGranular},
		{"strict", ModeStrict},
		{"sandbox", ModeSandbox},
		{"dry-run", ModeSandbox},
		{"auto", ModeAuto},
		{"yolo", ModeAuto},
		{"unknown", ModeGranular},
		{"", ModeGranular},
	}

	for _, tt := range tests {
		got := ParseMode(tt.input)
		if got != tt.expected {
			t.Errorf("ParseMode(%q) = %v; want %v", tt.input, got, tt.expected)
		}
	}
}

func TestManagerConfirmationRules(t *testing.T) {
	// Granular mode
	gm := NewManager(ModeGranular, false)
	if gm.RequiresConfirmation(ActionRead, "read_file", "main.go") {
		t.Errorf("Granular mode should not require confirmation for Read")
	}
	if !gm.RequiresConfirmation(ActionWrite, "edit_file", "main.go") {
		t.Errorf("Granular mode must require confirmation for Write")
	}
	if !gm.RequiresConfirmation(ActionExecute, "bash", "go test ./...") {
		t.Errorf("Granular mode must require confirmation for Execute")
	}

	// Whitelist in granular mode
	gm.Whitelist("bash", "go test ./...")
	if gm.RequiresConfirmation(ActionExecute, "bash", "go test ./...") {
		t.Errorf("Whitelisted command should not require confirmation")
	}
	// Different target still requires confirmation
	if !gm.RequiresConfirmation(ActionExecute, "bash", "rm -rf /") {
		t.Errorf("Non-whitelisted command still requires confirmation")
	}

	// Tool wildcard whitelist
	gm.WhitelistTool("edit_file")
	if gm.RequiresConfirmation(ActionWrite, "edit_file", "anything.go") {
		t.Errorf("Wildcard whitelisted tool should not require confirmation")
	}

	// Strict mode
	sm := NewManager(ModeStrict, false)
	if !sm.RequiresConfirmation(ActionRead, "read_file", "main.go") {
		t.Errorf("Strict mode must require confirmation for Read")
	}
	if !sm.RequiresConfirmation(ActionWrite, "edit_file", "main.go") {
		t.Errorf("Strict mode must require confirmation for Write")
	}

	// Auto mode
	am := NewManager(ModeAuto, false)
	if am.RequiresConfirmation(ActionExecute, "bash", "rm -rf tmp") {
		t.Errorf("Auto mode must never require confirmation")
	}

	// Always allow flag override
	om := NewManager(ModeStrict, true)
	if om.RequiresConfirmation(ActionExecute, "bash", "dangerous_command") {
		t.Errorf("alwaysAllowAll must bypass confirmation even in Strict mode")
	}
}

