package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateComposeProjectNames_Missing(t *testing.T) {
	tmpDir := t.TempDir()

	// compose file without name: key
	composeFile := filepath.Join(tmpDir, "compose.yml")
	os.WriteFile(composeFile, []byte("services:\n  app:\n    image: nginx\n"), 0644)

	cfg := &Config{
		filePath: filepath.Join(tmpDir, "dva.yml"),
		Compose: ComposeConfig{
			Files:       []string{"compose.yml"},
			ProjectName: "myproject",
		},
	}

	warnings := cfg.ValidateComposeProjectNames()
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	if warnings[0].ComposeName != "" {
		t.Errorf("expected empty ComposeName, got %q", warnings[0].ComposeName)
	}
	if warnings[0].DvaName != "myproject" {
		t.Errorf("expected DvaName=myproject, got %q", warnings[0].DvaName)
	}
}

func TestValidateComposeProjectNames_Mismatch(t *testing.T) {
	tmpDir := t.TempDir()

	composeFile := filepath.Join(tmpDir, "compose.yml")
	os.WriteFile(composeFile, []byte("name: wrong-name\nservices:\n  app:\n    image: nginx\n"), 0644)

	cfg := &Config{
		filePath: filepath.Join(tmpDir, "dva.yml"),
		Compose: ComposeConfig{
			Files:       []string{"compose.yml"},
			ProjectName: "myproject",
		},
	}

	warnings := cfg.ValidateComposeProjectNames()
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	if warnings[0].ComposeName != "wrong-name" {
		t.Errorf("expected ComposeName=wrong-name, got %q", warnings[0].ComposeName)
	}
}

func TestValidateComposeProjectNames_Match(t *testing.T) {
	tmpDir := t.TempDir()

	composeFile := filepath.Join(tmpDir, "compose.yml")
	os.WriteFile(composeFile, []byte("name: myproject\nservices:\n  app:\n    image: nginx\n"), 0644)

	cfg := &Config{
		filePath: filepath.Join(tmpDir, "dva.yml"),
		Compose: ComposeConfig{
			Files:       []string{"compose.yml"},
			ProjectName: "myproject",
		},
	}

	warnings := cfg.ValidateComposeProjectNames()
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %d", len(warnings))
	}
}

func TestValidateComposeProjectNames_NoProjectName(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		filePath: filepath.Join(tmpDir, "dva.yml"),
		Compose: ComposeConfig{
			Files: []string{"compose.yml"},
		},
	}

	warnings := cfg.ValidateComposeProjectNames()
	if len(warnings) != 0 {
		t.Errorf("expected no warnings when project_name is empty, got %d", len(warnings))
	}
}

func TestValidateComposeProjectNames_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Main file with correct name
	os.WriteFile(filepath.Join(tmpDir, "compose.yml"),
		[]byte("name: myproject\nservices:\n  app:\n    image: nginx\n"), 0644)
	// Supplementary file without name (normal pattern — should NOT warn)
	os.WriteFile(filepath.Join(tmpDir, "compose.tools.yml"),
		[]byte("services:\n  adminer:\n    image: adminer\n"), 0644)

	cfg := &Config{
		filePath: filepath.Join(tmpDir, "dva.yml"),
		Compose: ComposeConfig{
			Files:       []string{"compose.yml", "compose.tools.yml"},
			ProjectName: "myproject",
		},
	}

	warnings := cfg.ValidateComposeProjectNames()
	// Only the first compose file is checked — supplementary files are ignored
	if len(warnings) != 0 {
		t.Errorf("expected no warnings (first file matches), got %d", len(warnings))
	}
}

func TestValidateComposeProjectNames_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		filePath: filepath.Join(tmpDir, "dva.yml"),
		Compose: ComposeConfig{
			Files:       []string{"nonexistent.yml"},
			ProjectName: "myproject",
		},
	}

	// Should not crash on missing file
	warnings := cfg.ValidateComposeProjectNames()
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for missing file, got %d", len(warnings))
	}
}

func TestFixComposeProjectName_InsertMissing(t *testing.T) {
	tmpDir := t.TempDir()

	composeFile := filepath.Join(tmpDir, "compose.yml")
	os.WriteFile(composeFile, []byte("services:\n  app:\n    image: nginx\n"), 0644)

	cfg := &Config{
		filePath: filepath.Join(tmpDir, "dva.yml"),
		Compose: ComposeConfig{
			Files:       []string{"compose.yml"},
			ProjectName: "myproject",
		},
	}

	w := ComposeNameWarning{File: "compose.yml", DvaName: "myproject"}
	if err := cfg.FixComposeProjectName(w); err != nil {
		t.Fatalf("FixComposeProjectName error: %v", err)
	}

	// Verify the fix
	name, err := readComposeNameKey(composeFile)
	if err != nil {
		t.Fatalf("readComposeNameKey error: %v", err)
	}
	if name != "myproject" {
		t.Errorf("expected name=myproject after fix, got %q", name)
	}

	// Verify original content is preserved
	data, _ := os.ReadFile(composeFile)
	content := string(data)
	if !contains(content, "services:") {
		t.Error("original services: section lost after fix")
	}
}

func TestFixComposeProjectName_ReplaceMismatch(t *testing.T) {
	tmpDir := t.TempDir()

	composeFile := filepath.Join(tmpDir, "compose.yml")
	os.WriteFile(composeFile, []byte("name: old-name\n\nservices:\n  app:\n    image: nginx\n"), 0644)

	cfg := &Config{
		filePath: filepath.Join(tmpDir, "dva.yml"),
		Compose: ComposeConfig{
			Files:       []string{"compose.yml"},
			ProjectName: "myproject",
		},
	}

	w := ComposeNameWarning{File: "compose.yml", ComposeName: "old-name", DvaName: "myproject"}
	if err := cfg.FixComposeProjectName(w); err != nil {
		t.Fatalf("FixComposeProjectName error: %v", err)
	}

	name, err := readComposeNameKey(composeFile)
	if err != nil {
		t.Fatalf("readComposeNameKey error: %v", err)
	}
	if name != "myproject" {
		t.Errorf("expected name=myproject after fix, got %q", name)
	}

	// Verify old name is gone
	data, _ := os.ReadFile(composeFile)
	if contains(string(data), "old-name") {
		t.Error("old name still present after fix")
	}
}

// contains is defined in reserved_test.go
