// Package cli — regression tests for TASK-172.
//
// TASK-145 stopped DVA's own flags reaching docker, but left one spelling out of scope: a
// *malformed* boolean. parseDvaFlags dropped it into `filtered` "for the caller's own
// unknown-flag rejection to name", which 7 of its 12 call sites have. `dva build` is not one
// of them — its leftovers are docker's argv. Measured 2026-08-03 on 53cdba2 with an
// argv-recording docker shim:
//
//	dva build --debug=notabool  -> compose -f <file> build --debug=notabool
//	dva build --debug=true      -> compose -f <file> build            (control: consumed)
//
// The tests below drive buildCmd.RunE specifically. TASK-145's table drives stack log,
// compose and logs, none of which reach compose.go's build path.
package cli

import (
	"strings"
	"testing"
)

// buildFixture is a compose stack with a build-capable entry, plus the argv-recording shim.
const buildFixtureYAML = `version: "0.1.44"
stack:
  infra:
    order: 1
    default_runner: compose
    runners:
      compose:
        files:
          - docker-compose.yml
`

// TestBuildRejectsMalformedBoolInsteadOfForwardingIt is the criterion test: the malformed
// token must not appear in docker's argv, and docker must not be run at all.
//
// Both halves matter. Asserting only "the token is absent" would also pass if the fix had
// silently dropped it and built anyway, which trades a confusing docker error for a build
// that ignored what the user typed.
func TestBuildRejectsMalformedBoolInsteadOfForwardingIt(t *testing.T) {
	for _, in := range [][]string{
		{"--debug=notabool"},
		{"--json=maybe"},
		{"--dry-run=perhaps"},
		// After a positional, where the token is furthest from the flag parser's start.
		{"web", "--debug=notabool"},
	} {
		t.Run(strings.Join(in, " "), func(t *testing.T) {
			argv := composePassthroughFixtureWith(t, buildFixtureYAML)

			err := buildCmd.RunE(buildCmd, in)
			if err == nil {
				t.Errorf("dva build %v returned nil — the malformed value was accepted", in)
			} else if !strings.Contains(err.Error(), "invalid boolean value") {
				t.Errorf("error = %q, want it to name the problem in DVA's words", err)
			}

			// Reached even when the check above failed, which is why it is not a Fatalf up
			// there. "Did DVA complain?" and "did the token reach docker?" are different
			// regressions: a change that restored the error while still forwarding would
			// satisfy the first. Restoring the original bug makes this the line that names
			// the leak, and a Fatalf above would have hidden it behind "returned nil".
			got := argv()
			if len(got) != 0 {
				t.Fatalf("docker was invoked %d time(s) despite the bad flag: %v", len(got), got)
			}
		})
	}
}

// TestBuildStillForwardsWhatIsDockersAndConsumesWhatIsDvas is criterion 4: the fix must not
// widen into the well-formed spellings TASK-145 measured, and must not start eating docker's
// own flags — `--no-cache` is the reason the rejection cannot live at this call site.
func TestBuildStillForwardsWhatIsDockersAndConsumesWhatIsDvas(t *testing.T) {
	cases := []struct {
		name string
		args []string
		// absent: DVA's, must be consumed before docker sees it.
		absent []string
		// present: docker's, must survive.
		present []string
	}{
		{"--debug=true is consumed", []string{"--debug=true"}, []string{"--debug=true", "--debug"}, []string{"build"}},
		{"--debug=false is consumed", []string{"--debug=false"}, []string{"--debug=false", "--debug"}, []string{"build"}},
		{"docker's own flags survive", []string{"--no-cache", "--pull"}, []string{"--debug"}, []string{"build", "--no-cache", "--pull"}},
		{"a service name survives", []string{"web"}, []string{"--debug"}, []string{"build", "web"}},
		{
			// Mixed: DVA's consumed, docker's forwarded, in one argv.
			name:    "mixed argv splits correctly",
			args:    []string{"--debug=true", "--no-cache", "web"},
			absent:  []string{"--debug=true", "--debug"},
			present: []string{"build", "--no-cache", "web"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			argv := composePassthroughFixtureWith(t, buildFixtureYAML)

			if err := buildCmd.RunE(buildCmd, tc.args); err != nil {
				t.Fatalf("dva build %v returned %v", tc.args, err)
			}

			got := argv()
			t.Logf("docker invocations (%d): %v", len(got), got)
			if len(got) != 1 {
				t.Fatalf("docker invoked %d times, want 1 — no argv to assert on", len(got))
			}
			line := got[0]

			for _, a := range tc.absent {
				if hasArg(line, a) {
					t.Errorf("%s reached docker in %q — DVA's flag became docker's", a, line)
				}
			}
			for _, p := range tc.present {
				if !hasArg(line, p) {
					t.Errorf("%s did not survive into %q — the passthrough is broken", p, line)
				}
			}
		})
	}
}

// TestTeardownDoesNotAdviseRunningDvasOwnFlag covers criterion 3. teardownCommon read any
// leftover as a service name and quoted it into its suggestion, so `dva down --debug=notabool`
// answered "Use 'dva stack down --debug=notabool'" — advice that cannot work, because the
// token was never a service name.
func TestTeardownDoesNotAdviseRunningDvasOwnFlag(t *testing.T) {
	t.Run("malformed bool is named, not suggested", func(t *testing.T) {
		composePassthroughFixtureWith(t, buildFixtureYAML)

		_, _, _, _, _, err := teardownCommon([]string{"--debug=notabool"}, "down")
		if err == nil {
			t.Fatal("teardownCommon accepted a malformed boolean")
		}
		if !strings.Contains(err.Error(), "invalid boolean value") {
			t.Errorf("error = %q, want it to name the malformed value", err)
		}
		if strings.Contains(err.Error(), "dva stack down") {
			t.Errorf("error still suggests a selective teardown built from DVA's own flag: %q", err)
		}
	})

	t.Run("an unknown flag is not called a service name", func(t *testing.T) {
		composePassthroughFixtureWith(t, buildFixtureYAML)

		_, _, _, _, _, err := teardownCommon([]string{"--bogus"}, "down")
		if err == nil {
			t.Fatal("teardownCommon accepted an unknown flag")
		}
		// The old message quoted it as a name: "Use 'dva stack down --bogus'". Running that
		// fails for exactly the same reason, so the advice was worse than none.
		if strings.Contains(err.Error(), "dva stack down --bogus") {
			t.Errorf("error suggests a command that cannot work: %q", err)
		}
		if !strings.Contains(err.Error(), "unknown flag") {
			t.Errorf("error = %q, want it to say the token is a flag", err)
		}
	})

	t.Run("a real name still gets the selective hint", func(t *testing.T) {
		// The control. The selective-teardown suggestion is useful when the leftover really is
		// a name, and a fix that removed it for everything would be a regression.
		composePassthroughFixtureWith(t, buildFixtureYAML)

		_, _, _, _, _, err := teardownCommon([]string{"infra"}, "down")
		if err == nil {
			t.Fatal("teardownCommon accepted a service name for a whole-stack teardown")
		}
		if !strings.Contains(err.Error(), "dva stack down infra") {
			t.Errorf("error = %q, want it to keep suggesting the selective form", err)
		}
	})
}
