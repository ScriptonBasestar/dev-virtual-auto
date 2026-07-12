package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateAcceptsComposeRunnerSchema(t *testing.T) {
	tmpDir := t.TempDir()
	content := `version: "0.1.44"
stack:
  core-compose:
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
        project_name: app
plans:
  local-dev:
    entries:
      - name: core-compose
        runner: compose
        services: [postgres]
`
	cfg := loadConfigForSchemaTest(t, tmpDir, content)

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsLegacyNestedComposeSchema(t *testing.T) {
	tmpDir := t.TempDir()
	content := `version: "0.1.44"
stack:
  core-compose:
    compose:
      files: [compose.yml]
`
	cfg := loadConfigForSchemaTest(t, tmpDir, content)

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for legacy stack.<entry>.compose")
	}
	if !strings.Contains(err.Error(), "Must not validate the schema") {
		t.Fatalf("Validate() error = %v, want schema rejection", err)
	}
}

func TestValidateRejectsLegacyFlatComposePluginSchema(t *testing.T) {
	tmpDir := t.TempDir()
	content := `version: "0.1.44"
stack:
  core-compose:
    plugin: compose
    files: [compose.yml]
`
	cfg := loadConfigForSchemaTest(t, tmpDir, content)

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for legacy plugin: compose")
	}
	if !strings.Contains(err.Error(), "Must not validate the schema") {
		t.Fatalf("Validate() error = %v, want schema rejection", err)
	}
}

func TestValidateRejectsLegacyAutoInferredComposeSchema(t *testing.T) {
	tmpDir := t.TempDir()
	content := `version: "0.1.44"
stack:
  compose:
    command: /bin/echo
`
	cfg := loadConfigForSchemaTest(t, tmpDir, content)

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for legacy auto-inferred compose")
	}
	if !strings.Contains(err.Error(), "Must not validate the schema") {
		t.Fatalf("Validate() error = %v, want schema rejection", err)
	}
}

func TestValidateRejectsComposeRunnerWithoutDefaultRunner(t *testing.T) {
	tmpDir := t.TempDir()
	content := `version: "0.1.44"
stack:
  core-compose:
    runners:
      compose:
        files: [compose.yml]
`
	cfg := loadConfigForSchemaTest(t, tmpDir, content)

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for compose runner without default_runner")
	}
	if !strings.Contains(err.Error(), "Must not validate the schema") {
		t.Fatalf("Validate() error = %v, want schema rejection", err)
	}
}

func TestValidateRejectsUnknownComposeRunnerFields(t *testing.T) {
	tmpDir := t.TempDir()
	content := `version: "0.1.44"
stack:
  core-compose:
    default_runner: compose
    runners:
      compose:
        fiels: [compose.yml]
`
	cfg := loadConfigForSchemaTest(t, tmpDir, content)

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for unknown compose runner field")
	}
	if !strings.Contains(err.Error(), "Additional property fiels is not allowed") {
		t.Fatalf("Validate() error = %v, want unknown field rejection", err)
	}
}

func TestValidateRejectsWhitespaceComposeRunnerKeyBySchema(t *testing.T) {
	tmpDir := t.TempDir()
	content := `version: "0.1.44"
stack:
  core-compose:
    runners:
      " compose ":
        fiels: [compose.yml]
`
	cfg := loadConfigForSchemaTest(t, tmpDir, content)

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for whitespace compose runner key without default_runner")
	}
	if !strings.Contains(err.Error(), "runner names must not include leading or trailing whitespace") {
		t.Fatalf("Validate() error = %v, want runner-name whitespace rejection", err)
	}
}

func TestComposeHelpersReadComposeRunnerSchema(t *testing.T) {
	tmpDir := t.TempDir()
	content := `version: "0.1.44"
stack:
  core-compose:
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
        project_name: app
        tags: [infra]
        services:
          postgres:
            tags: [data]
`
	cfg := loadConfigForSchemaTest(t, tmpDir, content)

	if got := cfg.ComposeProjectName(); got != "app" {
		t.Fatalf("ComposeProjectName() = %q, want app", got)
	}
	files := cfg.AllComposeFiles()
	if len(files) != 1 || files[0] != "compose.yml" {
		t.Fatalf("AllComposeFiles() = %v, want [compose.yml]", files)
	}
	if got := cfg.ComposeTags(); len(got) != 1 || got[0] != "infra" {
		t.Fatalf("ComposeTags() = %v, want [infra]", got)
	}
	services := cfg.ComposeServices()
	if _, ok := services["postgres"]; !ok {
		t.Fatalf("ComposeServices() missing postgres: %v", services)
	}
}

func loadConfigForSchemaTest(t *testing.T, dir, content string) *Config {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return cfg
}
