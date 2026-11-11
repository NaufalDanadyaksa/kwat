package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Palette
	colorPrimary   = lipgloss.Color("39")  // Cyan / Bright Blue
	colorSecondary = lipgloss.Color("208") // Orange / Amber
	colorAccent    = lipgloss.Color("141") // Purple / Violet
	colorSuccess   = lipgloss.Color("42")  // Green
	colorWarning   = lipgloss.Color("214") // Yellow
	colorDanger    = lipgloss.Color("196") // Red
	colorMuted     = lipgloss.Color("242") // Dark Gray
	colorSubtle    = lipgloss.Color("250") // Light Gray
	colorBgDark    = lipgloss.Color("236") // Dark Background
	colorBgCard    = lipgloss.Color("237") // Card Background
	colorHighlight = lipgloss.Color("63")  // Selection Highlight

	// Header & Pill Badges
	HeaderBrandStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255")).
				Background(colorPrimary).
				Padding(0, 1)

	BadgeModelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255")).
			Background(colorAccent).
			Bold(true).
			Padding(0, 1)

	BadgePermStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255")).
			Background(colorSecondary).
			Bold(true).
			Padding(0, 1)

	BadgeDirStyle = lipgloss.NewStyle().
			Foreground(colorSubtle).
			Background(colorBgDark).
			Padding(0, 1)

	SubHeaderStyle = lipgloss.NewStyle().
			Foreground(colorSubtle).
			Padding(0, 1)

	// Chat Bubbles & Labels
	UserCardStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(colorPrimary).
			Padding(0, 1).
			MarginTop(1).
			MarginBottom(1)

	UserLabelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary)

	AssistantLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorSuccess)

	ToolCardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorSecondary).
			Background(colorBgCard).
			Padding(0, 1).
			MarginTop(1).
			MarginBottom(1)

	ToolLabelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorSecondary)

	ToolOutputStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			PaddingLeft(2).
			MarginBottom(1)

	// Command Palette & Modal Styles
	PaletteBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Background(colorBgDark).
			Padding(0, 1).
			MarginTop(1)

	PaletteTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorPrimary).
				MarginBottom(0)

	PaletteItemStyle = lipgloss.NewStyle().
				Foreground(colorSubtle).
				Padding(0, 1)

	PaletteItemSelectedStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Color("255")).
					Background(colorHighlight).
					Padding(0, 1)

	PaletteDescStyle = lipgloss.NewStyle().
				Foreground(colorMuted)

	// Model / Mode Selection Modal
	ModalBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(colorAccent).
			Background(colorBgDark).
			Padding(1, 2).
			MarginTop(1)

	ModalTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent).
			MarginBottom(1)

	// Approval Dialog
	ApprovalBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.DoubleBorder()).
				BorderForeground(colorWarning).
				Background(colorBgDark).
				Padding(1, 2).
				Bold(true)

	// Status Bar & Hints
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(colorSubtle).
			Background(colorBgDark).
			Padding(0, 1)

	KeyBadgeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255")).
			Background(colorBgCard).
			Padding(0, 1)

	// Error & Notice
	ErrorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorDanger)

	SuccessStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorSuccess)
)
