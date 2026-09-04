package config

import (
	"strings"
	"testing"
)

const modesStackFixture = `version: "0.1.44"
stack:
  core:
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
  api:
    default_runner: native
    runners:
      native:
        run: "go run ."

default_mode: full

# Each mode picks entries.
modes:
  minimal:
    description: "db only"
    stack: [core]

  full:
    description: "everything"
    stack: [core, api]
    endpoint_tags: [web]
`

// TestMigrateModesStackSelect: a mode that only selects stack entries is a plan whose
// entries are that selection, named after the mode. default_mode follows it.
func TestMigrateModesStackSelect(t *testing.T) {
	out, report, err := MigrateModes([]byte(modesStackFixture))
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, want := range []string{
		"plans:\n  minimal:\n    description: \"db only\"\n    entries:\n      - name: core\n",
		"  full:\n    description: \"everything\"\n    endpoint_tags: [web]\n    entries:\n      - name: core\n      - name: api\n",
		"default_plan: full\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	for _, gone := range []string{"modes:", "default_mode:", "  minimal:\n    description: \"db only\"\n    stack:"} {
		if strings.Contains(got, gone) {
			t.Errorf("%q should have been removed from:\n%s", gone, got)
		}
	}
	if !strings.Contains(got, "# Each mode picks entries.") || !strings.Contains(got, "  api:\n    default_runner: native") {
		t.Errorf("untouched lines must survive byte for byte:\n%s", got)
	}
	wantChanges := []string{
		"modes.minimal → plans.minimal (run 'dva up minimal' instead of '--mode minimal')",
		"modes.full → plans.full (run 'dva up full' instead of '--mode full')",
		"default_mode: full → default_plan: full",
	}
	if strings.Join(report.Changes, "|") != strings.Join(wantChanges, "|") {
		t.Errorf("changes = %q, want %q", report.Changes, wantChanges)
	}
	if len(report.Blocked) != 0 {
		t.Errorf("nothing should be blocked, got %q", report.Blocked)
	}

	// The result is a config this version loads, and it loads as plans.
	cfg := loadConfigForSchemaTest(t, t.TempDir(), got)
	if cfg.DefaultPlanName != "full" || len(cfg.Plans) != 2 || len(cfg.Modes) != 0 {
		t.Errorf("default_plan=%q plans=%d modes=%d", cfg.DefaultPlanName, len(cfg.Plans), len(cfg.Modes))
	}
	if entries := cfg.Plans["full"].Entries; len(entries) != 2 || entries[0].Name != "core" || entries[1].Name != "api" {
		t.Errorf("plans.full.entries = %+v", entries)
	}
}

// TestMigrateModesComposeServices: a mode narrowing compose services attaches the list to
// the one compose entry it runs, with no stack: meaning every entry.
func TestMigrateModesComposeServices(t *testing.T) {
	src := `stack:
  core:
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
modes:
  db-only:
    compose_services: [db, cache]
`
	out, report, err := MigrateModes([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	want := "plans:\n  db-only:\n    entries:\n      - name: core\n        services: [db, cache]\n"
	if !strings.Contains(string(out), want) {
		t.Errorf("missing %q in:\n%s", want, out)
	}
	if len(report.Blocked) != 0 || len(report.Changes) != 1 {
		t.Errorf("report = %+v", report)
	}
	cfg := loadConfigForSchemaTest(t, t.TempDir(), string(out))
	if e := cfg.Plans["db-only"].Entries; len(e) != 1 || strings.Join(e[0].Services, ",") != "db,cache" {
		t.Errorf("entries = %+v", e)
	}
}

// TestMigrateModesLeavesWhatNeedsHands: each unconvertible shape stays in the file with
// its reason, while a convertible sibling still moves.
func TestMigrateModesLeavesWhatNeedsHands(t *testing.T) {
	src := `stack:
  core:
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
  extra:
    default_runner: compose
    runners:
      compose:
        files: [extra.yml]
default_mode: profiled
default_plan: existing
plans:
  existing:
    entries:
      - name: core
  taken:
    entries:
      - name: core
modes:
  profiled:
    stack: [core]
    compose_profiles: [debug]
  ambiguous:
    stack: [core, extra]
    compose_services: [db]
  taken:
    stack: [core]
  dangling:
    stack: [nope]
  empty:
    description: "selects nothing"
  fine:
    stack: [extra]
`
	out, report, err := MigrateModes([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "  fine:\n    entries:\n      - name: extra\n") {
		t.Errorf("the convertible mode should be appended to the existing plans block:\n%s", got)
	}
	if strings.Contains(got, "modes:\n  profiled:") == false || strings.Contains(got, "  fine:\n    stack: [extra]") {
		t.Errorf("unconvertible modes stay, converted ones go:\n%s", got)
	}
	if !strings.Contains(got, "default_mode: profiled") {
		t.Errorf("default_mode must stay while its mode is unconverted:\n%s", got)
	}
	for _, want := range []string{
		"modes.profiled: not converted — compose_profiles has no plan equivalent",
		"modes.ambiguous: not converted — compose_services could attach to any of core, extra",
		"modes.taken: not converted — plans.taken already exists",
		`modes.dangling: not converted — stack selects "nope", which is not declared under stack:`,
		"modes.empty: not converted — selects nothing",
	} {
		if !hasPrefixIn(report.Blocked, want) {
			t.Errorf("blocked should include %q, got %q", want, report.Blocked)
		}
	}
	if len(report.Blocked) != 5 {
		t.Errorf("expected 5 blocked, got %d: %q", len(report.Blocked), report.Blocked)
	}
}

// TestMigrateModesDefaultConflict: when default_plan is already set, the mode default_mode
// names is left alone rather than the tool choosing which default wins.
func TestMigrateModesDefaultConflict(t *testing.T) {
	src := `stack:
  core:
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
default_mode: full
default_plan: other
plans:
  other:
    entries:
      - name: core
modes:
  full:
    stack: [core]
`
	out, report, err := MigrateModes([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != src {
		t.Errorf("file should be untouched:\n%s", out)
	}
	if len(report.Blocked) != 1 || !strings.Contains(report.Blocked[0], `default_mode is "full" but default_plan is already "other"`) {
		t.Errorf("blocked = %q", report.Blocked)
	}
}

// TestMigratePipelineScaffoldsOnlyRemainingModes: the full pipeline converts what it can
// and ScaffoldModes describes only what is left.
func TestMigratePipelineScaffoldsOnlyRemainingModes(t *testing.T) {
	src := `stack:
  core:
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
modes:
  plain:
    stack: [core]
  profiled:
    stack: [core]
    compose_profiles: [debug]
`
	_, report, err := Migrate([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(report.Blocked, "\n")
	if strings.Contains(joined, "modes.plain: split by hand") {
		t.Errorf("a converted mode must not be scaffolded:\n%s", joined)
	}
	if !strings.Contains(joined, "modes.profiled: split by hand") || !strings.Contains(joined, "modes.profiled: not converted") {
		t.Errorf("the remaining mode gets both the reason and the field targets:\n%s", joined)
	}
}

func hasPrefixIn(list []string, prefix string) bool {
	for _, s := range list {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}
