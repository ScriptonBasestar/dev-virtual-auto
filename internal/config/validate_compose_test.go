package config

import (
	"os"
	"path/filepath"
	"testing"
)

func makeComposeLifecycleConfig(tmpDir string, files []string, projectName string) *Config {
	cfg := &Config{
		filePath: filepath.Join(tmpDir, "dva.yml"),
		Stack: map[string]*LifecycleEntry{
			"compose": {
				Order: 10,
				Compose: &ComposePluginConfig{
					Files:       files,
					ProjectName: projectName,
				},
			},
		},
	}
	return cfg
}

func TestValidateComposeProjectNames_Missing(t *testing.T) {
	tmpDir := t.TempDir()

	composeFile := filepath.Join(tmpDir, "compose.yml")
	os.WriteFile(composeFile, []byte("services:\n  app:\n    image: nginx\n"), 0644)

	cfg := makeComposeLifecycleConfig(tmpDir, []string{"compose.yml"}, "myproject")

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

	cfg := makeComposeLifecycleConfig(tmpDir, []string{"compose.yml"}, "myproject")

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

	cfg := makeComposeLifecycleConfig(tmpDir, []string{"compose.yml"}, "myproject")

	warnings := cfg.ValidateComposeProjectNames()
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %d", len(warnings))
	}
}

func TestValidateComposeProjectNames_NoProjectName(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := makeComposeLifecycleConfig(tmpDir, []string{"compose.yml"}, "")

	warnings := cfg.ValidateComposeProjectNames()
	if len(warnings) != 0 {
		t.Errorf("expected no warnings when project_name is empty, got %d", len(warnings))
	}
}

func TestValidateComposeProjectNames_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "compose.yml"),
		[]byte("name: myproject\nservices:\n  app:\n    image: nginx\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "compose.tools.yml"),
		[]byte("services:\n  adminer:\n    image: adminer\n"), 0644)

	cfg := makeComposeLifecycleConfig(tmpDir, []string{"compose.yml", "compose.tools.yml"}, "myproject")

	warnings := cfg.ValidateComposeProjectNames()
	if len(warnings) != 0 {
		t.Errorf("expected no warnings (first file matches), got %d", len(warnings))
	}
}

func TestValidateComposeProjectNames_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := makeComposeLifecycleConfig(tmpDir, []string{"nonexistent.yml"}, "myproject")

	warnings := cfg.ValidateComposeProjectNames()
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for missing file, got %d", len(warnings))
	}
}

func TestFixComposeProjectName_InsertMissing(t *testing.T) {
	tmpDir := t.TempDir()

	composeFile := filepath.Join(tmpDir, "compose.yml")
	os.WriteFile(composeFile, []byte("services:\n  app:\n    image: nginx\n"), 0644)

	cfg := makeComposeLifecycleConfig(tmpDir, []string{"compose.yml"}, "myproject")

	w := ComposeNameWarning{File: "compose.yml", DvaName: "myproject"}
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

	cfg := makeComposeLifecycleConfig(tmpDir, []string{"compose.yml"}, "myproject")

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

	data, _ := os.ReadFile(composeFile)
	if contains(string(data), "old-name") {
		t.Error("old name still present after fix")
	}
}

// contains is defined in reserved_test.go
