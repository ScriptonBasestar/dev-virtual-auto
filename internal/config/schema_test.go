package config

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
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

// The three legacy compose shapes are rejected by Load(), not only by Validate().
// Before, schema.json refused them while the loader accepted them, so `dva validate`
// failed on configs every other command ran happily — the split that made schema
// rejection unreachable in practice. Asserting on Load() pins the stronger contract:
// one answer from both, and an error that names the replacement shape.
func TestLoadRejectsLegacyNestedCompose(t *testing.T) {
	requireLegacyComposeRejected(t, `version: "0.1.44"
stack:
  core-compose:
    compose:
      files: [compose.yml]
`)
}

func TestLoadRejectsLegacyFlatComposePlugin(t *testing.T) {
	requireLegacyComposeRejected(t, `version: "0.1.44"
stack:
  core-compose:
    plugin: compose
    files: [compose.yml]
`)
}

func TestLoadRejectsLegacyAutoInferredCompose(t *testing.T) {
	requireLegacyComposeRejected(t, `version: "0.1.44"
stack:
  compose:
    command: /bin/echo
`)
}

func requireLegacyComposeRejected(t *testing.T, content string) {
	t.Helper()

	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, FileName), []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(tmpDir)
	if err == nil {
		t.Fatal("Load() accepted a compose shape schema.json rejects")
	}
	if !strings.Contains(err.Error(), "runners.compose") {
		t.Fatalf("Load() error = %v, want the message to name runners.compose", err)
	}
	if !strings.Contains(err.Error(), "default_runner: compose") {
		t.Fatalf("Load() error = %v, want the message to show the replacement shape", err)
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

// TestValidateAcceptsInteractionExecutionForms pins every execution form
// LocalRunner.Execute dispatches on (steps > script_file > script > command list >
// command) as schema-legal. Three of them — script, script_file, steps — were
// implemented in the struct and both runners while interaction_command listed
// neither, so `additionalProperties: false` made documented features unreachable and
// left `runner: local` + a single command: the only way through.
//
// Asserting the parsed fields, not just Validate(), is the point: the defect was the
// schema and the struct disagreeing, and only a round trip catches that.
func TestValidateAcceptsInteractionExecutionForms(t *testing.T) {
	tmpDir := t.TempDir()
	content := `version: "0.1.44"
interaction:
  seed:
    description: "Inline script"
    runner: local
    script: |
      set -e
      echo seeding
  seed-file:
    description: "External script"
    runner: local
    script_file: scripts/seed.sh
  bootstrap:
    description: "Named steps"
    runner: local
    steps:
      - step: "Install"
        run: "pnpm install"
      - "pnpm build"
  bench:
    description: "Command list"
    runner: local
    command:
      - "go build ./..."
      - "go test ./..."
`
	cfg := loadConfigForSchemaTest(t, tmpDir, content)

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want accept script/script_file/steps/command list", err)
	}

	if got := cfg.Interaction["seed"]; !got.HasScript() || !strings.Contains(got.Script, "echo seeding") {
		t.Errorf("seed.script = %q, want the inline block", got.Script)
	}
	if got := cfg.Interaction["seed-file"]; got.ScriptFile != "scripts/seed.sh" {
		t.Errorf("seed-file.script_file = %q, want scripts/seed.sh", got.ScriptFile)
	}
	if got := cfg.Interaction["bootstrap"]; len(got.Steps) != 2 || got.Steps[0].Step != "Install" {
		t.Errorf("bootstrap.steps = %+v, want 2 items starting with Install", got.Steps)
	}
	// command: as a list keeps Command set to the first line for display, so asserting
	// CommandLines is what distinguishes a parsed list from a parsed scalar.
	if got := cfg.Interaction["bench"]; !got.HasMultiCommand() || len(got.CommandLines) != 2 {
		t.Errorf("bench.command lines = %+v, want 2", got.CommandLines)
	}
}

// TestValidateRejectsDevcontainerConfigPath locks TASK-037: config_path was never
// honored; schema must reject it rather than validate-green no-op.
func TestValidateRejectsDevcontainerConfigPath(t *testing.T) {
	tmpDir := t.TempDir()
	content := `version: "0.1.44"
devcontainer:
  enabled: true
  config_path: custom/somewhere/devcontainer.json
  image: mcr.microsoft.com/devcontainers/base:ubuntu
`
	cfg := loadConfigForSchemaTest(t, tmpDir, content)

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for devcontainer.config_path")
	}
	if !strings.Contains(err.Error(), "Additional property config_path is not allowed") {
		t.Fatalf("Validate() error = %v, want config_path rejection", err)
	}
}

// TestValidateRejectsLegacyProvisionShellSleepDocker locks TASK-044: shell/sleep/docker
// structured keys were never executed; schema must reject them.
func TestValidateRejectsLegacyProvisionShellSleepDocker(t *testing.T) {
	cases := []struct {
		name    string
		snippet string
		key     string
	}{
		{"sleep", `provision:
  setup:
    - sleep: 10
`, "sleep"},
		{"shell", `provision:
  setup:
    - shell: echo should-fail
`, "shell"},
		{"docker", `provision:
  setup:
    - docker: {compose: docker-compose.yml}
`, "docker"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			content := "version: \"0.1.44\"\n" + tc.snippet
			cfg := loadConfigForSchemaTest(t, tmpDir, content)
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("Validate() expected error for provision %s", tc.name)
			}
			msg := err.Error()
			if !strings.Contains(msg, tc.key) &&
				!strings.Contains(msg, "Must validate one and only one schema") &&
				!strings.Contains(msg, "Additional property") {
				t.Fatalf("Validate() error = %v, want rejection naming %q", msg, tc.key)
			}
		})
	}
}

// TestValidateAcceptsLegacyProvisionEchoCmd ensures echo/cmd still validate.
func TestValidateAcceptsLegacyProvisionEchoCmd(t *testing.T) {
	tmpDir := t.TempDir()
	content := `version: "0.1.44"
provision:
  setup:
    - echo: "Starting..."
    - cmd: echo ok
    - sleep 1
`
	cfg := loadConfigForSchemaTest(t, tmpDir, content)
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want accept echo/cmd and raw sleep", err)
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

// TestValidateWithoutVersion locks TASK-067 rule A. `version:` is optional in Go —
// checkConfigVersion returns nil early on "" and comments the empty case as allowed —
// but schema.json listed it in the top-level `required`. So a config with no `version:`
// key loaded and ran under ls/doctor/manifest/show and failed under `dva validate`
// alone: the validator reported a defect in a config the rest of the tool ran without
// complaint, which is the worst direction for a validator to be wrong in.
//
// Nothing pinned either behavior before this — no test in the repo contained the string
// "version is required" or referenced checkConfigVersion, which is why the schema and
// the loader could disagree for as long as they did.
func TestValidateWithoutVersion(t *testing.T) {
	tmpDir := t.TempDir()
	content := `stack:
  core-compose:
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
        project_name: app
`
	cfg := loadConfigForSchemaTest(t, tmpDir, content)

	if cfg.Version != "" {
		t.Fatalf("Version = %q, want empty for a config with no version: key", cfg.Version)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want a version-less config to validate", err)
	}
}

// TestValidateAcceptsModeBuildRunKeys covers TASK-138: schema.json's `modes` object rejected
// `build` and `run` even though ModeConfig declares them and the build runner reads Build
// (compose.go). The schema and the struct disagreed, so `dva validate` failed on a config every
// other command ran. Asserting the parsed fields, not just Validate(), is the point — the defect
// was the schema forbidding what the struct parses, and only a round trip catches that.
func TestValidateAcceptsModeBuildRunKeys(t *testing.T) {
	tmpDir := t.TempDir()
	content := `version: "0.1.44"
modes:
  native:
    build: native
    run: native
`
	cfg := loadConfigForSchemaTest(t, tmpDir, content)

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want modes.<name>.build/.run accepted", err)
	}
	mode, ok := cfg.Modes["native"]
	if !ok {
		t.Fatalf("modes.native not parsed; got %+v", cfg.Modes)
	}
	if mode.Build != "native" {
		t.Errorf("modes.native.build = %q, want \"native\"", mode.Build)
	}
	if mode.Run != "native" {
		t.Errorf("modes.native.run = %q, want \"native\"", mode.Run)
	}
}

// TestModeConfigSchemaAgreesWithStruct is the guard TASK-138 leaves behind. A field added to
// ModeConfig but not to schema.json is invisible until someone writes a config using it and runs
// `dva validate` — the last moment to learn the schema disagrees. This test compares the struct's
// yaml tags against the schema's `modes` properties and fails the moment their KEY SETS diverge
// in either direction. Scope is presence only, not type agreement — type drift is caught by the
// round-trip test above (TestValidateAcceptsModeBuildRunKeys).
//
// Non-vacuous: the second half removes a key from a copy of the schema and asserts the SAME
// comparison surfaces it — through the shared helper, not a re-implementation, so a refactor of
// the main assertions cannot make the proof vacuous while the test still passes.
func TestModeConfigSchemaAgreesWithStruct(t *testing.T) {
	schemaProps := modeSchemaPropertyKeys(t)

	for _, k := range missingFromSchema(schemaProps) {
		t.Errorf("ModeConfig field %q has no matching property in schema.json modes — the schema rejects what the struct parses", k)
	}
	for _, k := range missingFromStruct(schemaProps) {
		t.Errorf("schema.json modes property %q has no matching ModeConfig field — the struct silently drops it", k)
	}

	// Prove the comparison is not vacuous through the same code path the main assertions use.
	t.Run("detects a removed schema key", func(t *testing.T) {
		trimmed := maps.Clone(schemaProps)
		// `environment` is always present; removing it must surface as missing-from-schema.
		delete(trimmed, "environment")
		missing := missingFromSchema(trimmed)
		if !slices.Contains(missing, "environment") {
			t.Fatalf("removing 'environment' was not detected (missing=%v) — the agreement check is vacuous", missing)
		}
	})
}

// missingFromSchema returns ModeConfig yaml keys absent from schemaProps; missingFromStruct the
// reverse. Both derive ModeConfig's keys from modeConfigYamlKeys, so the agreement test and its
// non-vacuous proof share one notion of the Go-side contract.
func missingFromSchema(schemaProps map[string]bool) []string {
	want := modeConfigYamlKeys()
	var missing []string
	for k := range want {
		if !schemaProps[k] {
			missing = append(missing, k)
		}
	}
	return missing
}

func missingFromStruct(schemaProps map[string]bool) []string {
	want := modeConfigYamlKeys()
	var missing []string
	for k := range schemaProps {
		if !want[k] {
			missing = append(missing, k)
		}
	}
	return missing
}

// modeConfigYamlKeys returns ModeConfig's yaml-tagged field names — the Go-side contract.
func modeConfigYamlKeys() map[string]bool {
	want := map[string]bool{}
	for field := range reflect.TypeFor[ModeConfig]().Fields() {
		name, _, _ := strings.Cut(field.Tag.Get("yaml"), ",")
		if name == "" || name == "-" {
			continue
		}
		want[name] = true
	}
	return want
}

// modeSchemaPropertyKeys reads the embedded schema.json and returns the property keys declared
// under modes.patternProperties.<pattern>.properties.
func modeSchemaPropertyKeys(t *testing.T) map[string]bool {
	t.Helper()
	raw, err := embeddedSchema.ReadFile("schema.json")
	if err != nil {
		t.Fatalf("read embedded schema: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	modes, ok := schema["properties"].(map[string]any)["modes"].(map[string]any)
	if !ok {
		t.Fatal("schema has no modes object")
	}
	patternProps, ok := modes["patternProperties"].(map[string]any)
	if !ok {
		t.Fatal("modes has no patternProperties")
	}
	// Exactly one pattern is expected; a second would otherwise be silently dropped by the loop.
	if len(patternProps) != 1 {
		t.Fatalf("expected exactly 1 pattern under modes.patternProperties, got %d", len(patternProps))
	}
	var props map[string]any
	for _, v := range patternProps {
		if entry, ok := v.(map[string]any); ok {
			if p, ok := entry["properties"].(map[string]any); ok {
				props = p
			}
		}
	}
	if props == nil {
		t.Fatal("no properties found under modes.patternProperties")
	}
	keys := make(map[string]bool, len(props))
	for k := range props {
		keys[k] = true
	}
	return keys
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
