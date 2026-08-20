// Package cli — regression tests for TASK-211.
//
// parseDvaFlags used to drop a value-taking flag that had nothing to take. The token
// was neither stored nor forwarded to filtered, so `dva restart --mode` ran the whole
// stack and exited 0 — the widest possible result for someone who typed a narrowing
// flag. These tests pin the refusal, and pin that it happens before anything runs.
package cli

import (
	"strings"
	"testing"
)

// TestParseDvaFlagsRejectsAMissingValue covers both spellings of "nothing to take":
// the flag at the end of argv, and the flag immediately before the terminator, which
// dvaFlagEnd makes the end as far as flagValue is concerned. A fix that handled only
// the first would leave `--mode --` silently dropped, which is the shape TASK-207's
// review actually walked into.
func TestParseDvaFlagsRejectsAMissingValue(t *testing.T) {
	// wantFlag is the flag the message must name, spelled as the user spelled it. Asserting
	// only the "requires a value" phrase was not enough: an adversarial review replaced
	// fmt.Errorf("%s requires a value", name) with a hardcoded "--mode requires a value"
	// and the whole package stayed green. That build sends someone who typed `dva restart
	// --tag` off to fix a --mode they never wrote. Passing the flag's name through is also
	// the sole argument for reporting in parseDvaFlags rather than in flagValue, so until
	// these rows existed the suite did not check the property the design rests on.
	missing := []struct {
		what     string
		args     []string
		wantFlag string
	}{
		{"a trailing --mode", []string{"--mode"}, "--mode"},
		{"--mode before the terminator", []string{"--mode", "--"}, "--mode"},
		{"the short form -M", []string{"-M"}, "-M"},
		{"a repeatable --tag", []string{"--tag"}, "--tag"},
		{"--tag before the terminator", []string{"--tag", "--"}, "--tag"},
		{"--env", []string{"--env"}, "--env"},
		{"--exclude-tag", []string{"--exclude-tag"}, "--exclude-tag"},
	}
	for _, tc := range missing {
		t.Run(tc.what, func(t *testing.T) {
			dir := writeRestartProbeConfig(t)

			err := restartCmd.RunE(restartCmd, tc.args)
			if err == nil {
				t.Fatalf("restart %v: want an error, got nil", tc.args)
			}
			if !strings.Contains(err.Error(), "requires a value") {
				t.Errorf("restart %v: %q does not say a value is required", tc.args, err)
			}
			// The short form matters most here: -M must be reported as -M. A message that
			// silently rewrote it to --mode would be naming a token the user did not type.
			if !strings.Contains(err.Error(), tc.wantFlag) {
				t.Errorf("restart %v: %q does not name %s", tc.args, err, tc.wantFlag)
			}
			// Asserting only that it errored would pass on a build that acts first and
			// complains afterwards. The markers are what say nothing was touched.
			if got := ranMarkers(t, dir); len(got) != 0 {
				t.Errorf("restart %v: nothing should have run, ran %v", tc.args, got)
			}
		})
	}

	// The control. Same fixture, same flag, value present — it must still be taken and
	// the stack must still bounce. Without this row a build that refused every --env
	// outright, or one whose restart had stopped running anything at all, would satisfy
	// every row above.
	t.Run("but a value that is there is still taken", func(t *testing.T) {
		dir := writeRestartProbeConfig(t)

		if err := restartCmd.RunE(restartCmd, []string{"--env", "dev"}); err != nil {
			t.Fatalf("restart --env dev: %v", err)
		}
		got := ranMarkers(t, dir)
		for _, m := range []string{"s1_up", "s1_stop", "s2_up", "s2_stop"} {
			if !got[m] {
				t.Errorf("restart --env dev: %s missing, ran %v", m, got)
			}
		}
	})
}

// TestParseDvaFlagsRejectsAnEmptyValue covers the other way a value-taking flag ends up
// with no usable value: one was supplied and it is empty. TASK-211 closed `--mode` and
// `--mode --`; `--mode=` went on running the whole stack at rc=0, which is verbatim the
// harm TASK-211's summary describes, reached through the branch above the one it fixed.
// flagValue returns ok=true for an empty inline value, so takeValue never fired and mode
// was set to "" — indistinguishable downstream from never having typed the flag, because
// that is exactly what "no --mode" leaves behind. TASK-213.
//
// The `--mode ""` rows are here for the same reason `--mode --` is in the missing table:
// it is the same emptiness reached through the other flagValue branch, so a fix written
// only against the `=` spelling leaves a twin open. Both are refused by one check in
// takeValue, which is why this table mixes them rather than splitting the function.
func TestParseDvaFlagsRejectsAnEmptyValue(t *testing.T) {
	empty := []struct {
		what     string
		args     []string
		wantFlag string
	}{
		{"--mode=", []string{"--mode="}, "--mode"},
		{"the short form -M=", []string{"-M="}, "-M"},
		{"--env=", []string{"--env="}, "--env"},
		{"--tag=", []string{"--tag="}, "--tag"},
		{"--exclude-tag=", []string{"--exclude-tag="}, "--exclude-tag"},
		{"--mode with an empty next token", []string{"--mode", ""}, "--mode"},
		{"--tag with an empty next token", []string{"--tag", ""}, "--tag"},
	}
	for _, tc := range empty {
		t.Run(tc.what, func(t *testing.T) {
			dir := writeRestartProbeConfig(t)

			err := restartCmd.RunE(restartCmd, tc.args)
			if err == nil {
				t.Fatalf("restart %v: want an error, got nil", tc.args)
			}
			// Deliberately not "requires a value": the user did supply one. Asserting the
			// distinct phrase also keeps this table from passing on the TASK-211 message,
			// which would mean the empty case had been folded into the missing case and
			// the two branches were no longer separable by test.
			if !strings.Contains(err.Error(), "requires a non-empty value") {
				t.Errorf("restart %v: %q does not say the value must be non-empty", tc.args, err)
			}
			if !strings.Contains(err.Error(), tc.wantFlag) {
				t.Errorf("restart %v: %q does not name %s", tc.args, err, tc.wantFlag)
			}
			if got := ranMarkers(t, dir); len(got) != 0 {
				t.Errorf("restart %v: nothing should have run, ran %v", tc.args, got)
			}
		})
	}

	// The control. Every rejection above is satisfied by a build that refuses the inline
	// spelling outright — `--env=dev` too — which would be a far worse regression than the
	// bug being fixed and is invisible to the table.
	//
	// It is not, however, the only guard, and the first draft of this comment claimed it
	// was. Sabotaging takeValue to refuse every inline value fails this row plus eight
	// pre-existing tests that call parseDvaFlags directly — TestParseDvaFlags_EqualsSyntax,
	// _ShortEqualsSyntax, _TagEqualsFormat, _TagsEqualsFormat, _ExcludeTagEquals,
	// _ExcludeTagsEquals, _IncludeTagsCommaSeparated and _ExcludeTagsCommaSeparated in
	// compose_flags_test.go. The claim was written before it was measured and the sabotage
	// disproved it; a "this is the only test that catches X" comment is exactly what a
	// later refactor cites when deleting the row. What this row adds over those eight is
	// the path: they stop at parseDvaFlags, this one goes through restartCmd.RunE and
	// asserts the stack actually bounced, so it also fails on a build that refuses the
	// value somewhere further down.
	//
	// It is deliberately `--env=dev`, the same flag and value as the missing table's
	// control one spelling apart, so the only difference between a passing control and a
	// passing rejection row is the `=`. `--mode=dev` looks like the better control and is
	// not: this fixture declares no per-entry modes, so mode filtering drops s1 and s2 and
	// the row fails with "no lifecycle entries matched filters" on a correct build. That
	// is a control reporting on the fixture rather than on the fix.
	t.Run("but a non-empty inline value is still taken", func(t *testing.T) {
		dir := writeRestartProbeConfig(t)

		if err := restartCmd.RunE(restartCmd, []string{"--env=dev"}); err != nil {
			t.Fatalf("restart --env=dev: %v", err)
		}
		got := ranMarkers(t, dir)
		for _, m := range []string{"s1_up", "s1_stop", "s2_up", "s2_stop"} {
			if !got[m] {
				t.Errorf("restart --env=dev: %s missing, ran %v", m, got)
			}
		}
	})
}

// The four below predate TASK-211 and lived in compose_flags_test.go, where every one of
// them discarded err with `_`. They therefore passed identically before the fix and
// after it — the state they asserted (an empty result) was true either way — so what
// they were actually documenting was the silent drop, as intended behaviour. They now
// require the error, which is what makes them a test of this fix rather than a record of
// the defect. They are kept because they reach parseDvaFlags directly: the subtests
// above go through restartCmd.RunE, so a refusal that came from somewhere else on that
// path would still satisfy them.

func TestParseDvaFlags_MissingValue(t *testing.T) {
	mode, _, _, _, filtered, err := parseDvaFlags([]string{"--mode"})
	if err == nil {
		t.Fatal("parseDvaFlags([--mode]) returned no error, want a missing-value error")
	}
	if mode != "" {
		t.Errorf("mode = %q, want empty (no value provided)", mode)
	}
	// Worth asserting apart from the error: a recognised flag is never appended to
	// filtered, so the token reaches no passthrough caller's argv either.
	if len(filtered) != 0 {
		t.Errorf("filtered = %v, want empty", filtered)
	}
}

func TestParseDvaFlags_MissingEnvValue(t *testing.T) {
	_, env, _, _, _, err := parseDvaFlags([]string{"--env"})
	if err == nil {
		t.Fatal("parseDvaFlags([--env]) returned no error, want a missing-value error")
	}
	if env != "" {
		t.Errorf("env = %q, want empty (no value)", env)
	}
}

func TestParseDvaFlags_MissingTagValue(t *testing.T) {
	_, _, includeTags, _, _, err := parseDvaFlags([]string{"--tag"})
	if err == nil {
		t.Fatal("parseDvaFlags([--tag]) returned no error, want a missing-value error")
	}
	if len(includeTags) != 0 {
		t.Errorf("includeTags = %v, want empty (no value)", includeTags)
	}
}

func TestParseDvaFlags_MissingExcludeTagValue(t *testing.T) {
	_, _, _, excludeTags, _, err := parseDvaFlags([]string{"--exclude-tag"})
	if err == nil {
		t.Fatal("parseDvaFlags([--exclude-tag]) returned no error, want a missing-value error")
	}
	if len(excludeTags) != 0 {
		t.Errorf("excludeTags = %v, want empty (no value)", excludeTags)
	}
}
