package permission

import (
	"strings"
	"sync"
)

// Mode represents the permission security profile.
type Mode string

const (
	ModeGranular Mode = "granular" // Auto-approve read; confirm write & execute (default)
	ModeStrict   Mode = "strict"   // Confirm every tool call (including read)
	ModeSandbox  Mode = "sandbox"  // Dry-run mode: preview diffs and commands, no live execution
	ModeAuto     Mode = "auto"     // Auto-approve all operations without interactive prompt
)

// ParseMode parses a string into a valid Mode, defaulting to ModeGranular.
func ParseMode(s string) Mode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "strict":
		return ModeStrict
	case "sandbox", "dry-run":
		return ModeSandbox
	case "auto", "yolo", "unattended":
		return ModeAuto
	default:
		return ModeGranular
	}
}

// ActionType categorizes the sensitivity of a tool operation.
type ActionType string

const (
	ActionRead    ActionType = "read"
	ActionWrite   ActionType = "write"
	ActionExecute ActionType = "execute"
)

// Manager handles authorization checks and session whitelisting.
type Manager struct {
	mode            Mode
	alwaysAllowAll  bool
	whitelistMu     sync.RWMutex
	whitelistedKeys map[string]bool
}

// NewManager creates a new permission manager with the specified mode.
func NewManager(mode Mode, alwaysAllowAll bool) *Manager {
	return &Manager{
		mode:            mode,
		alwaysAllowAll:  alwaysAllowAll,
		whitelistedKeys: make(map[string]bool),
	}
}

// CurrentMode returns the configured mode.
func (m *Manager) CurrentMode() Mode {
	return m.mode
}

// RequiresConfirmation checks if a tool invocation needs explicit user approval.
func (m *Manager) RequiresConfirmation(action ActionType, toolName string, target string) bool {
	if m.alwaysAllowAll || m.mode == ModeAuto {
		return false
	}

	// Check if this specific tool/target was whitelisted for this session
	m.whitelistMu.RLock()
	key := toolName + ":" + target
	if m.whitelistedKeys[key] || m.whitelistedKeys[toolName+":*"] {
		m.whitelistMu.RUnlock()
		return false
	}
	m.whitelistMu.RUnlock()

	switch m.mode {
	case ModeStrict:
		return true
	case ModeSandbox:
		// In sandbox mode, read is safe; write and execute require preview/confirmation
		return action != ActionRead
	case ModeGranular:
		// Read operations are auto-approved; write and execute need user confirmation
		return action != ActionRead
	default:
		return true
	}
}

// Whitelist adds a tool target pattern to the session whitelist.
func (m *Manager) Whitelist(toolName, target string) {
	m.whitelistMu.Lock()
	defer m.whitelistMu.Unlock()
	key := toolName + ":" + target
	m.whitelistedKeys[key] = true
}

// WhitelistTool allows all invocations of a given tool for the rest of the session.
func (m *Manager) WhitelistTool(toolName string) {
	m.Whitelist(toolName, "*")
}

