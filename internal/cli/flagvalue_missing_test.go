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
		// Every spelling parseDvaFlags recognises, not a representative sample. The four
		// below were missing until an adversarial review guarded takeValue with `&& name !=
		// "--tags" && name != "--exclude-tags" && name != "-E" && name != "-T"` and the
		// whole package stayed green while `dva restart -E=` and `dva restart
		// --exclude-tags=` bounced the entire stack at rc=0. The case arms alias these to
		// the long forms, so a row per alias looks redundant — it is exactly what catches a
		// fix applied per-name instead of at the funnel. TASK-213.
		{"the short form -E=", []string{"-E="}, "-E"},
		{"the short form -T=", []string{"-T="}, "-T"},
		{"the plural --tags=", []string{"--tags="}, "--tags"},
		{"the plural --exclude-tags=", []string{"--exclude-tags="}, "--exclude-tags"},
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

// TestParseDvaFlagsRejectsADegenerateValue covers the values that are not empty but carry
// no more information than empty does. An adversarial review of the fix above found the
// harm reachable one character past the spelling it refuses: `dva restart --exclude-tag=,`
// is a one-character value, so the whole-value check passes it, and strings.Split turns it
// into ["", ""] — two tags that match nothing. For --exclude-tag, matching nothing means
// excluding nothing, so that spelling bounced the entire stack and exited 0, which is
// verbatim the harm this card exists to close. TASK-213.
//
// The family is not uniform and the rows say so per flag, because the first draft of the
// card measured the safe member (`--tag=a,,b`, which narrows to nothing) and generalised to
// the class. Measured on the unfixed build: `--exclude-tag=,` and `--exclude-tag=" "` run
// EVERYTHING, `--tag=,` and `--tag=" "` run NOTHING, and both are reported as success. The
// second pair is the quieter defect, not the absent one.
func TestParseDvaFlagsRejectsADegenerateValue(t *testing.T) {
	// wantPhrase differs by shape on purpose. Blankness and an empty list element are
	// distinct mistakes with distinct fixes, and a single shared phrase would let a fix for
	// one satisfy the rows of the other — the same reason the empty table refuses to accept
	// TASK-211's "requires a value".
	degenerate := []struct {
		what       string
		args       []string
		wantFlag   string
		wantPhrase string
	}{
		// Blank: non-empty by len, empty by content. --mode and --env already failed loudly
		// downstream ("mode ' ' not found"); these rows move the refusal to the argument
		// layer so all four flags answer the same way rather than three by accident.
		{"a blank --mode", []string{"--mode= "}, "--mode", "requires a non-blank value"},
		{"a blank --env", []string{"--env= "}, "--env", "requires a non-blank value"},
		{"a blank --tag", []string{"--tag= "}, "--tag", "requires a non-blank value"},
		{"a tab as --exclude-tag", []string{"--exclude-tag=\t"}, "--exclude-tag", "requires a non-blank value"},
		{"a blank next token", []string{"--mode", " "}, "--mode", "requires a non-blank value"},
		// A lone separator. Every list spelling, for the reason the empty table lists every
		// alias: the split happens in the case arms, so a fix written into one arm leaves
		// the other open.
		{"--tag=,", []string{"--tag=,"}, "--tag", "requires non-empty tags"},
		{"--tags=,", []string{"--tags=,"}, "--tags", "requires non-empty tags"},
		{"-T=,", []string{"-T=,"}, "-T", "requires non-empty tags"},
		{"--exclude-tag=,", []string{"--exclude-tag=,"}, "--exclude-tag", "requires non-empty tags"},
		{"--exclude-tags=,", []string{"--exclude-tags=,"}, "--exclude-tags", "requires non-empty tags"},
		// A hole in an otherwise real list. This is the row the card originally called
		// harmless; it is harmless for --tag and not for --exclude-tag, and neither of them
		// is something a user can have meant.
		{"a hole in a --tag list", []string{"--tag=a,,b"}, "--tag", "requires non-empty tags"},
		{"a hole in an --exclude-tag list", []string{"--exclude-tag=a,,b"}, "--exclude-tag", "requires non-empty tags"},
		{"a blank element", []string{"--tag=a, ,b"}, "--tag", "requires non-empty tags"},
	}
	for _, tc := range degenerate {
		t.Run(tc.what, func(t *testing.T) {
			dir := writeRestartProbeConfig(t)

			err := restartCmd.RunE(restartCmd, tc.args)
			// Errorf, not Fatalf, and that is the point of the row rather than a style
			// choice: a Fatalf here returns before the marker check, so the failure a
			// missing refusal produces would read "want an error, got nil" for every flag
			// alike and say nothing about what the build did instead. With the check
			// reached, the unfixed build separates into its two harms in the log —
			// --exclude-tag=, reports `ran [s1_stop s1_up s2_stop s2_up]`, --tag=, reports
			// nothing at all — which is the distinction the card got wrong by reading only
			// the second one.
			if err == nil {
				t.Errorf("restart %v: want an error, got nil", tc.args)
			} else {
				if !strings.Contains(err.Error(), tc.wantPhrase) {
					t.Errorf("restart %v: %q does not say %q", tc.args, err, tc.wantPhrase)
				}
				if !strings.Contains(err.Error(), tc.wantFlag) {
					t.Errorf("restart %v: %q does not name %s", tc.args, err, tc.wantFlag)
				}
			}
			if got := ranMarkers(t, dir); len(got) != 0 {
				t.Errorf("restart %v: nothing should have run, ran %v", tc.args, got)
			}
		})
	}

	// The control has to be a comma list: every row above is satisfied by a build that
	// refuses commas outright. Use declared tags so TASK-214's unknown-tag rejection
	// cannot satisfy this control; excluding both entries proves both comma values parsed.
	t.Run("but a real comma list is still taken", func(t *testing.T) {
		dir := writeRestartTaggedPlanProbeConfig(t)

		if err := restartCmd.RunE(restartCmd, []string{"--exclude-tag=web,db"}); err != nil {
			t.Fatalf("restart --exclude-tag=web,db: %v", err)
		}
		if got := ranMarkers(t, dir); len(got) != 0 {
			t.Errorf("restart --exclude-tag=web,db: both declared tags must exclude their entries, ran %v", got)
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

// TestParseDvaFlags_RejectedValueIsStillConsumed pins the invariant the comment in
// TestParseDvaFlags_MissingValue calls worth asserting, for the one spelling that used to
// violate it. `--mode ""` was rejected without advancing i, so the loop re-read the empty
// token and appended it to filtered: parseDvaFlags(["--mode","","s1"]) returned ["", "s1"],
// the value of a recognised flag sitting in what passthrough callers hand to docker.
//
// It was unreachable — err is set and every caller returns on it — which is why the first
// version of the fix left it and described it. A review declined that: unreachable-by-what-
// the-callers-do-next is a property of six other functions, not of this one. Asserting it
// here is what turns the argument into a test. TASK-213.
//
// Every flag gets a row, and that is the whole point of the table. The first version of
// this test asserted only --mode, the one spelling that had actually leaked. A review
// sabotaged the fix by moving `i += n` back inside `if ok` for --env, --tag and
// --exclude-tag — three of the four lines the commit consists of — and the package stayed
// green: `ok internal/cli 9.2s`, zero failures, with ["--env","","s1"] leaking ["","s1"]
// again. A fix is pinned at the granularity it was written, not at the granularity of the
// example that motivated it: four near-identical case arms are four chances to revert one.
func TestParseDvaFlags_RejectedValueIsStillConsumed(t *testing.T) {
	// Each flag is spelled with its own case arm in parseDvaFlags, so each needs its own
	// row; the aliases share an arm with the long form and are covered by it.
	for _, tc := range []struct {
		name string
		flag string
	}{
		{"--mode", "--mode"},
		{"--env", "--env"},
		{"--tag", "--tag"},
		{"--exclude-tag", "--exclude-tag"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := []string{tc.flag, "", "s1"}
			_, _, _, _, filtered, err := parseDvaFlags(args)
			if err == nil {
				t.Errorf("parseDvaFlags(%q) returned no error, want an empty-value error", args)
			}
			// s1 survives and "" does not: the flag's value was consumed even though it was
			// refused, and the positional argument that followed it is untouched. Asserted
			// with Errorf above so this runs either way — a build that both accepts the
			// value and leaks it should report both, not just the first.
			if len(filtered) != 1 || filtered[0] != "s1" {
				t.Errorf("parseDvaFlags(%q): filtered = %q, want [\"s1\"] — the refused value leaked into passthrough args", args, filtered)
			}
		})
	}
}

// TestParseDvaFlags_FirstBadFlagIsReported pins the `if err == nil` guards inside takeValue
// and takeList. They exist so the message names the flag the user has to fix first, rather
// than whichever bad flag happens to sit last on the line.
//
// It is here because a review deleted both guards — making the last error win — and the
// package stayed green: no existing row used two bad flags, so nothing could tell the
// orders apart. A property stated in a comment and asserted nowhere is a property that
// survives exactly until someone reads the comment as decoration. TASK-213.
func TestParseDvaFlags_FirstBadFlagIsReported(t *testing.T) {
	// Two different flags, two different rules (blank value vs empty-after-splitting), so
	// the two messages cannot be confused for one another.
	args := []string{"--mode=", "--tag=,"}
	_, _, _, _, _, err := parseDvaFlags(args)
	if err == nil {
		t.Fatalf("parseDvaFlags(%q) returned no error, want the first flag's error", args)
	}
	if !strings.Contains(err.Error(), "--mode") {
		t.Errorf("parseDvaFlags(%q): err = %v, want the FIRST bad flag (--mode) named, not the last", args, err)
	}
	if strings.Contains(err.Error(), "--tag") {
		t.Errorf("parseDvaFlags(%q): err = %v, names --tag; the later flag overwrote the first error", args, err)
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
