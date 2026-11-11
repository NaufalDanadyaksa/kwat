package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/md101/kwat/internal/llm"
	"github.com/md101/kwat/internal/tools"
)

func TestReadFileTool(t *testing.T) {
	tmpDir := t.TempDir()
	sampleFile := filepath.Join(tmpDir, "sample.txt")
	content := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\n"
	if err := os.WriteFile(sampleFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write sample file: %v", err)
	}

	tool := tools.NewReadFileTool(tmpDir)

	// Read full file
	out, err := tool.Execute(context.Background(), `{"path": "sample.txt"}`)
	if err != nil {
		t.Fatalf("unexpected error reading file: %v", err)
	}
	if !strings.Contains(out, "Line 1") || !strings.Contains(out, "Line 5") {
		t.Errorf("expected all lines, got:\n%s", out)
	}

	// Read line range 2 to 4
	outRange, err := tool.Execute(context.Background(), `{"path": "sample.txt", "start_line": 2, "end_line": 4}`)
	if err != nil {
		t.Fatalf("unexpected error reading range: %v", err)
	}
	if strings.Contains(outRange, "Line 1") {
		t.Errorf("did not expect Line 1 in range 2-4, got:\n%s", outRange)
	}
	if !strings.Contains(outRange, "Line 2") || !strings.Contains(outRange, "Line 4") {
		t.Errorf("expected Line 2 and Line 4, got:\n%s", outRange)
	}
}

func TestWriteFileTool(t *testing.T) {
	tmpDir := t.TempDir()
	tool := tools.NewWriteFileTool(tmpDir)

	args := `{"path": "nested/dir/test.txt", "content": "Hello KWAT!"}`
	out, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Successfully wrote") {
		t.Errorf("unexpected output: %s", out)
	}

	readBack, err := os.ReadFile(filepath.Join(tmpDir, "nested/dir/test.txt"))
	if err != nil {
		t.Fatalf("failed to read back created file: %v", err)
	}
	if string(readBack) != "Hello KWAT!" {
		t.Errorf("expected 'Hello KWAT!', got %q", string(readBack))
	}
}

func TestEditFileTool_ExactMatch(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "code.go")
	initialCode := `package main

func Add(a, b int) int {
    return a + b
}
`
	if err := os.WriteFile(filePath, []byte(initialCode), 0644); err != nil {
		t.Fatalf("failed to write initial code: %v", err)
	}

	tool := tools.NewEditFileTool(tmpDir)

	args := `{
		"path": "code.go",
		"old_str": "func Add(a, b int) int {\n    return a + b\n}",
		"new_str": "func Add(a, b int) int {\n    // Sum\n    return a + b\n}"
	}`

	out, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error editing file: %v", err)
	}
	if !strings.Contains(out, "Successfully edited") {
		t.Errorf("unexpected output: %s", out)
	}

	data, _ := os.ReadFile(filePath)
	if !strings.Contains(string(data), "// Sum") {
		t.Errorf("expected code to contain '// Sum', got:\n%s", string(data))
	}
}

func TestEditFileTool_MultipleMatchesError(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "duplicate.txt")
	content := "foo\nbar\nfoo\n"
	_ = os.WriteFile(filePath, []byte(content), 0644)

	tool := tools.NewEditFileTool(tmpDir)
	_, err := tool.Execute(context.Background(), `{"path": "duplicate.txt", "old_str": "foo", "new_str": "baz"}`)
	if err == nil {
		t.Errorf("expected error due to multiple matches, got nil")
	}
	if !strings.Contains(err.Error(), "matched 2 times") {
		t.Errorf("expected error message to mention 2 matches, got: %v", err)
	}
}

func TestEditFileTool_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "target.txt")
	_ = os.WriteFile(filePath, []byte("hello world"), 0644)

	tool := tools.NewEditFileTool(tmpDir)
	_, err := tool.Execute(context.Background(), `{"path": "target.txt", "old_str": "not_existing", "new_str": "replacement"}`)
	if err == nil {
		t.Errorf("expected error when old_str is not found, got nil")
	}
}

func TestGlobTool(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmpDir, "sub"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "root.go"), []byte(""), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "sub/nested.go"), []byte(""), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "notes.txt"), []byte(""), 0644)

	tool := tools.NewGlobTool(tmpDir)

	out, err := tool.Execute(context.Background(), `{"pattern": "**/*.go"}`)
	if err != nil {
		t.Fatalf("unexpected glob error: %v", err)
	}

	if !strings.Contains(out, "root.go") || !strings.Contains(out, "sub/nested.go") {
		t.Errorf("expected matching .go files, got:\n%s", out)
	}
	if strings.Contains(out, "notes.txt") {
		t.Errorf("notes.txt should not match **/*.go")
	}
}

func TestGrepTool(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "auth.go"), []byte("func AuthenticateUser() bool {\n    return true\n}"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "user.go"), []byte("type User struct {}\n"), 0644)

	tool := tools.NewGrepTool(tmpDir)

	out, err := tool.Execute(context.Background(), `{"pattern": "AuthenticateUser"}`)
	if err != nil {
		t.Fatalf("unexpected grep error: %v", err)
	}

	if !strings.Contains(out, "auth.go:1: func AuthenticateUser") {
		t.Errorf("expected line match in auth.go, got:\n%s", out)
	}
	if strings.Contains(out, "user.go") {
		t.Errorf("user.go should not match AuthenticateUser")
	}
}

func TestBashTool(t *testing.T) {
	tool := tools.NewBashTool("")

	out, err := tool.Execute(context.Background(), `{"command": "echo 'Hello Terminal'"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out, "Hello Terminal") {
		t.Errorf("expected output to contain 'Hello Terminal', got:\n%s", out)
	}
	if !strings.Contains(out, "Command exited with code 0") {
		t.Errorf("expected exit code 0, got:\n%s", out)
	}
}

func TestRegistry(t *testing.T) {
	reg := tools.DefaultRegistry("")
	defs := reg.GetDefinitions()

	if len(defs) < 6 {
		t.Errorf("expected at least 6 tools in default registry, got %d", len(defs))
	}

	// Test executing via registry
	res, err := reg.Execute(context.Background(), llm.ToolCall{
		ID:        "call_1",
		Name:      "bash",
		Arguments: `{"command": "echo registry_test"}`,
	})
	if err != nil {
		t.Fatalf("registry execution failed: %v", err)
	}
	if res.IsError {
		t.Errorf("expected successful tool result, got error: %s", res.Content)
	}
	if !strings.Contains(res.Content, "registry_test") {
		t.Errorf("expected output to contain 'registry_test', got %s", res.Content)
	}
}

