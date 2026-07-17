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
	// The runners map is closed (additionalProperties: false), so a whitespace-padded
	// runner name is rejected by the schema before Validate() reaches the Go-side
	// whitespace check. Either layer reporting it satisfies the contract.
	if !strings.Contains(err.Error(), "runner names must not include leading or trailing whitespace") &&
		!strings.Contains(err.Error(), "Additional property  compose  is not allowed") {
		t.Fatalf("Validate() error = %v, want runner-name whitespace rejection", err)
	}
}

// TestValidateRejectsServiceRelatedAndHint locks TASK-036: related/hint were
// never consumed; schema must reject them rather than validate-green no-op.
func TestValidateRejectsServiceRelatedAndHint(t *testing.T) {
	tmpDir := t.TempDir()
	content := `version: "0.1.44"
stack:
  core-compose:
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
        services:
          api:
            tags: [web]
            related: [worker]
            hint: "never shown"
`
	cfg := loadConfigForSchemaTest(t, tmpDir, content)

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for services related/hint")
	}
	msg := err.Error()
	if !strings.Contains(msg, "Additional property related is not allowed") &&
		!strings.Contains(msg, "Additional property hint is not allowed") {
		t.Fatalf("Validate() error = %v, want related or hint rejection", msg)
	}
}

// TestValidateRejectsEnvFileInterpolate locks TASK-035: interpolate was never
// read; schema must reject it rather than validate-green no-op.
func TestValidateRejectsEnvFileInterpolate(t *testing.T) {
	tmpDir := t.TempDir()
	content := `version: "0.1.44"
env_file:
  files: [.env]
  interpolate: false
`
	cfg := loadConfigForSchemaTest(t, tmpDir, content)

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for env_file.interpolate")
	}
	if !strings.Contains(err.Error(), "Additional property interpolate is not allowed") {
		t.Fatalf("Validate() error = %v, want interpolate rejection", err)
	}
}

// TestValidateRejectsEnvFilePriority locks TASK-035: priority was never read.
func TestValidateRejectsEnvFilePriority(t *testing.T) {
	tmpDir := t.TempDir()
	content := `version: "0.1.44"
env_file:
  files: [.env]
  priority: after_environment
`
	cfg := loadConfigForSchemaTest(t, tmpDir, content)

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for env_file.priority")
	}
	if !strings.Contains(err.Error(), "Additional property priority is not allowed") {
		t.Fatalf("Validate() error = %v, want priority rejection", err)
	}
}

// TestValidateAcceptsEnvFileRequiredOnly ensures required still works after removal.
func TestValidateAcceptsEnvFileRequiredOnly(t *testing.T) {
	tmpDir := t.TempDir()
	content := `version: "0.1.44"
env_file:
  files: [.env]
  required: false
`
	cfg := loadConfigForSchemaTest(t, tmpDir, content)

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want accept env_file with required only", err)
	}
}

// TestValidateAcceptsDoctorCheckFix locks TASK-045: checks[].fix is honored by
// doctor --fix and must be schema-legal (not "Additional property fix is not allowed").
func TestValidateAcceptsDoctorCheckFix(t *testing.T) {
	tmpDir := t.TempDir()
	content := `version: "0.1.44"
checks:
  - name: "Sentinel file exists"
    type: file_exists
    path: .sentinel
    fix_hint: "touch .sentinel"
    fix: "touch .sentinel"
`
	cfg := loadConfigForSchemaTest(t, tmpDir, content)

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want accept checks[].fix", err)
	}
	if len(cfg.DoctorChecks) != 1 || cfg.DoctorChecks[0].Fix != "touch .sentinel" {
		t.Fatalf("DoctorChecks = %+v, want fix=touch .sentinel", cfg.DoctorChecks)
	}
}

// TestValidateAcceptsDoctorCheckWithoutFix ensures exposing fix does not require it.
func TestValidateAcceptsDoctorCheckWithoutFix(t *testing.T) {
	tmpDir := t.TempDir()
	content := `version: "0.1.44"
checks:
  - name: "Sentinel file exists"
    type: file_exists
    path: .sentinel
    fix_hint: "touch .sentinel"
`
	cfg := loadConfigForSchemaTest(t, tmpDir, content)

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want accept checks without fix", err)
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
