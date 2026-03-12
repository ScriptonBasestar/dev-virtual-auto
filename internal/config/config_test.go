package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindConfigWalksUp(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	subDir := filepath.Join(projectDir, "src", "deep")
	os.MkdirAll(subDir, 0755)

	// Write dva.yml in project root
	dvaYml := filepath.Join(projectDir, "dva.yml")
	os.WriteFile(dvaYml, []byte("version: '0.1.0'\n"), 0644)

	// Find from deep subdir
	found, err := findConfig(subDir)
	if err != nil {
		t.Fatalf("findConfig(%s) error: %v", subDir, err)
	}
	if found != dvaYml {
		t.Errorf("findConfig(%s) = %s, want %s", subDir, found, dvaYml)
	}
}

func TestFindConfigNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := findConfig(tmpDir)
	if err == nil {
		t.Error("findConfig should fail when no dva.yml exists")
	}
}

func TestFindConfigDVAFILE(t *testing.T) {
	tmpDir := t.TempDir()
	customFile := filepath.Join(tmpDir, "custom.yml")
	os.WriteFile(customFile, []byte("version: '0.1.0'\n"), 0644)

	t.Setenv("DVA_FILE", customFile)

	found, err := findConfig(tmpDir)
	if err != nil {
		t.Fatalf("findConfig with DVA_FILE error: %v", err)
	}
	if found != customFile {
		t.Errorf("got %s, want %s", found, customFile)
	}
}

func TestLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	dvaYml := filepath.Join(tmpDir, "dva.yml")

	content := `version: "0.1.0"
compose:
  files:
    - docker-compose.yml
  project_name: myapp

environment:
  RAILS_ENV: development
  NODE_ENV: development

interaction:
  shell:
    description: "Open shell"
    service: app
    command: /bin/bash
  test:
    description: "Run tests"
    service: app
    command: bundle exec rspec
`
	os.WriteFile(dvaYml, []byte(content), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if cfg.Version != "0.1.0" {
		t.Errorf("version = %s, want 0.1.0", cfg.Version)
	}
	if cfg.Compose.ProjectName != "myapp" {
		t.Errorf("project_name = %s, want myapp", cfg.Compose.ProjectName)
	}
	if len(cfg.Compose.Files) != 1 || cfg.Compose.Files[0] != "docker-compose.yml" {
		t.Errorf("compose.files = %v, want [docker-compose.yml]", cfg.Compose.Files)
	}
	if len(cfg.Interaction) != 2 {
		t.Errorf("interaction count = %d, want 2", len(cfg.Interaction))
	}
	if cfg.Interaction["shell"].Command != "/bin/bash" {
		t.Errorf("shell command = %s, want /bin/bash", cfg.Interaction["shell"].Command)
	}
	if len(cfg.Environment) != 2 {
		t.Errorf("environment count = %d, want 2", len(cfg.Environment))
	}
}

func TestLoadConfigWithModules(t *testing.T) {
	tmpDir := t.TempDir()
	dvaDir := filepath.Join(tmpDir, ".dva")
	os.MkdirAll(dvaDir, 0755)

	// Main config with module reference
	os.WriteFile(filepath.Join(tmpDir, "dva.yml"), []byte(`
modules:
  - extra

interaction:
  shell:
    description: "Open shell"
    service: app
    command: /bin/bash
`), 0644)

	// Module file
	os.WriteFile(filepath.Join(dvaDir, "extra.yml"), []byte(`
interaction:
  test:
    description: "Run tests"
    service: app
    command: bundle exec rspec
`), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	// Should have both shell and test commands
	if len(cfg.Interaction) != 2 {
		t.Errorf("interaction count = %d, want 2 (merged module)", len(cfg.Interaction))
	}
	if _, ok := cfg.Interaction["test"]; !ok {
		t.Error("module command 'test' not found after merge")
	}
}

func TestLoadConfigWithOverride(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "dva.yml"), []byte(`
compose:
  project_name: original
interaction:
  shell:
    service: app
    command: /bin/bash
`), 0644)

	os.WriteFile(filepath.Join(tmpDir, "dva.override.yml"), []byte(`
compose:
  project_name: overridden
`), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if cfg.Compose.ProjectName != "overridden" {
		t.Errorf("project_name = %s, want overridden", cfg.Compose.ProjectName)
	}
}

func TestVersionCompatibility(t *testing.T) {
	tests := []struct {
		required   string
		compatible bool
	}{
		{"0.1.0", true},
		{"0.0.9", true},
		{"1.0.0", false},
	}

	for _, tt := range tests {
		if got := isVersionCompatible(tt.required); got != tt.compatible {
			t.Errorf("isVersionCompatible(%s) = %v, want %v", tt.required, got, tt.compatible)
		}
	}
}
