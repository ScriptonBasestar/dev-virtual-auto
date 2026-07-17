package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSubprojects(t *testing.T) {
	parentDir := t.TempDir()

	subDir := filepath.Join(parentDir, "engine")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, FileName), []byte(`
version: "0.1.0"
stack:
  compose:
    default_runner: compose
    order: 10
    tags: [app]
    runners:
      compose:
        files:
          - docker-compose.yml
        project_name: engine
        services:
          postgres:
            tags: [infra]
          django-engine:
            tags: [app]
interaction:
  test:
    description: "Run tests"
    service: django-engine
    command: "pytest"
    tags: [test]
  shell:
    description: "Open shell"
    service: django-engine
    command: "bash"
`), 0644)

	subs := map[string]SubprojectConfig{
		"engine": {Path: "engine", ExcludeTags: []string{"infra"}},
	}

	result, err := LoadSubprojects(parentDir, subs)
	if err != nil {
		t.Fatalf("LoadSubprojects error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 subproject, got %d", len(result))
	}

	eng, ok := result["engine"]
	if !ok {
		t.Fatal("subproject 'engine' not found")
	}
	if eng.ComposeProjectName() != "engine" {
		t.Errorf("project_name = %s, want engine", eng.ComposeProjectName())
	}
	if len(eng.Interaction) != 2 {
		t.Errorf("interaction count = %d, want 2", len(eng.Interaction))
	}
}

func TestLoadSubprojectsMissing(t *testing.T) {
	parentDir := t.TempDir()

	subs := map[string]SubprojectConfig{
		"missing": {Path: "nonexistent"},
	}

	_, err := LoadSubprojects(parentDir, subs)
	if err == nil {
		t.Error("expected error for missing subproject, got nil")
	}
}

func TestLoadConfigWithUnimportedMissingSubproject(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, FileName), []byte(`
version: "0.1.0"
subprojects:
  pending:
    path: pending
`), 0644); err != nil {
		t.Fatalf("write parent config: %v", err)
	}

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if _, ok := cfg.Subprojects["pending"]; !ok {
		t.Fatal("subproject 'pending' not found")
	}
}

func TestLoadConfigWithEmptyImportMissingSubproject(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, FileName), []byte(`
version: "0.1.0"
subprojects:
  pending:
    path: pending
    import: {}
`), 0644); err != nil {
		t.Fatalf("write parent config: %v", err)
	}

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if _, ok := cfg.Subprojects["pending"]; !ok {
		t.Fatal("subproject 'pending' not found")
	}
}

func TestLoadConfigWithImportedMissingSubproject(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, FileName), []byte(`
version: "0.1.0"
subprojects:
  backend:
    path: backend
    import:
      interactions: [shell]
`), 0644); err != nil {
		t.Fatalf("write parent config: %v", err)
	}

	_, err := Load(tmpDir)
	if err == nil {
		t.Fatal("expected error for imported subproject without dva.yml")
	}
	if !strings.Contains(err.Error(), "backend/dva.yml") {
		t.Fatalf("error = %v, want missing backend/dva.yml", err)
	}
}

// TestLoadSubprojects_NestedModules_Fails locks TASK-043: a subproject module that
// declares modules: must fail with the same nested-modules rule as root Load, not
// silently drop the nested file and later report a misleading import miss.
func TestLoadSubprojects_NestedModules_Fails(t *testing.T) {
	// Given: parent imports an interaction that only exists in a nested module file
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub")
	modDir := filepath.Join(subDir, DotDirName)
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, FileName), []byte(`
version: "0.1.0"
subprojects:
  sub:
    path: sub
    import:
      interactions: [sub_from_nested]
`), 0o644); err != nil {
		t.Fatalf("write parent: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, FileName), []byte(`
version: "0.1.0"
modules:
  - submod
interaction:
  sub_from_module:
    command: echo HELLO_SUB_FROM_MODULE
`), 0o644); err != nil {
		t.Fatalf("write sub dva.yml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "submod.yml"), []byte(`
modules:
  - subnested
interaction:
  from_outer_module:
    command: echo OUTER
`), 0o644); err != nil {
		t.Fatalf("write submod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "subnested.yml"), []byte(`
interaction:
  sub_from_nested:
    command: echo HELLO_SUB_FROM_NESTED
`), 0o644); err != nil {
		t.Fatalf("write subnested: %v", err)
	}

	// When
	_, err := Load(tmpDir)

	// Then: accurate nested-modules error (not "interaction not found")
	if err == nil {
		t.Fatal("Load() error = nil, want nested modules rejection for subproject")
	}
	msg := err.Error()
	if !strings.Contains(msg, "nested modules are not supported") {
		t.Errorf("error = %q, want nested modules are not supported", msg)
	}
	if !strings.Contains(msg, "sub") {
		t.Errorf("error = %q, want subproject name sub", msg)
	}
	if strings.Contains(msg, "interaction") && strings.Contains(msg, "not found") {
		t.Errorf("error = %q, must not blame missing interaction", msg)
	}
}

func TestLoadConfigWithSubprojects(t *testing.T) {
	tmpDir := t.TempDir()

	subDir := filepath.Join(tmpDir, "sub-app")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, FileName), []byte(`
version: "0.1.0"
stack:
  compose:
    default_runner: compose
    order: 10
    runners:
      compose:
        files: [docker-compose.yml]
interaction:
  test:
    description: "Sub test"
    service: app
    command: "npm test"
`), 0644)

	os.WriteFile(filepath.Join(tmpDir, FileName), []byte(`
version: "0.1.0"
stack:
  compose:
    default_runner: compose
    order: 10
    tags: [infra]
    runners:
      compose:
        files: [docker-compose.yml]
subprojects:
  sub-app:
    path: sub-app
    exclude_tags: [infra]
interaction:
  shell:
    service: db
    command: "psql"
`), 0644)

	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if len(cfg.Subprojects) != 1 {
		t.Fatalf("expected 1 subproject, got %d", len(cfg.Subprojects))
	}

	sub, ok := cfg.Subprojects["sub-app"]
	if !ok {
		t.Fatal("subproject 'sub-app' not found")
	}
	if sub.Path != "sub-app" {
		t.Errorf("path = %s, want sub-app", sub.Path)
	}
	if len(sub.ExcludeTags) != 1 || sub.ExcludeTags[0] != "infra" {
		t.Errorf("exclude_tags = %v, want [infra]", sub.ExcludeTags)
	}

	subs, err := LoadSubprojects(cfg.FileDir(), cfg.Subprojects)
	if err != nil {
		t.Fatalf("LoadSubprojects error: %v", err)
	}
	if _, ok := subs["sub-app"]; !ok {
		t.Error("sub-app not loaded")
	}
}
