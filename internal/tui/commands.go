package tui

import (
	"fmt"
	"strings"

	"github.com/md101/kwat/internal/permission"
)

// CommandItem defines a registered slash command with metadata.
type CommandItem struct {
	Command     string
	Description string
	Category    string
}

// AvailableCommands lists all slash commands supported in KWAT REPL.
var AvailableCommands = []CommandItem{
	{Command: "/model", Description: "Switch AI model and provider (Claude, GPT-4o, Gemini, Ollama)", Category: "Config"},
	{Command: "/mode", Description: "Change execution security profile (granular, strict, sandbox, auto)", Category: "Config"},
	{Command: "/compact", Description: "Compact conversation history to prune tokens", Category: "Session"},
	{Command: "/cost", Description: "Display estimated token consumption and cost", Category: "Session"},
	{Command: "/session", Description: "Show current session ID and message count", Category: "Session"},
	{Command: "/rules", Description: "Inspect project instructions loaded from KWAT.md", Category: "Config"},
	{Command: "/clear", Description: "Clear chat viewport history", Category: "UI"},
	{Command: "/help", Description: "Display available commands and keybindings", Category: "General"},
	{Command: "/version", Description: "Show KWAT version, git commit, and build info", Category: "General"},
	{Command: "/exit", Description: "Exit KWAT session", Category: "General"},
}

// ModelPreset represents a selectable AI model option.
type ModelPreset struct {
	Provider    string
	Model       string
	DisplayName string
	Description string
}

// AvailableModels lists standard pre-configured models for selection.
var AvailableModels = []ModelPreset{
	{Provider: "anthropic", Model: "claude-3-5-sonnet-20241022", DisplayName: "Claude 3.5 Sonnet", Description: "Anthropic's flagship coding model (High Intelligence)"},
	{Provider: "anthropic", Model: "claude-3-5-haiku-20241022", DisplayName: "Claude 3.5 Haiku", Description: "Ultra-fast Anthropic model for snappy responses"},
	{Provider: "openai", Model: "gpt-4o", DisplayName: "OpenAI GPT-4o", Description: "Versatile, highly capable multimodal model"},
	{Provider: "openai", Model: "gpt-4o-mini", DisplayName: "OpenAI GPT-4o Mini", Description: "Fast, cost-efficient model for quick edits"},
	{Provider: "gemini", Model: "gemini-1.5-pro-latest", DisplayName: "Gemini 1.5 Pro", Description: "Google's 2M token context reasoning model"},
	{Provider: "gemini", Model: "gemini-1.5-flash-latest", DisplayName: "Gemini 1.5 Flash", Description: "High-speed Google model with 1M token context"},
	{Provider: "ollama", Model: "llama3.1", DisplayName: "Ollama: Llama 3.1", Description: "Local private model running via localhost:11434"},
	{Provider: "ollama", Model: "qwen2.5-coder", DisplayName: "Ollama: Qwen 2.5 Coder", Description: "Specialized open-weight coding model (local)"},
}

// ModePreset represents an available permission security setting.
type ModePreset struct {
	Mode        permission.Mode
	DisplayName string
	Description string
}

// AvailableModes lists all selectable security profiles.
var AvailableModes = []ModePreset{
	{Mode: permission.ModeGranular, DisplayName: "Granular (Default)", Description: "Auto-approve read/search; confirm write & shell commands"},
	{Mode: permission.ModeStrict, DisplayName: "Strict", Description: "Prompt user confirmation for every single tool invocation"},
	{Mode: permission.ModeSandbox, DisplayName: "Sandbox / Dry-run", Description: "Simulate file edits & commands safely without changes"},
	{Mode: permission.ModeAuto, DisplayName: "Auto / YOLO", Description: "Fully unattended execution without interactive confirmation"},
}

// FilterCommands returns commands that match a prefix string.
func FilterCommands(prefix string) []CommandItem {
	prefix = strings.TrimSpace(strings.ToLower(prefix))
	var matches []CommandItem
	for _, cmd := range AvailableCommands {
		if strings.HasPrefix(strings.ToLower(cmd.Command), prefix) {
			matches = append(matches, cmd)
		}
	}
	return matches
}

// HandleSlashCommand processes interactive slash commands entered by the user.
// Returns (handled bool, responseMsg string, shouldQuit bool).
func (m *Model) HandleSlashCommand(input string) (bool, string, bool) {
	trimmed := strings.TrimSpace(input)
	if !strings.HasPrefix(trimmed, "/") {
		return false, "", false
	}

	parts := strings.Fields(trimmed)
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/help":
		helpText := `Available Slash Commands:
  /model [name]       - Switch AI model & provider (interactive selector if empty)
  /mode [name]        - Change permission mode (interactive selector if empty)
  /compact            - Compact older message history into a summary checkpoint
  /cost               - Estimate session token usage and cost
  /session            - Show active session ID and message count
  /rules              - View active KWAT.md project rules
  /clear              - Clear the chat viewport history
  /version            - Show KWAT version information
  /exit, /quit        - Exit the KWAT REPL session

Keybindings:
  /                   - Trigger command autocomplete palette
  ↑ / ↓               - Navigate command suggestions or options
  Tab / Enter         - Select highlighted command or submit
  Ctrl+C              - Cancel current query or exit
  Esc                 - Dismiss menu / return to prompt`
		return true, helpText, false

	case "/model":
		if len(parts) >= 3 {
			// Manual switch e.g. /model openai gpt-4o
			providerName := parts[1]
			modelName := parts[2]
			if err := m.agent.SwitchModel(providerName, modelName); err != nil {
				return true, fmt.Sprintf("Error switching model: %v", err), false
			}
			return true, fmt.Sprintf("✓ Switched to %s (Provider: %s)", modelName, providerName), false
		}
		// Open interactive model selector modal
		m.state = stateModelSelect
		m.selectedModelIdx = 0
		return true, "Select a model using Arrow Keys and press Enter:", false

	case "/mode":
		if len(parts) >= 2 {
			newModeStr := parts[1]
			newMode := permission.ParseMode(newModeStr)
			m.agent.PermManager = permission.NewManager(newMode, newMode == permission.ModeAuto)
			m.agent.Config.PermissionMode = string(newMode)
			m.agent.Engine.PermManager = m.agent.PermManager
			return true, fmt.Sprintf("✓ Permission mode updated to: %s", newMode), false
		}
		// Open interactive mode selector modal
		m.state = stateModeSelect
		m.selectedModeIdx = 0
		return true, "Select a security mode using Arrow Keys and press Enter:", false

	case "/compact":
		before, after := m.agent.CompactSession()
		saved := before - after
		if saved < 0 {
			saved = 0
		}
		return true, fmt.Sprintf("✓ Compacted conversation history!\nTokens before: %d | Tokens after: %d\nTokens saved: ~%d", before, after, saved), false

	case "/cost":
		tokens := 0
		if m.agent.Session != nil {
			tokens = m.agent.Session.TotalTokens
		}
		return true, fmt.Sprintf("Session Token Estimation:\nTotal tokens used: ~%d\nEstimated cost: < $0.01", tokens), false

	case "/session":
		if m.agent.Session == nil {
			return true, "No active session.", false
		}
		return true, fmt.Sprintf("Active Session:\nID: %s\nProvider: %s\nModel: %s\nMessages: %d\nEstimated Tokens: ~%d",
			m.agent.Session.ID, m.agent.Session.Provider, m.agent.Session.Model, len(m.agent.Engine.History), m.agent.Session.TotalTokens), false

	case "/clear":
		m.history = nil
		m.currentStream.Reset()
		m.viewport.SetContent("")
		return true, "Chat history cleared.", false

	case "/rules":
		if m.rulesContent != "" {
			return true, fmt.Sprintf("Active Project Rules (%s):\n%s", m.rulesPath, m.rulesContent), false
		}
		return true, "No project-specific rule file found (KWAT.md). Run 'kwat init' to create one.", false

	case "/version":
		return true, fmt.Sprintf("KWAT CLI Agent v%s (commit: %s)", m.version, m.gitCommit), false

	case "/exit", "/quit":
		return true, "Exiting KWAT. Goodbye!", true

	default:
		return true, fmt.Sprintf("Unknown command: %s. Type '/help' or press '/' for commands.", cmd), false
	}
}
