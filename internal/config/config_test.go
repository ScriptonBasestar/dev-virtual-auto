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

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input string
		want  [3]int
	}{
		{"1.2.3", [3]int{1, 2, 3}},
		{"0.1", [3]int{0, 1, 0}},
		{"v1.0.0", [3]int{1, 0, 0}},
		{"", [3]int{0, 0, 0}},
		{"invalid", [3]int{0, 0, 0}},
	}
	for _, tt := range tests {
		got := parseVersion(tt.input)
		if got != tt.want {
			t.Errorf("parseVersion(%s) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestProvisionConfigParsing(t *testing.T) {
	tmpDir := t.TempDir()
	dvaYml := filepath.Join(tmpDir, "dva.yml")

	content := `provision:
  default_profile: setup
  setup:
    - step: Install deps
      run: npm install
  reset:
    - step: Reset DB
      run: db reset
`
	os.WriteFile(dvaYml, []byte(content), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if cfg.Provision.DefaultProfile != "setup" {
		t.Errorf("default_profile = %q, want %q", cfg.Provision.DefaultProfile, "setup")
	}
	if len(cfg.Provision.Profiles) != 2 {
		t.Errorf("profiles count = %d, want 2", len(cfg.Provision.Profiles))
	}
	if _, ok := cfg.Provision.Profiles["setup"]; !ok {
		t.Error("profile 'setup' not found")
	}
	if _, ok := cfg.Provision.Profiles["reset"]; !ok {
		t.Error("profile 'reset' not found")
	}
}

func TestProvisionConfigWithoutDefaultProfile(t *testing.T) {
	tmpDir := t.TempDir()
	dvaYml := filepath.Join(tmpDir, "dva.yml")

	content := `provision:
  setup:
    - step: Install deps
      run: npm install
`
	os.WriteFile(dvaYml, []byte(content), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if cfg.Provision.DefaultProfile != "" {
		t.Errorf("default_profile = %q, want empty", cfg.Provision.DefaultProfile)
	}
	if len(cfg.Provision.Profiles) != 1 {
		t.Errorf("profiles count = %d, want 1", len(cfg.Provision.Profiles))
	}
}

func TestProvisionConfigMergeOverride(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "dva.yml"), []byte(`provision:
  setup:
    - step: Install
      run: npm install
  reset:
    - step: Reset
      run: db reset
`), 0644)

	os.WriteFile(filepath.Join(tmpDir, "dva.override.yml"), []byte(`provision:
  default_profile: reset
`), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if cfg.Provision.DefaultProfile != "reset" {
		t.Errorf("default_profile = %q, want %q after override", cfg.Provision.DefaultProfile, "reset")
	}
	// Original profiles should still be present
	if len(cfg.Provision.Profiles) != 2 {
		t.Errorf("profiles count = %d, want 2 (originals preserved)", len(cfg.Provision.Profiles))
	}
}

func TestServiceRelatedFieldParsing(t *testing.T) {
	tmpDir := t.TempDir()
	dvaYml := filepath.Join(tmpDir, "dva.yml")

	content := `compose:
  services:
    api:
      tags: [web]
      related: [worker, scheduler]
      hint: "Worker is needed for async processing"
    worker:
      tags: [background]
`
	os.WriteFile(dvaYml, []byte(content), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	api := cfg.Compose.Services["api"]
	if len(api.Related) != 2 {
		t.Fatalf("expected 2 related, got %d", len(api.Related))
	}
	if api.Related[0] != "worker" || api.Related[1] != "scheduler" {
		t.Errorf("related = %v, want [worker scheduler]", api.Related)
	}
	if api.Hint != "Worker is needed for async processing" {
		t.Errorf("hint = %q", api.Hint)
	}
}

func TestDoctorChecksParsing(t *testing.T) {
	tmpDir := t.TempDir()
	dvaYml := filepath.Join(tmpDir, "dva.yml")

	content := `checks:
  - name: Docker accessible
    type: docker_socket
    fix_hint: Start Docker
  - name: .env exists
    type: file_exists
    path: .env
    fix_hint: cp .env.example .env
  - name: Migrations applied
    type: command
    command: make migrate-status
    fix_hint: dva provision setup
`
	os.WriteFile(dvaYml, []byte(content), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if len(cfg.DoctorChecks) != 3 {
		t.Fatalf("expected 3 checks, got %d", len(cfg.DoctorChecks))
	}
	if cfg.DoctorChecks[0].Type != "docker_socket" {
		t.Errorf("check[0].type = %q, want docker_socket", cfg.DoctorChecks[0].Type)
	}
	if cfg.DoctorChecks[1].Path != ".env" {
		t.Errorf("check[1].path = %q, want .env", cfg.DoctorChecks[1].Path)
	}
	if cfg.DoctorChecks[2].Command != "make migrate-status" {
		t.Errorf("check[2].command = %q", cfg.DoctorChecks[2].Command)
	}
}

func TestModeProvisionField(t *testing.T) {
	tmpDir := t.TempDir()
	dvaYml := filepath.Join(tmpDir, "dva.yml")

	content := `modes:
  full-stack:
    description: "Everything"
    provision: setup
provision:
  setup:
    - step: Install deps
      run: npm install
`
	os.WriteFile(dvaYml, []byte(content), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	m, ok := cfg.Modes["full-stack"]
	if !ok {
		t.Fatal("mode full-stack not found")
	}
	if m.Provision != "setup" {
		t.Errorf("provision = %q, want setup", m.Provision)
	}
}

func TestEmptyConfig(t *testing.T) {
	tmpDir := t.TempDir()
	dvaYml := filepath.Join(tmpDir, "dva.yml")

	// Empty file
	os.WriteFile(dvaYml, []byte(""), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Empty config should load without error, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("Expected cfg to not be nil")
	}
}
