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
lifecycle:
  compose:
    plugin: compose
    order: 10
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
	if cfg.ComposeProjectName() != "myapp" {
		t.Errorf("project_name = %s, want myapp", cfg.ComposeProjectName())
	}
	files := cfg.AllComposeFiles()
	if len(files) != 1 || files[0] != "docker-compose.yml" {
		t.Errorf("compose.files = %v, want [docker-compose.yml]", files)
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
	dvaDir := filepath.Join(tmpDir, DotDirName)
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
lifecycle:
  compose:
    plugin: compose
    order: 10
    project_name: original
interaction:
  shell:
    service: app
    command: /bin/bash
`), 0644)

	os.WriteFile(filepath.Join(tmpDir, "dva.override.yml"), []byte(`
lifecycle:
  compose-override:
    plugin: compose
    order: 10
    project_name: overridden
`), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if cfg.ComposeProjectName() != "original" {
		t.Errorf("project_name = %s, want original (first compose entry)", cfg.ComposeProjectName())
	}
	if len(cfg.Lifecycle) != 2 {
		t.Errorf("lifecycle entries = %d, want 2 (merged)", len(cfg.Lifecycle))
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

	content := `lifecycle:
  compose:
    plugin: compose
    order: 10
    files: [docker-compose.yml]
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

	services := cfg.ComposeServices()
	api := services["api"]
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

func TestEndpointsParsing(t *testing.T) {
	tmpDir := t.TempDir()
	dvaYml := filepath.Join(tmpDir, "dva.yml")

	content := `endpoints:
  api:
    url: http://localhost:8080
    label: "API Server"
    tags: [app]
    paths:
      /health: "Health check"
      /api/v1: "REST API"
  git-ssh:
    url: ssh://git@localhost:2222
    label: "Git SSH"
    tags: [app, scm]
`
	os.WriteFile(dvaYml, []byte(content), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if len(cfg.Endpoints) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(cfg.Endpoints))
	}

	api := cfg.Endpoints["api"]
	if api.URL != "http://localhost:8080" {
		t.Errorf("api.URL = %q", api.URL)
	}
	if api.Label != "API Server" {
		t.Errorf("api.Label = %q", api.Label)
	}
	if len(api.Tags) != 1 || api.Tags[0] != "app" {
		t.Errorf("api.Tags = %v", api.Tags)
	}
	if len(api.Paths) != 2 {
		t.Errorf("api.Paths count = %d, want 2", len(api.Paths))
	}
	if api.Paths["/health"] != "Health check" {
		t.Errorf("api.Paths[/health] = %q", api.Paths["/health"])
	}

	ssh := cfg.Endpoints["git-ssh"]
	if ssh.URL != "ssh://git@localhost:2222" {
		t.Errorf("ssh.URL = %q", ssh.URL)
	}
	if len(ssh.Tags) != 2 {
		t.Errorf("ssh.Tags = %v", ssh.Tags)
	}
}

func TestEndpointsMergeOverride(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "dva.yml"), []byte(`endpoints:
  api:
    url: http://localhost:8080
    label: "API"
  db:
    url: localhost:5432
    label: "DB"
`), 0644)

	os.WriteFile(filepath.Join(tmpDir, "dva.override.yml"), []byte(`endpoints:
  api:
    url: http://localhost:9090
    label: "API Override"
  admin:
    url: http://localhost:3000
    label: "Admin"
`), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if len(cfg.Endpoints) != 3 {
		t.Fatalf("expected 3 endpoints after merge, got %d", len(cfg.Endpoints))
	}
	if cfg.Endpoints["api"].URL != "http://localhost:9090" {
		t.Errorf("api.URL = %q, want override", cfg.Endpoints["api"].URL)
	}
	if cfg.Endpoints["db"].URL != "localhost:5432" {
		t.Errorf("db should be preserved from base")
	}
	if cfg.Endpoints["admin"].URL != "http://localhost:3000" {
		t.Errorf("admin should be added from override")
	}
}

func TestModeEndpointTags(t *testing.T) {
	tmpDir := t.TempDir()
	dvaYml := filepath.Join(tmpDir, "dva.yml")

	content := `modes:
  dev:
    description: "Dev mode"
    endpoint_tags: [app, monitoring]
`
	os.WriteFile(dvaYml, []byte(content), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	m := cfg.Modes["dev"]
	if len(m.EndpointTags) != 2 {
		t.Fatalf("expected 2 endpoint_tags, got %d", len(m.EndpointTags))
	}
	if m.EndpointTags[0] != "app" || m.EndpointTags[1] != "monitoring" {
		t.Errorf("endpoint_tags = %v", m.EndpointTags)
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
