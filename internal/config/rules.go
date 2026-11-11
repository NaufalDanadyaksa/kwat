package config

import (
	"os"
	"path/filepath"
	"strings"
)

// ProjectRuleFiles lists the filenames checked for project-specific instructions in priority order.
var ProjectRuleFiles = []string{
	"KWAT.md",
	".kwat/rules",
	".kwat/KWAT.md",
	"RULES.md",
}

// LoadProjectRules checks the current project directory for custom instructions.
// Returns the rules as a string, or an empty string if no rule file is present.
func LoadProjectRules(workDir string) (string, string, error) {
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return "", "", err
		}
	}

	for _, filename := range ProjectRuleFiles {
		fullPath := filepath.Join(workDir, filename)
		data, err := os.ReadFile(fullPath)
		if err == nil {
			return strings.TrimSpace(string(data)), fullPath, nil
		}
	}

	return "", "", nil
}

