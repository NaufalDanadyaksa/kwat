package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// GlobTool searches for files matching a glob pattern.
type GlobTool struct {
	workDir string
}

func NewGlobTool(workDir string) *GlobTool {
	return &GlobTool{workDir: workDir}
}

func (t *GlobTool) Name() string {
	return "glob"
}

func (t *GlobTool) Description() string {
	return "Finds files matching a glob pattern (e.g. '**/*.go', 'internal/**/*.json')."
}

func (t *GlobTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "Glob pattern to match files against (e.g. '**/*.go').",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Base directory to search in (defaults to current workspace).",
			},
		},
		"required": []string{"pattern"},
	}
}

type globInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
}

func (t *GlobTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var in globInput
	if err := json.Unmarshal([]byte(argsJSON), &in); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	baseDir := in.Path
	if baseDir == "" {
		baseDir = t.workDir
	}
	if baseDir == "" {
		baseDir, _ = os.Getwd()
	}

	fsys := os.DirFS(baseDir)
	matches, err := doublestar.Glob(fsys, in.Pattern)
	if err != nil {
		return "", fmt.Errorf("glob pattern error: %w", err)
	}

	var filtered []string
	for _, m := range matches {
		// Ignore hidden directories and build outputs
		if strings.HasPrefix(m, ".git/") || strings.HasPrefix(m, "bin/") || strings.HasPrefix(m, "node_modules/") {
			continue
		}
		filtered = append(filtered, m)
	}

	sort.Strings(filtered)

	if len(filtered) == 0 {
		return fmt.Sprintf("No files matched pattern %q in %s", in.Pattern, baseDir), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d matching file(s):\n", len(filtered)))
	for _, f := range filtered {
		sb.WriteString(f + "\n")
	}

	return strings.TrimRight(sb.String(), "\n"), nil
}
