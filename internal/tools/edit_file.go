package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EditFileTool modifies a file using search & replace blocks.
type EditFileTool struct {
	workDir string
}

func NewEditFileTool(workDir string) *EditFileTool {
	return &EditFileTool{workDir: workDir}
}

func (t *EditFileTool) Name() string {
	return "edit_file"
}

func (t *EditFileTool) Description() string {
	return "Replaces an exact block of text (old_str) in a file with new text (new_str). old_str must match uniquely."
}

func (t *EditFileTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the file to edit.",
			},
			"old_str": map[string]any{
				"type":        "string",
				"description": "Exact text block to replace. Must include enough surrounding context to match uniquely.",
			},
			"new_str": map[string]any{
				"type":        "string",
				"description": "The new text that will replace old_str.",
			},
		},
		"required": []string{"path", "old_str", "new_str"},
	}
}

type editFileInput struct {
	Path   string `json:"path"`
	OldStr string `json:"old_str"`
	NewStr string `json:"new_str"`
}

func (t *EditFileTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var in editFileInput
	if err := json.Unmarshal([]byte(argsJSON), &in); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	if in.OldStr == "" {
		return "", fmt.Errorf("old_str cannot be empty")
	}

	targetPath := in.Path
	if !filepath.IsAbs(targetPath) && t.workDir != "" {
		targetPath = filepath.Join(t.workDir, targetPath)
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", in.Path, err)
	}

	content := string(data)

	// Step 1: Check for exact match
	count := strings.Count(content, in.OldStr)
	if count == 1 {
		newContent := strings.Replace(content, in.OldStr, in.NewStr, 1)
		if err := os.WriteFile(targetPath, []byte(newContent), 0644); err != nil {
			return "", fmt.Errorf("failed to save edited file %s: %w", in.Path, err)
		}
		return fmt.Sprintf("Successfully edited %s (replaced 1 occurrence)", in.Path), nil
	}

	if count > 1 {
		return "", fmt.Errorf("target string (old_str) matched %d times in %s; please provide more surrounding context lines to match uniquely", count, in.Path)
	}

	// Step 2: Attempt fuzzy normalization (handling CRLF vs LF and trailing whitespace)
	normalizedContent, normalizedOld := normalizeLineEndings(content), normalizeLineEndings(in.OldStr)
	normCount := strings.Count(normalizedContent, normalizedOld)
	if normCount == 1 {
		newContent := strings.Replace(normalizedContent, normalizedOld, normalizeLineEndings(in.NewStr), 1)
		if err := os.WriteFile(targetPath, []byte(newContent), 0644); err != nil {
			return "", fmt.Errorf("failed to save edited file %s: %w", in.Path, err)
		}
		return fmt.Sprintf("Successfully edited %s (replaced via normalized line endings)", in.Path), nil
	}

	return "", fmt.Errorf("target string (old_str) not found in %s; please check the file with read_file first", in.Path)
}

func normalizeLineEndings(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.Join(lines, "\n")
}

