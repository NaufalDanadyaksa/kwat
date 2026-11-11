package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize KWAT rules and configuration in the current directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		rulePath := "KWAT.md"
		if _, err := os.Stat(rulePath); err == nil {
			fmt.Printf("%s already exists.\n", rulePath)
		} else {
			sampleRules := `# Project Guidelines for KWAT

## Code Style & Architecture
- Maintain clean architecture and idiomatic patterns.
- Ensure all public functions and interfaces have clear comments.
- Always provide unit tests with high coverage for new features.

## Shell & Environment
- Run tests using 'go test -v ./...'
- Format code using 'go fmt ./...'
`
			if err := os.WriteFile(rulePath, []byte(sampleRules), 0644); err != nil {
				return fmt.Errorf("failed to create %s: %w", rulePath, err)
			}
			successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
			fmt.Println(successStyle.Render("✓ Created " + rulePath))
		}

		// Also check user home config
		home, err := os.UserHomeDir()
		if err == nil {
			kwatDir := filepath.Join(home, ".kwat")
			_ = os.MkdirAll(kwatDir, 0755)
			cfgFile := filepath.Join(kwatDir, "config.yaml")
			if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
				defaultConfigYAML := `# KWAT Global Configuration
default_provider: "anthropic"
default_model: "claude-3-5-sonnet-20241022"
permission_mode: "granular" # Options: granular, strict, sandbox, auto
max_context_tokens: 128000
auto_compact: true

providers:
  anthropic:
    model: "claude-3-5-sonnet-20241022"
  openai:
    model: "gpt-4o"
  gemini:
    model: "gemini-1.5-pro-latest"
  ollama:
    base_url: "http://localhost:11434"
    model: "llama3.1"
`
				_ = os.WriteFile(cfgFile, []byte(defaultConfigYAML), 0644)
				fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("✓ Created global config at " + cfgFile))
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

