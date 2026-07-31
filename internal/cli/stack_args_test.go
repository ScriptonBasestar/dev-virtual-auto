// Package cli — regression tests for TASK-087.
//
// stack up/stop/down set DisableFlagParsing and read whatever parseDvaFlags leaves
// behind as stack entry NAMEs. A mistyped flag therefore became a name, matched
// nothing, and the command exited 0: `dva stack up infra --nowait` started infra with
// --wait still on, and `dva stack up --nowait` started nothing at all and reported
// success. These tests drive the real RunE bodies — the rejections return before the
// orchestrator is built, and the accepting cases run a `script:` stack whose hooks touch
// marker files, so "did anything actually run" is observable without docker.
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeStackArgsConfig creates a two-entry script stack in a temp dir and chdirs into
// it. Entry names are the ones the task measured, so the fixture and the CLI evidence
// in tasks/done/087 describe the same shapes.
func writeStackArgsConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	body := `version: "0.1.44"
stack:
  infra:
    order: 1
    script:
      up: touch infra_up
      stop: touch infra_stop
      down: touch infra_down
  cache:
    order: 2
    script:
      up: touch cache_up
      stop: touch cache_stop
      down: touch cache_down
`
	if err := os.WriteFile(filepath.Join(dir, "dva.yml"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	// loadConfig/loadEnv memoize into package globals, and parseDvaFlags writes dryRun.
	// Without a reset each test would reuse the previous test's (already-removed) dir.
	oldDryRun := dryRun
	cfg, env = nil, nil
	t.Cleanup(func() {
		os.Chdir(oldWd)
		cfg, env = nil, nil
		dryRun = oldDryRun
	})
	return dir
}

// TestStackRejectsUnknownArgs is the core guard. Every row exited 0 before the fix,
// having quietly discarded the argument it names.
func TestStackRejectsUnknownArgs(t *testing.T) {
	cases := []struct {
		name string
		sub  string
		args []string
		want []string // every substring the message must carry
	}{
		{"up: mistyped --no-wait", "up", []string{"infra", "--nowait", "--dry-run"},
			[]string{"unknown flag", "--nowait", "--no-wait"}},
		{"up: mistyped --force", "up", []string{"infra", "--forse", "--dry-run"},
			[]string{"unknown flag", "--forse", "--force"}},
		{"up: mistyped flag with no NAME empties the whole selection", "up", []string{"--nowait", "--dry-run"},
			[]string{"unknown flag", "--nowait"}},
		{"stop: mistyped flag", "stop", []string{"--nowiat", "--dry-run"},
			[]string{"unknown flag", "--nowiat"}},
		{"down: mistyped --volumes", "down", []string{"--volume", "--dry-run"},
			[]string{"unknown flag", "--volume", "--volumes"}},
		{"up: name matching no entry", "up", []string{"infr", "--dry-run"},
			[]string{"no such stack entry", "infr", "infra"}},
		{"stop: name matching no entry", "stop", []string{"nosuchentry", "--dry-run"},
			[]string{"no such stack entry", "nosuchentry"}},
		{"down: name matching no entry", "down", []string{"nosuchentry", "--dry-run"},
			[]string{"no such stack entry", "nosuchentry"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeStackArgsConfig(t)

			err := runStackSub(t, tc.sub, tc.args)
			if err == nil {
				t.Fatalf("dva stack %s %v returned nil; the argument was silently discarded and the command reported success", tc.sub, tc.args)
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("message does not mention %q:\n%s", want, err.Error())
				}
			}
			// Nothing may have run: a rejected invocation must not half-start the stack.
			for _, m := range []string{"infra_up", "cache_up", "infra_stop", "infra_down"} {
				if _, statErr := os.Stat(filepath.Join(dir, m)); statErr == nil {
					t.Errorf("marker %s exists; the command acted before rejecting the argument", m)
				}
			}
		})
	}
}

// TestStackAcceptsKnownArgsAndNames is the control that makes the test above mean
// something. Without it, "the typo returns an error" would also pass on a stack command
// that could never succeed — which is exactly the failure mode the first attempt at
// measuring TASK-087 hit, where --nosuchflag and a valid invocation both exited 0 and
// made the finding look vacuous in the other direction.
func TestStackAcceptsKnownArgsAndNames(t *testing.T) {
	cases := []struct {
		name       string
		sub        string
		args       []string
		wantMarker string
	}{
		{"up with its own flag and a real name", "up", []string{"infra", "--no-wait"}, "infra_up"},
		{"up with no NAME still starts everything", "up", []string{}, "cache_up"},
		{"stop with a real name", "stop", []string{"infra"}, "infra_stop"},
		{"down with -v and a real name", "down", []string{"cache", "-v"}, "cache_down"},
		{"down with --volumes and no NAME", "down", []string{"--volumes"}, "infra_down"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeStackArgsConfig(t)

			if err := runStackSub(t, tc.sub, tc.args); err != nil {
				t.Fatalf("dva stack %s %v returned %v; the new rejection is over-broad", tc.sub, tc.args, err)
			}
			if _, err := os.Stat(filepath.Join(dir, tc.wantMarker)); err != nil {
				t.Errorf("marker %s absent: the command returned nil without doing its work", tc.wantMarker)
			}
		})
	}
}

// TestStackLogKeepsForwardingUnknownFlags pins the one subcommand deliberately left
// out of the rejection. `stack log` builds a docker argv from its arguments verbatim, so
// --tail/--since/-f are docker's flags, not DVA's; rejecting them there would delete a
// working feature. Asserted through buildComposeArgs — the function stackLogCmd routes
// through — because running the command itself would exec docker.
func TestStackLogKeepsForwardingUnknownFlags(t *testing.T) {
	writeStackArgsConfig(t)
	c := mustLoadConfig()
	e := loadEnv(c)

	_, argv := mustComposeArgs(t, e, c, []string{"logs", "infra", "--tail=5", "--since=1h", "-f"})
	joined := strings.Join(argv, " ")
	for _, want := range []string{"--tail=5", "--since=1h", "-f"} {
		if !strings.Contains(joined, want) {
			t.Errorf("%s did not survive into the docker argv %q; `stack log` must forward what it does not recognise", want, joined)
		}
	}

	// The other half of the same distinction: `stack up` does not forward anything to
	// docker, so the identical flag is a typo there and must be rejected.
	if err := rejectUnknownFlags("stack up", "a stack entry name", []string{"--tail=5"}, []string{"--force"}); err == nil {
		t.Error("rejectUnknownFlags accepted --tail=5 for `stack up`, which has no passthrough to forward it to")
	}
}

// runStackSub invokes a stack subcommand's RunE directly. cobra's Execute would need
// os.Args surgery and would swallow the error into its own output; RunE is the seam the
// fix lives behind.
func runStackSub(t *testing.T, sub string, args []string) error {
	t.Helper()
	switch sub {
	case "up":
		return stackUpCmd.RunE(stackUpCmd, args)
	case "stop":
		return stackStopCmd.RunE(stackStopCmd, args)
	case "down":
		return stackDownCmd.RunE(stackDownCmd, args)
	}
	t.Fatalf("unknown subcommand %q", sub)
	return nil
}
