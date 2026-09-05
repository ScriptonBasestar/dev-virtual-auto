package config

import (
	"strings"
	"testing"
)

// TASK-317: every hint `dva config migrate` prints has to name a key that exists and a
// value the author wrote, and every legacy field that still parses has to be named.

func migrateBlocked(t *testing.T, src string) string {
	t.Helper()
	_, report, err := Migrate([]byte(src))
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return strings.Join(report.Blocked, "\n")
}

func migrateChanges(t *testing.T, src string) string {
	t.Helper()
	_, report, err := Migrate([]byte(src))
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return strings.Join(report.Changes, "\n")
}

func wantAll(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("missing %q in:\n%s", w, got)
		}
	}
}

func TestScaffoldModesQuotesTheProfilesTheModeDeclared(t *testing.T) {
	got := migrateBlocked(t, `version: "1.0"
stack:
  db:
    default_runner: compose
    runners: {compose: {files: [c.yml]}}
modes:
  full:
    stack: [db]
    compose_profiles: [dev-tools, debug]
`)
	wantAll(t, got, "--profile dev-tools --profile debug")
	if strings.Contains(got, "--profile full") {
		t.Errorf("mode name substituted for a profile:\n%s", got)
	}
}

func TestScaffoldModesPointsEnvironmentAtTheKeyTheSchemaAccepts(t *testing.T) {
	got := migrateBlocked(t, `version: "1.0"
stack:
  db:
    default_runner: compose
    runners: {compose: {files: [c.yml]}}
modes:
  hybrid:
    stack: [db]
    environment: {FOO: bar}
`)
	wantAll(t, got, "environments.hybrid.environment, selected by plans.hybrid.environment")
	if strings.Contains(got, "environments.hybrid.vars") {
		t.Errorf("hint names environments.<name>.vars, which the schema rejects:\n%s", got)
	}
}

func TestScaffoldModesTellsHowToRunAProvisionProfile(t *testing.T) {
	got := migrateBlocked(t, `version: "1.0"
stack:
  db:
    default_runner: compose
    runners: {compose: {files: [c.yml]}}
modes:
  full:
    stack: [db]
    provision: seed
`)
	wantAll(t, got, "'dva provision seed'", "before 'dva up full'")
}

func TestMigrateExplainsWhyTagsAppearTwice(t *testing.T) {
	got := migrateChanges(t, `version: "1.0"
stack:
  compose:
    files: [c.yml]
    tags: [core]
`)
	wantAll(t, got, "stack.compose.tags: kept on the entry and copied to runners.compose.tags on purpose")
}

func TestMigrateApplicationsSuggestsAnEndpointForTheDroppedPort(t *testing.T) {
	_, report, err := Migrate([]byte(`version: "1.0"
applications:
  api:
    port: 11200
    run: "cargo run"
`))
	if err != nil {
		t.Fatal(err)
	}
	wantAll(t, strings.Join(report.Changes, "\n"), `endpoints.api: {url: "http://localhost:11200"}`)
}

func TestMigrateApplicationsNamesTheInteractionAlreadyRunningDev(t *testing.T) {
	got := migrateBlocked(t, `version: "1.0"
applications:
  web:
    dev: pnpm dev
interaction:
  web-dev:
    command: pnpm dev
`)
	wantAll(t, got, "applications.web.dev", "interaction.web-dev already runs this exact command")
}

func TestReportLegacyFieldsNamesEachRemnant(t *testing.T) {
	got := migrateBlocked(t, `version: "1.0"
env_file:
  files: [.env]
  priority: high
  interpolate: true
environments:
  dev:
    compose_files: [dev.yml]
health_checks:
  api:
    type: http
    url: http://localhost:8080/health
    start: go run .
    start_hint: run the api first
interaction:
  boot:
    command: dva up -M full
  nested:
    subcommands:
      go:
        steps:
          - run: dva up --mode=full
  scripted:
    script: |
      dva up --mode full
provision:
  seed:
    - run: dva up -M full
`)
	wantAll(t, got,
		"environments.dev.compose_files: not read",
		"environments.dev.stack_overrides.<entry>",
		"env_file.priority: not read",
		"env_file.interpolate: not read",
		"health_checks.api.start: not run by 'dva up <plan>'",
		"health_checks.api.start_hint: not run",
		"stack.api.runners.native.command",
		"interaction.boot.command: still passes --mode/-M",
		"interaction.nested.subcommands.go.steps[0].run: still passes",
		"interaction.scripted.script: still passes",
		"provision.seed[0].run: still passes",
	)
}

func TestReportLegacyFieldsStaysQuietOnACleanConfig(t *testing.T) {
	if got := ReportLegacyFields([]byte(`version: "1.0"
env_file: {files: [.env]}
environments:
  dev: {environment: {A: b}}
interaction:
  up:
    command: dva up full
`)); len(got) != 0 {
		t.Fatalf("unexpected report: %v", got)
	}
}
