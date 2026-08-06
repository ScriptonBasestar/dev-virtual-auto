package config

import (
	"strings"
	"testing"
)

const stackOrderConfig = `version: "0.1.44"
stack:
  infra:
    order: 1
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
  api:
    order: 2
    default_runner: native
    runners:
      native:
        run: ./api
plans:
  full:
    entries:
      - name: infra
        services: [db]
      - name: api
`

// TestMigrateStackOrderMovesOrderOntoThePlanEntries.
//
// The decoded plan is the assertion rather than the text, because the value has to land
// where ResolvePlan reads it. It reads planEntry.Order with no fallback to the
// declaration, so an order that stayed on the stack entry is an order nothing applies.
func TestMigrateStackOrderMovesOrderOntoThePlanEntries(t *testing.T) {
	out, report, err := MigrateStackOrder([]byte(stackOrderConfig))
	if err != nil {
		t.Fatalf("MigrateStackOrder() error = %v", err)
	}
	if err := VerifyMigrated(out); err != nil {
		t.Fatalf("migrated config does not load: %v\n%s", err, out)
	}
	cfg, err := decodeConfig(out)
	if err != nil {
		t.Fatalf("decode migrated config: %v\n%s", err, out)
	}

	got := map[string]int{}
	for _, e := range cfg.Plans["full"].Entries {
		got[e.Name] = e.Order
	}
	for name, want := range map[string]int{"infra": 1, "api": 2} {
		if got[name] != want {
			t.Errorf("plans.full.entries[%s].order = %d, want %d\n%s", name, got[name], want, out)
		}
	}

	for name := range cfg.Stack {
		if cfg.Stack[name].Order != 0 {
			t.Errorf("stack.%s.order survived the move, so the file still declares it twice:\n%s", name, out)
		}
	}

	// The entry's other keys must be untouched: the order line is removed, not the block
	// re-encoded around it.
	if !strings.Contains(string(out), "        files: [compose.yml]") {
		t.Errorf("the entry was reformatted rather than edited:\n%s", out)
	}
	if !strings.Contains(string(out), "        services: [db]") {
		t.Errorf("the plan entry's own keys were disturbed:\n%s", out)
	}

	joined := strings.Join(report.Changes, "\n")
	if !strings.Contains(joined, "plans.full.entries[infra].order") {
		t.Errorf("the report must name where each order went:\n%s", joined)
	}
}

// TestMigrateStackOrderRefusesWhenNoPlanReferencesTheEntry: the plan-less path still
// sorts declarations by this field, so removing it without a destination would break the
// ordering instead of relocating it.
//
// A refusal is reported, not returned as an error. The order is one section of one file,
// and failing the whole run over it would take the conversions that did succeed with it.
func TestMigrateStackOrderRefusesWhenNoPlanReferencesTheEntry(t *testing.T) {
	src := `version: "0.1.44"
stack:
  infra:
    order: 1
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
plans:
  other:
    entries:
      - name: api
`
	out, report, err := MigrateStackOrder([]byte(src))
	if err != nil {
		t.Fatalf("MigrateStackOrder() error = %v", err)
	}
	if string(out) != src {
		t.Errorf("an order with nowhere to go was moved anyway:\n%s", out)
	}
	if len(report.Changes) != 0 {
		t.Errorf("a refused migration still reported changes: %v", report.Changes)
	}
	if !strings.Contains(strings.Join(report.Blocked, "\n"), "stack.infra.order") {
		t.Errorf("the refusal must name the entry, got: %v", report.Blocked)
	}
}

// TestMigrateStackOrderSaysWhenThereAreNoPlansAtAll: "add it to a plan's entries[]" is
// useless advice when there is no plan, and writing one is the larger job.
func TestMigrateStackOrderSaysWhenThereAreNoPlansAtAll(t *testing.T) {
	_, report, err := MigrateStackOrder([]byte(`version: "0.1.44"
stack:
  infra:
    order: 1
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
`))
	if err != nil {
		t.Fatalf("MigrateStackOrder() error = %v", err)
	}

	if !strings.Contains(strings.Join(report.Blocked, "\n"), "declares no plans") {
		t.Errorf("the refusal must say a plan has to exist first, got: %v", report.Blocked)
	}
}

// TestMigrateStackOrderDropsAnOrderThePlanAlreadyOverrides: the plan's value is the one
// ResolvePlan uses, so the declaration's copy was already dead. Removing it silently
// would still be a change to the file, so it is reported.
func TestMigrateStackOrderDropsAnOrderThePlanAlreadyOverrides(t *testing.T) {
	out, report, err := MigrateStackOrder([]byte(`version: "0.1.44"
stack:
  infra:
    order: 1
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
plans:
  full:
    entries:
      - name: infra
        order: 7
`))
	if err != nil {
		t.Fatalf("MigrateStackOrder() error = %v", err)
	}

	cfg, err := decodeConfig(out)
	if err != nil {
		t.Fatalf("decode migrated config: %v\n%s", err, out)
	}
	if got := cfg.Plans["full"].Entries[0].Order; got != 7 {
		t.Errorf("plan order = %d, want the plan's own 7 kept\n%s", got, out)
	}
	if cfg.Stack["infra"].Order != 0 {
		t.Errorf("the dead declaration order survived:\n%s", out)
	}
	if !strings.Contains(strings.Join(report.Changes, "\n"), "already orders this entry") {
		t.Errorf("the report must explain why the value was dropped rather than moved: %v", report.Changes)
	}
}

// TestMigrateStackOrderIgnoresAConfigWithoutIt keeps the no-op byte-exact: the command
// runs on every config, and a reformat is not a migration.
func TestMigrateStackOrderIgnoresAConfigWithoutIt(t *testing.T) {
	src := `version: "0.1.44"

stack:
  infra:
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
`
	out, report, err := MigrateStackOrder([]byte(src))
	if err != nil {
		t.Fatalf("MigrateStackOrder() error = %v", err)
	}
	if !report.Empty() {
		t.Errorf("reported changes for a config with no stack order: %+v", report)
	}
	if string(out) != src {
		t.Errorf("a config with nothing to migrate was rewritten:\n%s", out)
	}
}
