package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectDocsDir_DocsExists(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	os.MkdirAll("docs", 0755)
	got := detectDocsDir()
	if got != "docs" {
		t.Errorf("detectDocsDir = %q, want 'docs'", got)
	}
}

func TestDetectDocsDir_DocExists(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	os.MkdirAll("doc", 0755)
	got := detectDocsDir()
	if got != "doc" {
		t.Errorf("detectDocsDir = %q, want 'doc'", got)
	}
}

func TestDetectDocsDir_CreatesDocs(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	got := detectDocsDir()
	if got != "docs" {
		t.Errorf("detectDocsDir = %q, want 'docs'", got)
	}
	if _, err := os.Stat("docs"); err != nil {
		t.Error("docs dir should have been created")
	}
}

func TestUpsertDVASection_NewFile(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "CLAUDE.md")
	os.WriteFile(file, []byte("# CLAUDE.md\n\nSome content\n"), 0644)

	err := upsertDVASection(file, "docs/dva-guide.md")
	if err != nil {
		t.Fatalf("upsertDVASection error: %v", err)
	}

	data, _ := os.ReadFile(file)
	content := string(data)
	if !strings.Contains(content, "## DVA (Docker Virtual Auto)") {
		t.Error("should append DVA section")
	}
	if !strings.Contains(content, "docs/dva-guide.md") {
		t.Error("should reference guide path")
	}
}

func TestUpsertDVASection_UpdateExisting(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "AGENTS.md")
	os.WriteFile(file, []byte("# AGENTS.md\n\n## DVA (Docker Virtual Auto)\n\nOld content\n"), 0644)

	err := upsertDVASection(file, "docs/dva-guide.md")
	if err != nil {
		t.Fatalf("upsertDVASection error: %v", err)
	}

	data, _ := os.ReadFile(file)
	content := string(data)
	if strings.Contains(content, "Old content") {
		t.Error("should have replaced old DVA section")
	}
	if !strings.Contains(content, "docs/dva-guide.md") {
		t.Error("should reference new guide path")
	}
}

func TestUpsertDVASection_NonExistentFile(t *testing.T) {
	err := upsertDVASection("/nonexistent/CLAUDE.md", "guide.md")
	if err != nil {
		t.Errorf("should return nil for non-existent file, got: %v", err)
	}
}

func TestReplaceDVASection_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "test.md")
	os.WriteFile(file, []byte("no dva section"), 0644)

	err := replaceDVASection(file, "no dva section", "new snippet")
	if err != nil {
		t.Fatalf("replaceDVASection error: %v", err)
	}

	data, _ := os.ReadFile(file)
	if string(data) != "no dva section" {
		t.Error("content should be unchanged when marker not found")
	}
}

func TestGenerateAIDocs(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	// Create CLAUDE.md so it gets updated
	os.WriteFile("CLAUDE.md", []byte("# CLAUDE.md\n\nProject info\n"), 0644)

	guidePath, err := generateAIDocs()
	if err != nil {
		t.Fatalf("generateAIDocs error: %v", err)
	}
	if guidePath == "" {
		t.Fatal("expected non-empty guide path")
	}

	// Verify guide file exists
	if _, err := os.Stat(guidePath); err != nil {
		t.Errorf("guide file not created at %s: %v", guidePath, err)
	}

	// Verify CLAUDE.md was updated
	data, _ := os.ReadFile("CLAUDE.md")
	if !strings.Contains(string(data), "DVA") {
		t.Error("CLAUDE.md should contain DVA section")
	}
}

func TestGenerateAIDocs_NoAgentFiles(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	// No CLAUDE.md or AGENTS.md
	guidePath, err := generateAIDocs()
	if err != nil {
		t.Fatalf("generateAIDocs error: %v", err)
	}
	if guidePath == "" {
		t.Fatal("expected non-empty guide path")
	}

	// Guide should still be created
	if _, err := os.Stat(guidePath); err != nil {
		t.Errorf("guide file not created: %v", err)
	}
}

func TestReplaceDVASection_WithFollowingSection(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "test.md")
	content := "# Top\n\n## DVA (Docker Virtual Auto)\n\nOld stuff\n\n## Other Section\n\nKeep this\n"
	os.WriteFile(file, []byte(content), 0644)

	newSnippet := "\n## DVA (Docker Virtual Auto)\n\nNew content\n"
	err := replaceDVASection(file, content, newSnippet)
	if err != nil {
		t.Fatalf("replaceDVASection error: %v", err)
	}

	data, _ := os.ReadFile(file)
	result := string(data)
	if !strings.Contains(result, "New content") {
		t.Error("should contain new content")
	}
	if !strings.Contains(result, "## Other Section") {
		t.Error("should preserve following section")
	}
	if !strings.Contains(result, "Keep this") {
		t.Error("should preserve content after DVA section")
	}
}
