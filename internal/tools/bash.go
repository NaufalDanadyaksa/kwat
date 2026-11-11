package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// BashTool executes shell commands in the current workspace.
type BashTool struct {
	workDir string
}

func NewBashTool(workDir string) *BashTool {
	return &BashTool{workDir: workDir}
}

func (t *BashTool) Name() string {
	return "bash"
}

func (t *BashTool) Description() string {
	return "Executes a shell command in the workspace directory with timeout and output capture."
}

func (t *BashTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The exact shell command line string to execute.",
			},
			"timeout_seconds": map[string]any{
				"type":        "integer",
				"description": "Optional timeout in seconds (default: 30, max: 180).",
			},
		},
		"required": []string{"command"},
	}
}

type bashInput struct {
	Command        string `json:"command"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

func (t *BashTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var in bashInput
	if err := json.Unmarshal([]byte(argsJSON), &in); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	timeout := 30 * time.Second
	if in.TimeoutSeconds > 0 {
		if in.TimeoutSeconds > 180 {
			in.TimeoutSeconds = 180
		}
		timeout = time.Duration(in.TimeoutSeconds) * time.Second
	}

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "sh", "-c", in.Command)
	if t.workDir != "" {
		cmd.Dir = t.workDir
	}

	var combinedBuf bytes.Buffer
	cmd.Stdout = &combinedBuf
	cmd.Stderr = &combinedBuf

	err := cmd.Run()

	output := combinedBuf.String()
	// Limit output to 64KB
	maxLen := 64 * 1024
	truncated := false
	if len(output) > maxLen {
		output = output[:maxLen]
		truncated = true
	}

	var sb strings.Builder
	if cmdCtx.Err() == context.DeadlineExceeded {
		sb.WriteString(fmt.Sprintf("Command timed out after %v\n", timeout))
	} else if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			sb.WriteString(fmt.Sprintf("Command exited with code %d\n", exitErr.ExitCode()))
		} else {
			sb.WriteString(fmt.Sprintf("Command failed: %v\n", err))
		}
	} else {
		sb.WriteString("Command exited with code 0\n")
	}

	if output != "" {
		sb.WriteString("Output:\n" + output)
		if truncated {
			sb.WriteString("\n[Output truncated due to size limit]")
		}
	} else {
		sb.WriteString("(No output)")
	}

	return strings.TrimRight(sb.String(), "\n"), nil
}

