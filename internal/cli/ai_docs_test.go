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

func TestDVAGuideUsesNamedPlanLifecycle(t *testing.T) {
	required := []string{
		"dva up PLAN",
		"dva stop PLAN",
		"dva down PLAN",
		"dva config validate",
		"dva up *",
		"Compose-less, non-standard, or multi-project layouts",
		"am run dva-discover",
		"am run dva-improve -p mode=rewrite",
	}
	for _, fragment := range required {
		if !strings.Contains(dvaGuideTemplate, fragment) {
			t.Errorf("guide is missing named-plan guidance %q", fragment)
		}
	}

	for _, stale := range []string{"dva init --ai", "dva init --prompt", "dva up -M"} {
		if strings.Contains(dvaGuideTemplate, stale) {
			t.Errorf("guide still teaches removed or migration-only command %q", stale)
		}
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
	if !strings.Contains(content, "## DVA (Dev Virtual Auto)") {
		t.Error("should append DVA section")
	}
	if !strings.Contains(content, "docs/dva-guide.md") {
		t.Error("should reference guide path")
	}
}

func TestUpsertDVASection_UpdateExisting(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "AGENTS.md")
	os.WriteFile(file, []byte("# AGENTS.md\n\n## DVA (Dev Virtual Auto)\n\nOld content\n"), 0644)

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

// A block owned by another generator opens with an HTML comment above its
// heading. Replacing the DVA section must not consume that opening marker and
// leave the closing one orphaned.
func TestReplaceDVASection_PreservesGeneratedBlockMarker(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "AGENTS.md")
	content := "# AGENTS.md\n\n## DVA (Dev Virtual Auto)\n\nOld stuff\n\n" +
		"<!-- skills:auto:start -->\n## AI Skills\n\n- **dva** — See `skills/dva/SKILL.md`.\n<!-- skills:auto:end -->\n"
	os.WriteFile(file, []byte(content), 0644)

	newSnippet := "\n## DVA (Dev Virtual Auto)\n\nNew content\n"
	if err := replaceDVASection(file, content, newSnippet); err != nil {
		t.Fatalf("replaceDVASection error: %v", err)
	}

	data, _ := os.ReadFile(file)
	result := string(data)
	if strings.Count(result, "<!-- skills:auto:start -->") != 1 {
		t.Errorf("opening marker must survive, got:\n%s", result)
	}
	if strings.Count(result, "<!-- skills:auto:end -->") != 1 {
		t.Errorf("closing marker must survive, got:\n%s", result)
	}
	if strings.Index(result, "<!-- skills:auto:start -->") > strings.Index(result, "## AI Skills") {
		t.Error("opening marker must stay above the heading it introduces")
	}
	if !strings.Contains(result, "New content") {
		t.Error("should contain new content")
	}
	if strings.Contains(result, "Old stuff") {
		t.Error("should have replaced the old DVA section")
	}
}

func TestReplaceDVASection_PreservesConsecutiveComments(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "AGENTS.md")
	content := "## DVA (Dev Virtual Auto)\n\nOld stuff\n\n" +
		"<!-- markdownlint-disable MD013 -->\n<!-- gen:start -->\n## Generated\n\nBody\n"
	os.WriteFile(file, []byte(content), 0644)

	if err := replaceDVASection(file, content, "\n## DVA (Dev Virtual Auto)\n\nNew\n"); err != nil {
		t.Fatalf("replaceDVASection error: %v", err)
	}

	data, _ := os.ReadFile(file)
	result := string(data)
	for _, want := range []string{"<!-- markdownlint-disable MD013 -->", "<!-- gen:start -->", "## Generated", "Body"} {
		if !strings.Contains(result, want) {
			t.Errorf("missing %q in:\n%s", want, result)
		}
	}
}

// Regenerating a file whose DVA section is already current and whose following
// heading carries no marker must not change a single byte.
func TestReplaceDVASection_MarkerFreeIsByteIdentical(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "AGENTS.md")
	snippet := "\n## DVA (Dev Virtual Auto)\n\nCurrent content\n"
	content := "# AGENTS.md\n\n## DVA (Dev Virtual Auto)\n\nCurrent content\n\n## Build & Test\n\nRun make build.\n"
	os.WriteFile(file, []byte(content), 0644)

	if err := replaceDVASection(file, content, snippet); err != nil {
		t.Fatalf("replaceDVASection error: %v", err)
	}

	data, _ := os.ReadFile(file)
	if string(data) != content {
		t.Errorf("marker-free content must be byte-identical\nwant:\n%q\ngot:\n%q", content, string(data))
	}
}

func TestReplaceDVASection_WithFollowingSection(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "test.md")
	content := "# Top\n\n## DVA (Dev Virtual Auto)\n\nOld stuff\n\n## Other Section\n\nKeep this\n"
	os.WriteFile(file, []byte(content), 0644)

	newSnippet := "\n## DVA (Dev Virtual Auto)\n\nNew content\n"
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
