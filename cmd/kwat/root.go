package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/md101/kwat/internal/agent"
	"github.com/md101/kwat/internal/config"
	"github.com/md101/kwat/internal/permission"
	"github.com/md101/kwat/internal/tui"
)

var (
	cfgPath        string
	providerFlag   string
	modelFlag      string
	permissionFlag string
	resumeFlag     string
	yesFlag        bool
	verboseFlag    bool

	appConfig   *config.Config
	permManager *permission.Manager
)

var rootCmd = &cobra.Command{
	Use:   "kwat [prompt]",
	Short: "KWAT - Autonomous AI Coding Assistant in your terminal",
	Long: `KWAT is a fast, terminal-native AI coding assistant built in Go.
It helps you explore codebases, run shell commands safely, and edit files reliably.

Usage examples:
  kwat                           # Launch interactive REPL mode
  kwat "explain internal/config" # Single-shot query
  kwat --permission-mode strict  # Enforce strict manual approvals
  kwat init                      # Scaffold KWAT.md and default config`,
	Args: cobra.ArbitraryArgs,
	SilenceUsage: true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		appConfig, err = config.LoadConfig(cfgPath)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Flag overrides
		if providerFlag != "" {
			appConfig.DefaultProvider = providerFlag
		}
		if modelFlag != "" {
			appConfig.DefaultModel = modelFlag
		}
		if permissionFlag != "" {
			appConfig.PermissionMode = permissionFlag
		}
		if yesFlag {
			appConfig.PermissionMode = "auto"
		}

		permMode := permission.ParseMode(appConfig.PermissionMode)
		permManager = permission.NewManager(permMode, yesFlag)

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		bannerStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")).
			Padding(0, 1)

		infoStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))

		rules, rulePath, _ := config.LoadProjectRules("")

		// If a prompt is provided via args, handle single-shot execution
		if len(args) > 0 {
			prompt := strings.Join(args, " ")
			fmt.Println(bannerStyle.Render("⚡ KWAT Single-shot Mode"))
			fmt.Printf("Model: %s/%s | Permissions: %s\n", appConfig.DefaultProvider, appConfig.DefaultModel, appConfig.PermissionMode)
			if rulePath != "" {
				fmt.Println(infoStyle.Render("Loaded rules from: " + rulePath))
			}
			fmt.Println()

			ag, err := agent.NewAgent(appConfig, permManager, nil, rules)
			if err != nil {
				return fmt.Errorf("failed to initialize agent: %w", err)
			}

			if resumeFlag != "" {
				if err := ag.ResumeSession(resumeFlag); err != nil {
					return fmt.Errorf("failed to resume session: %w", err)
				}
				fmt.Printf("✓ Resumed session %s (%d messages)\n", ag.Session.ID, len(ag.Engine.History))
			}

			// Interactive approval handler
			ag.SetApprover(func(action permission.ActionType, toolName string, target string) (bool, bool) {
				promptStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
				fmt.Printf("\n%s Allow %s action '%s'? [y/N/a(lways)]: ",
					promptStyle.Render("? [Permission Required]"), action, toolName)
				var resp string
				_, _ = fmt.Scanln(&resp)
				resp = strings.ToLower(strings.TrimSpace(resp))
				if resp == "a" || resp == "always" {
					return true, true
				}
				return resp == "y" || resp == "yes", false
			})

			eventChan := make(chan agent.TurnEvent, 64)
			doneChan := make(chan struct{})

			go func() {
				for ev := range eventChan {
					switch ev.Type {
					case agent.EventTextDelta:
						fmt.Print(ev.Text)
					case agent.EventToolCall:
						toolStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Bold(true)
						fmt.Printf("\n%s %s(%s)\n", toolStyle.Render("⚙ [Tool Call]"), ev.ToolCall.Name, ev.ToolCall.Arguments)
					case agent.EventToolResult:
						var outStyle lipgloss.Style
						if ev.ToolResult.IsError {
							outStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
						} else {
							outStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
						}
						fmt.Println(outStyle.Render(fmt.Sprintf("⮑ Output: %s", ev.ToolResult.Content)))
					case agent.EventError:
						errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
						fmt.Printf("\n%s %v\n", errStyle.Render("✗ Error:"), ev.Error)
					}
				}
				close(doneChan)
			}()

			ctx := context.Background()
			_, err = ag.Ask(ctx, prompt, eventChan)
			close(eventChan)
			<-doneChan

			if err != nil {
				// Provide helpful hint if API key is likely missing
				if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "key") || strings.Contains(err.Error(), "auth") {
					fmt.Printf("\nTip: Make sure your %s API key is set in environment or ~/.kwat/config.yaml\n", strings.ToUpper(appConfig.DefaultProvider))
				}
				return err
			}
			fmt.Println()
			return nil
		}

		// Otherwise, start interactive chat / REPL mode with Bubble Tea
		ag, err := agent.NewAgent(appConfig, permManager, nil, rules)
		if err != nil {
			return fmt.Errorf("failed to initialize agent: %w", err)
		}

		if resumeFlag != "" {
			if err := ag.ResumeSession(resumeFlag); err != nil {
				return fmt.Errorf("failed to resume session: %w", err)
			}
		}

		tuiModel := tui.NewModel(ag, rulePath, rules, Version, GitCommit)
		p := tea.NewProgram(tuiModel, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("error running TUI: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgPath, "config", "c", "", "path to custom config file")
	rootCmd.PersistentFlags().StringVarP(&providerFlag, "provider", "p", "", "LLM provider (anthropic, openai, gemini, ollama)")
	rootCmd.PersistentFlags().StringVarP(&modelFlag, "model", "m", "", "model identifier")
	rootCmd.PersistentFlags().StringVarP(&resumeFlag, "resume", "r", "", "resume previous session (use 'latest' or session ID)")
	rootCmd.PersistentFlags().StringVar(&permissionFlag, "permission-mode", "", "permission mode (granular, strict, sandbox, auto)")
	rootCmd.PersistentFlags().BoolVarP(&yesFlag, "yes", "y", false, "auto-approve all tool operations without confirmation")
	rootCmd.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "enable debug and verbose output")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
