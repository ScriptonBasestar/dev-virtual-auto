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
	missing := []struct {
		what string
		args []string
	}{
		{"a trailing --mode", []string{"--mode"}},
		{"--mode before the terminator", []string{"--mode", "--"}},
		{"the short form -M", []string{"-M"}},
		{"a repeatable --tag", []string{"--tag"}},
		{"--tag before the terminator", []string{"--tag", "--"}},
		{"--env", []string{"--env"}},
		{"--exclude-tag", []string{"--exclude-tag"}},
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
