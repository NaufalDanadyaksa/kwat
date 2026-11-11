package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// GrepTool performs regex or substring search across project files.
type GrepTool struct {
	workDir string
}

func NewGrepTool(workDir string) *GrepTool {
	return &GrepTool{workDir: workDir}
}

func (t *GrepTool) Name() string {
	return "grep"
}

func (t *GrepTool) Description() string {
	return "Searches for a regular expression or text pattern across files in the workspace."
}

func (t *GrepTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "Regular expression or keyword to search for.",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Directory or file to search in (defaults to workspace root).",
			},
			"case_sensitive": map[string]any{
				"type":        "boolean",
				"description": "Whether the search is case-sensitive (default: true).",
			},
		},
		"required": []string{"pattern"},
	}
}

type grepInput struct {
	Pattern       string `json:"pattern"`
	Path          string `json:"path,omitempty"`
	CaseSensitive *bool  `json:"case_sensitive,omitempty"`
}

func (t *GrepTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var in grepInput
	if err := json.Unmarshal([]byte(argsJSON), &in); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	searchPattern := in.Pattern
	if in.CaseSensitive != nil && !*in.CaseSensitive {
		searchPattern = "(?i)" + searchPattern
	}

	re, err := regexp.Compile(searchPattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex pattern: %w", err)
	}

	searchRoot := in.Path
	if searchRoot == "" {
		searchRoot = t.workDir
	}
	if searchRoot == "" {
		searchRoot, _ = os.Getwd()
	}

	var matches []string
	maxMatches := 100

	err = filepath.WalkDir(searchRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}

		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "bin" || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip binary files and known large extensions
		ext := filepath.Ext(path)
		switch ext {
		case ".png", ".jpg", ".jpeg", ".gif", ".exe", ".dll", ".so", ".dylib", ".pdf", ".zip", ".tar", ".gz":
			return nil
		}

		relPath, _ := filepath.Rel(searchRoot, path)
		if relPath == "" {
			relPath = path
		}

		fileMatches, fErr := searchInFile(path, relPath, re, maxMatches-len(matches))
		if fErr == nil {
			matches = append(matches, fileMatches...)
		}

		if len(matches) >= maxMatches {
			return filepath.SkipAll
		}

		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return "", fmt.Errorf("grep walk error: %w", err)
	}

	if len(matches) == 0 {
		return fmt.Sprintf("No matches found for pattern %q", in.Pattern), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d match(es):\n", len(matches)))
	for _, m := range matches {
		sb.WriteString(m + "\n")
	}

	if len(matches) >= maxMatches {
		sb.WriteString(fmt.Sprintf("\n(Results truncated at %d matches)", maxMatches))
	}

	return strings.TrimRight(sb.String(), "\n"), nil
}

func searchInFile(fullPath, displayPath string, re *regexp.Regexp, limit int) ([]string, error) {
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	lineNum := 1

	for scanner.Scan() {
		text := scanner.Text()
		if re.MatchString(text) {
			lines = append(lines, fmt.Sprintf("%s:%d: %s", displayPath, lineNum, strings.TrimSpace(text)))
			if len(lines) >= limit {
				break
			}
		}
		lineNum++
	}

	return lines, scanner.Err()
}

