package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// collectErrorsFixture carries three independent hard errors: a legacy compose entry
// (fails the strict load, and the schema separately), a hook on the removed 'clean' built-in (validateHookPlacement),
// and a default_plan that names no plan. Before TASK-305 `dva validate` stopped at the
// first of these and the author saw the other two only after fixing it.
const collectErrorsFixture = `default_plan: missing
stack:
  compose:
    files: [compose.yml]
interaction:
  clean:
    before:
      - {step: x, run: "echo x"}
    command: echo clean
`

func TestValidateReportsEveryHardErrorAtOnce(t *testing.T) {
	stdout, _, err := runValidate(t, collectErrorsFixture, false)
	if err == nil {
		t.Fatal("expected validate to fail")
	}
	msg := err.Error()
	for _, want := range []string{
		"4 errors found",
		`entry "compose": compose must be declared under runners.compose`,
		"interaction.clean: the 'clean' built-in was removed",
		"default_plan 'missing' is set but no plans are defined",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error should list %q, got:\n%s", want, msg)
		}
	}
	// Diagnostics after the hard errors still ran: the semantic warning about a config with
	// no plans is printed (on stdout, where notices go without --json), which the pre-305
	// flow never reached because the strict load failed first.
	if !strings.Contains(stdout, "[warn] semantic:") {
		t.Errorf("warnings should still be reported alongside hard errors, stdout:\n%s", stdout)
	}
}

func TestValidateJSONListsEveryHardError(t *testing.T) {
	stdout, _, err := runValidate(t, collectErrorsFixture, true)
	if err == nil {
		t.Fatal("expected validate to fail")
	}
	var report validateReport
	if jsonErr := json.Unmarshal([]byte(stdout), &report); jsonErr != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", jsonErr, stdout)
	}
	if report.Valid {
		t.Error("valid should be false")
	}
	// The schema error contributes one entry per offending key and the two plain errors one
	// each; the count is not asserted exactly because the schema list is the schema's to grow.
	var plain int
	for _, e := range report.Errors {
		if e.Path == "" {
			plain++
		}
	}
	if plain < 2 {
		t.Errorf("expected the hook and default_plan errors as separate entries, got %+v", report.Errors)
	}
	if report.Error == nil || !strings.Contains(report.Error.Message, "4 errors found") {
		t.Errorf("envelope should carry the joined message, got %+v", report.Error)
	}
}

func TestValidateSingleErrorMessageUnchanged(t *testing.T) {
	_, _, err := runValidate(t, schemaFailValidateFixture, false)
	if err == nil {
		t.Fatal("expected validate to fail")
	}
	if strings.Contains(err.Error(), "errors found") {
		t.Errorf("a single error must not be wrapped in the numbered list, got:\n%s", err)
	}
}
