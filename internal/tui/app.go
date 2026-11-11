package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/md101/kwat/internal/agent"
	"github.com/md101/kwat/internal/permission"
)

type state int

const (
	stateInput state = iota
	stateThinking
	stateApproval
	stateModelSelect
	stateModeSelect
)

type chatItem struct {
	role    string // "user", "assistant", "tool", "tool_output", "system", "error"
	content string
}

type approvalRequest struct {
	action   permission.ActionType
	toolName string
	target   string
	respChan chan approvalResponse
}

type approvalResponse struct {
	approved bool
	always   bool
}

// AgentEventMsg wraps an event emitted during an agent turn.
type AgentEventMsg struct {
	Event agent.TurnEvent
}

// TurnDoneMsg signals the completion of an agent turn.
type TurnDoneMsg struct {
	Result *agent.TurnResult
	Error  error
}

// Model represents the Bubble Tea TUI state.
type Model struct {
	agent              *agent.Agent
	state              state
	input              textinput.Model
	spinner            spinner.Model
	viewport           viewport.Model
	history            []chatItem
	currentStream      strings.Builder
	pendingReq         *approvalRequest
	cancelFunc         context.CancelFunc
	eventChan          chan agent.TurnEvent
	rulesPath          string
	rulesContent       string
	version            string
	gitCommit          string
	width              int
	height             int
	ready              bool
	selectedCommandIdx int
	selectedModelIdx   int
	selectedModeIdx    int
}

// NewModel creates an initialized TUI model.
func NewModel(ag *agent.Agent, rulesPath, rulesContent, version, commit string) *Model {
	ti := textinput.New()
	ti.Placeholder = "Type your query or press '/' for commands..."
	ti.Focus()
	ti.CharLimit = 4000
	ti.Width = 80

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorPrimary)

	m := &Model{
		agent:        ag,
		state:        stateInput,
		input:        ti,
		spinner:      sp,
		rulesPath:    rulesPath,
		rulesContent: rulesContent,
		version:      version,
		gitCommit:    commit,
		history:      make([]chatItem, 0),
	}

	// Wire interactive approval hook
	ag.SetApprover(func(action permission.ActionType, toolName string, target string) (bool, bool) {
		respChan := make(chan approvalResponse, 1)
		req := &approvalRequest{
			action:   action,
			toolName: toolName,
			target:   target,
			respChan: respChan,
		}
		m.pendingReq = req
		m.state = stateApproval
		resp := <-respChan
		m.state = stateThinking
		m.pendingReq = nil
		return resp.approved, resp.always
	})

	return m
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 3
		footerHeight := 5
		viewportHeight := msg.Height - headerHeight - footerHeight
		if viewportHeight < 5 {
			viewportHeight = 5
		}

		if !m.ready {
			m.viewport = viewport.New(msg.Width, viewportHeight)
			m.viewport.SetContent(m.renderHistory())
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = viewportHeight
		}

		m.input.Width = msg.Width - 6

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			if m.state == stateThinking && m.cancelFunc != nil {
				m.cancelFunc()
				m.state = stateInput
				m.appendHistory("system", "Cancelled turn.")
				return m, nil
			}
			return m, tea.Quit

		case tea.KeyEsc:
			if m.state == stateModelSelect || m.state == stateModeSelect {
				m.state = stateInput
				return m, nil
			}

		case tea.KeyUp:
			if m.state == stateModelSelect {
				if m.selectedModelIdx > 0 {
					m.selectedModelIdx--
				}
				return m, nil
			}
			if m.state == stateModeSelect {
				if m.selectedModeIdx > 0 {
					m.selectedModeIdx--
				}
				return m, nil
			}
			if m.state == stateInput && strings.HasPrefix(m.input.Value(), "/") {
				filtered := FilterCommands(m.input.Value())
				if len(filtered) > 0 {
					if m.selectedCommandIdx > 0 {
						m.selectedCommandIdx--
					}
					return m, nil
				}
			}

		case tea.KeyDown:
			if m.state == stateModelSelect {
				if m.selectedModelIdx < len(AvailableModels)-1 {
					m.selectedModelIdx++
				}
				return m, nil
			}
			if m.state == stateModeSelect {
				if m.selectedModeIdx < len(AvailableModes)-1 {
					m.selectedModeIdx++
				}
				return m, nil
			}
			if m.state == stateInput && strings.HasPrefix(m.input.Value(), "/") {
				filtered := FilterCommands(m.input.Value())
				if len(filtered) > 0 {
					if m.selectedCommandIdx < len(filtered)-1 {
						m.selectedCommandIdx++
					}
					return m, nil
				}
			}

		case tea.KeyTab:
			if m.state == stateInput && strings.HasPrefix(m.input.Value(), "/") {
				filtered := FilterCommands(m.input.Value())
				if len(filtered) > 0 {
					if m.selectedCommandIdx >= len(filtered) {
						m.selectedCommandIdx = 0
					}
					m.input.SetValue(filtered[m.selectedCommandIdx].Command)
					m.input.SetCursor(len(m.input.Value()))
					return m, nil
				}
			}

		case tea.KeyEnter:
			if m.state == stateModelSelect {
				if m.selectedModelIdx >= 0 && m.selectedModelIdx < len(AvailableModels) {
					sel := AvailableModels[m.selectedModelIdx]
					_ = m.agent.SwitchModel(sel.Provider, sel.Model)
					m.appendHistory("system", fmt.Sprintf("Switched model to %s (%s)", sel.DisplayName, sel.Provider))
					m.state = stateInput
					m.viewport.SetContent(m.renderHistory())
					m.viewport.GotoBottom()
				}
				return m, nil
			}

			if m.state == stateModeSelect {
				if m.selectedModeIdx >= 0 && m.selectedModeIdx < len(AvailableModes) {
					sel := AvailableModes[m.selectedModeIdx]
					m.agent.PermManager = permission.NewManager(sel.Mode, sel.Mode == permission.ModeAuto)
					m.agent.Config.PermissionMode = string(sel.Mode)
					m.agent.Engine.PermManager = m.agent.PermManager
					m.appendHistory("system", fmt.Sprintf("Updated security mode to: %s", sel.DisplayName))
					m.state = stateInput
					m.viewport.SetContent(m.renderHistory())
					m.viewport.GotoBottom()
				}
				return m, nil
			}

			if m.state == stateInput {
				val := strings.TrimSpace(m.input.Value())
				if val == "" {
					return m, nil
				}

				// If user was navigating suggestions and pressed Enter
				if strings.HasPrefix(val, "/") {
					filtered := FilterCommands(val)
					if len(filtered) > 0 && m.selectedCommandIdx < len(filtered) && val != filtered[m.selectedCommandIdx].Command {
						// Complete the command
						val = filtered[m.selectedCommandIdx].Command
					}
				}

				m.input.SetValue("")
				m.selectedCommandIdx = 0

				// Handle slash command
				if handled, resp, quit := m.HandleSlashCommand(val); handled {
					m.appendHistory("user", val)
					m.appendHistory("system", resp)
					if quit {
						return m, tea.Quit
					}
					m.viewport.SetContent(m.renderHistory())
					m.viewport.GotoBottom()
					return m, nil
				}

				// Normal prompt: submit to agent
				m.appendHistory("user", val)
				m.state = stateThinking
				m.currentStream.Reset()
				m.viewport.SetContent(m.renderHistory())
				m.viewport.GotoBottom()

				ctx, cancel := context.WithCancel(context.Background())
				m.cancelFunc = cancel
				m.eventChan = make(chan agent.TurnEvent, 64)

				cmds = append(cmds, m.spinner.Tick, m.startAgentTurn(ctx, val), m.waitForNextEvent())
				return m, tea.Batch(cmds...)
			}

		case tea.KeyRunes:
			if m.state == stateApproval && m.pendingReq != nil {
				r := strings.ToLower(string(msg.Runes))
				if r == "y" {
					m.pendingReq.respChan <- approvalResponse{approved: true, always: false}
					return m, nil
				} else if r == "a" {
					m.pendingReq.respChan <- approvalResponse{approved: true, always: true}
					return m, nil
				} else if r == "n" {
					m.pendingReq.respChan <- approvalResponse{approved: false, always: false}
					return m, nil
				}
			}
		}

	case AgentEventMsg:
		switch msg.Event.Type {
		case agent.EventTextDelta:
			m.currentStream.WriteString(msg.Event.Text)
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
		case agent.EventToolCall:
			toolCallInfo := fmt.Sprintf("⚙ %s(%s)", msg.Event.ToolCall.Name, msg.Event.ToolCall.Arguments)
			m.appendHistory("tool", toolCallInfo)
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
		case agent.EventToolResult:
			if msg.Event.ToolResult != nil {
				m.appendHistory("tool_output", msg.Event.ToolResult.Content)
				m.viewport.SetContent(m.renderHistory())
				m.viewport.GotoBottom()
			}
		case agent.EventError:
			m.appendHistory("error", fmt.Sprintf("Error: %v", msg.Event.Error))
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
		}

		if m.state == stateThinking {
			cmds = append(cmds, m.waitForNextEvent())
		}

	case TurnDoneMsg:
		m.state = stateInput
		if msg.Error != nil {
			m.appendHistory("error", fmt.Sprintf("Failed: %v", msg.Error))
		} else if m.currentStream.Len() > 0 {
			m.appendHistory("assistant", m.currentStream.String())
			m.currentStream.Reset()
		}
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()

	case spinner.TickMsg:
		if m.state == stateThinking {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	if m.state == stateInput {
		var inputCmd tea.Cmd
		m.input, inputCmd = m.input.Update(msg)
		cmds = append(cmds, inputCmd)
	}

	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

func (m *Model) startAgentTurn(ctx context.Context, prompt string) tea.Cmd {
	return func() tea.Msg {
		res, err := m.agent.Ask(ctx, prompt, m.eventChan)
		close(m.eventChan)
		return TurnDoneMsg{Result: res, Error: err}
	}
}

func (m *Model) waitForNextEvent() tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-m.eventChan
		if !ok {
			return nil
		}
		return AgentEventMsg{Event: ev}
	}
}

func (m *Model) appendHistory(role, content string) {
	m.history = append(m.history, chatItem{role: role, content: content})
}

func (m *Model) renderHistory() string {
	var sb strings.Builder

	if len(m.history) == 0 && m.currentStream.Len() == 0 {
		welcome := HeaderBrandStyle.Render("⚡ KWAT CLI Coding Agent") + "\n" +
			SubHeaderStyle.Render("Press '/' to see commands or type your coding question below.") + "\n"
		return welcome
	}

	for _, item := range m.history {
		switch item.role {
		case "user":
			sb.WriteString(UserCardStyle.Render(UserLabelStyle.Render("You ❯ ") + item.content) + "\n")
		case "assistant":
			sb.WriteString(AssistantLabelStyle.Render("KWAT ❯\n") + item.content + "\n\n")
		case "tool":
			sb.WriteString(ToolCardStyle.Render(item.content) + "\n")
		case "tool_output":
			sb.WriteString(ToolOutputStyle.Render("⮑ Output:\n"+truncateLines(item.content, 6)) + "\n\n")
		case "system":
			sb.WriteString(SubHeaderStyle.Render("ℹ "+item.content) + "\n\n")
		case "error":
			sb.WriteString(ErrorStyle.Render("✗ "+item.content) + "\n\n")
		}
	}

	if m.currentStream.Len() > 0 {
		sb.WriteString(AssistantLabelStyle.Render("KWAT ❯\n") + m.currentStream.String() + "\n")
	}

	return sb.String()
}

func truncateLines(s string, maxLines int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	return strings.Join(lines[:maxLines], "\n") + fmt.Sprintf("\n... (%d more lines)", len(lines)-maxLines)
}

func (m *Model) renderHeader() string {
	cwd, _ := os.Getwd()
	baseDir := filepath.Base(cwd)

	brand := HeaderBrandStyle.Render("⚡ KWAT")
	modelBadge := BadgeModelStyle.Render("🤖 " + m.agent.Config.DefaultModel)
	permBadge := BadgePermStyle.Render("🛡️ " + string(m.agent.PermManager.CurrentMode()))
	dirBadge := BadgeDirStyle.Render("📁 " + baseDir)

	return lipgloss.JoinHorizontal(lipgloss.Center, brand, " ", modelBadge, " ", permBadge, " ", dirBadge)
}

func (m *Model) renderCommandPalette() string {
	val := m.input.Value()
	filtered := FilterCommands(val)
	if len(filtered) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(PaletteTitleStyle.Render("Suggestions (Use ↑/↓, Tab/Enter to choose):") + "\n")

	limit := 5
	if len(filtered) < limit {
		limit = len(filtered)
	}

	for i := 0; i < limit; i++ {
		cmd := filtered[i]
		if i == m.selectedCommandIdx {
			item := fmt.Sprintf("❯ %-10s  %s", cmd.Command, cmd.Description)
			sb.WriteString(PaletteItemSelectedStyle.Render(item) + "\n")
		} else {
			item := fmt.Sprintf("  %-10s  %s", cmd.Command, cmd.Description)
			sb.WriteString(PaletteItemStyle.Render(item) + "\n")
		}
	}

	return PaletteBoxStyle.Render(strings.TrimRight(sb.String(), "\n"))
}

func (m *Model) renderModelSelector() string {
	var sb strings.Builder
	sb.WriteString(ModalTitleStyle.Render("🎯 Select AI Model (Arrow Keys + Enter)") + "\n\n")

	for i, p := range AvailableModels {
		activeMark := " "
		if p.Model == m.agent.Config.DefaultModel {
			activeMark = "●"
		}

		if i == m.selectedModelIdx {
			line := fmt.Sprintf("❯ [%s] %-24s (%s)", activeMark, p.DisplayName, p.Provider)
			sb.WriteString(PaletteItemSelectedStyle.Render(line) + "\n")
			sb.WriteString(PaletteDescStyle.Render("     "+p.Description) + "\n")
		} else {
			line := fmt.Sprintf("  [%s] %-24s (%s)", activeMark, p.DisplayName, p.Provider)
			sb.WriteString(PaletteItemStyle.Render(line) + "\n")
		}
	}

	sb.WriteString("\n" + PaletteDescStyle.Render("↑/↓: Navigate • Enter: Confirm • Esc: Cancel"))
	return ModalBoxStyle.Render(sb.String())
}

func (m *Model) renderModeSelector() string {
	var sb strings.Builder
	sb.WriteString(ModalTitleStyle.Render("🛡️ Select Security / Permission Mode (Arrow Keys + Enter)") + "\n\n")

	for i, p := range AvailableModes {
		activeMark := " "
		if p.Mode == m.agent.PermManager.CurrentMode() {
			activeMark = "●"
		}

		if i == m.selectedModeIdx {
			line := fmt.Sprintf("❯ [%s] %-22s", activeMark, p.DisplayName)
			sb.WriteString(PaletteItemSelectedStyle.Render(line) + "\n")
			sb.WriteString(PaletteDescStyle.Render("     "+p.Description) + "\n")
		} else {
			line := fmt.Sprintf("  [%s] %-22s", activeMark, p.DisplayName)
			sb.WriteString(PaletteItemStyle.Render(line) + "\n")
		}
	}

	sb.WriteString("\n" + PaletteDescStyle.Render("↑/↓: Navigate • Enter: Confirm • Esc: Cancel"))
	return ModalBoxStyle.Render(sb.String())
}

func (m *Model) View() string {
	if !m.ready {
		return "Initializing KWAT TUI..."
	}

	header := m.renderHeader()

	var bottomArea string
	switch m.state {
	case stateModelSelect:
		bottomArea = m.renderModelSelector()
	case stateModeSelect:
		bottomArea = m.renderModeSelector()
	case stateApproval:
		if m.pendingReq != nil {
			prompt := fmt.Sprintf("⚠️ Tool %s requires approval for action: %s\nTarget: %s\nAllow execution? [y]es / [n]o / [a]lways for session",
				m.pendingReq.toolName, m.pendingReq.action, m.pendingReq.target)
			bottomArea = ApprovalBoxStyle.Render(prompt)
		}
	case stateThinking:
		bottomArea = fmt.Sprintf("%s KWAT is thinking / executing...", m.spinner.View())
	default:
		palette := ""
		if strings.HasPrefix(m.input.Value(), "/") {
			palette = m.renderCommandPalette()
		}
		if palette != "" {
			bottomArea = palette + "\n" + m.input.View()
		} else {
			bottomArea = m.input.View()
		}
	}

	statusBar := StatusBarStyle.Render("Enter: Send/Select | /: Commands | Tab: Complete | ↑/↓: Navigate | Ctrl+C: Exit")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		m.viewport.View(),
		bottomArea,
		statusBar,
	)
}
