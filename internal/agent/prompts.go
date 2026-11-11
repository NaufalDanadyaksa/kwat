package agent

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// BuildSystemPrompt generates the system prompt for the coding agent.
func BuildSystemPrompt(projectRules string, workDir string) string {
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	gitInfo := getGitContext(workDir)

	var sb strings.Builder

	sb.WriteString("You are KWAT, an elite, autonomous AI coding assistant running directly inside the user's terminal.\n\n")
	sb.WriteString("### ENVIRONMENT CONTEXT:\n")
	sb.WriteString(fmt.Sprintf("- OS / Platform: %s (%s)\n", runtime.GOOS, runtime.GOARCH))
	sb.WriteString(fmt.Sprintf("- Working Directory: %s\n", workDir))
	if gitInfo != "" {
		sb.WriteString(fmt.Sprintf("- Git Status:\n%s\n", gitInfo))
	}
	sb.WriteString("\n")

	sb.WriteString("### CORE PRINCIPLES & WORKFLOW:\n")
	sb.WriteString("1. Be concise, direct, and actionable. Avoid unnecessary pleasantries.\n")
	sb.WriteString("2. Explore first: When solving a coding task, discover the relevant files using grep and glob before modifying.\n")
	sb.WriteString("3. Precise file modification: When editing files, replace specific matching blocks accurately rather than rewriting entire files.\n")
	sb.WriteString("4. Shell execution: Run commands cleanly and interpret outputs or errors constructively.\n")
	sb.WriteString("5. Verify: Whenever feasible, run tests or checks after modifying code to verify changes.\n\n")

	if strings.TrimSpace(projectRules) != "" {
		sb.WriteString("### PROJECT-SPECIFIC GUIDELINES (from KWAT.md):\n")
		sb.WriteString(strings.TrimSpace(projectRules))
		sb.WriteString("\n\n")
	}

	return sb.String()
}

func getGitContext(dir string) string {
	cmd := exec.Command("git", "status", "--short", "--branch")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return "Clean git working tree"
	}
	return trimmed
}

