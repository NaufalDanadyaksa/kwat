package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadFileTool reads the content of a file, with optional line range support.
type ReadFileTool struct {
	workDir string
}

func NewReadFileTool(workDir string) *ReadFileTool {
	return &ReadFileTool{workDir: workDir}
}

func (t *ReadFileTool) Name() string {
	return "read_file"
}

func (t *ReadFileTool) Description() string {
	return "Reads content of a file. Supports optional start_line and end_line parameters (1-indexed)."
}

func (t *ReadFileTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "The path of the file to read (relative to workspace or absolute).",
			},
			"start_line": map[string]any{
				"type":        "integer",
				"description": "Optional 1-based start line to begin reading from (inclusive).",
			},
			"end_line": map[string]any{
				"type":        "integer",
				"description": "Optional 1-based end line to stop reading at (inclusive).",
			},
		},
		"required": []string{"path"},
	}
}

type readFileInput struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
}

func (t *ReadFileTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var in readFileInput
	if err := json.Unmarshal([]byte(argsJSON), &in); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	targetPath := in.Path
	if !filepath.IsAbs(targetPath) && t.workDir != "" {
		targetPath = filepath.Join(t.workDir, targetPath)
	}

	file, err := os.Open(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %w", in.Path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Allow scanning lines up to 1MB
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	var sb strings.Builder
	lineNum := 1

	for scanner.Scan() {
		if in.StartLine > 0 && lineNum < in.StartLine {
			lineNum++
			continue
		}
		if in.EndLine > 0 && lineNum > in.EndLine {
			break
		}

		fmt.Fprintf(&sb, "%4d | %s\n", lineNum, scanner.Text())
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading file %s: %w", in.Path, err)
	}

	if sb.Len() == 0 {
		return fmt.Sprintf("(File %s is empty or range has no lines)", in.Path), nil
	}

	return sb.String(), nil
}

