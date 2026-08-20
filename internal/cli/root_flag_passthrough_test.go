// Package cli — regression tests for TASK-092.
//
// The commands that hand their arguments to docker (`dva compose`, `dva logs`,
// `dva stack log`) set DisableFlagParsing and appended args verbatim, so DVA's own root
// flags became docker's. Measured on 0.1.44:
//
//	dva --debug stack log infra --tail=5   -> docker compose … logs --debug infra --tail=5
//	dva --debug --json compose logs        -> docker compose … --debug --json logs
//	dva --debug logs --tail=5              -> docker compose … logs --debug --tail=5
//
// The middle one is the worst-positioned: those two land before the subcommand, so they
// are offered to `docker compose` itself rather than to `logs`.
package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// composePassthroughFixture writes a compose stack, puts a fake `docker` first on PATH and
// returns an accessor for the argv it was handed.
//
// forceSubprocess is the key to testing this at all. execComposePassthrough normally ends in
// ExecReplace → syscall.Exec, which would replace the *test binary*; the exec'd program's exit
// status then becomes the test's, and a broken assertion reports `ok` (the failure mode that
// cost TASK-094 two false-passing probes). forceSubprocess is the seam the hook wrapper already
// uses to keep the Go process alive, and execution_paths_test.go already drives it, so the
// passthrough runs as a subprocess and returns here.
//
// PATH is rebuilt as shim:/bin:/usr/bin rather than prepended to the caller's, and the test
// aborts unless `docker` resolves to the shim — a real docker must not be reachable.
func composePassthroughFixture(t *testing.T) func() []string {
	t.Helper()
	return composePassthroughFixtureWith(t, `version: "0.1.44"
stack:
  infra:
    order: 1
    default_runner: compose
    runners:
      compose:
        files:
          - docker-compose.yml
`)
}

// composePassthroughFixtureWith is the same fixture over a caller-supplied config, for tests
// where the stack's declaration shape is itself the thing under test.
func composePassthroughFixtureWith(t *testing.T, body string) func() []string {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "dva.yml"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}

	shimDir := filepath.Join(dir, "shim")
	if err := os.Mkdir(shimDir, 0755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "argv.log")
	shimPath := filepath.Join(shimDir, "docker")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + logPath + "\n"
	if err := os.WriteFile(shimPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shimDir+":/bin:/usr/bin")
	resolved, err := exec.LookPath("docker")
	if err != nil {
		t.Fatalf("docker does not resolve under the test PATH: %v", err)
	}
	if resolved != shimPath {
		t.Fatalf("docker resolves to %q, not the shim %q — refusing to run a real docker", resolved, shimPath)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	// These are package globals the commands read and write. Without the reset a later test
	// would inherit this one's debug/json and reuse an already-removed config dir.
	oldDebug, oldJSON, oldDryRun, oldForce := debug, jsonOutput, dryRun, forceSubprocess
	debug, jsonOutput, dryRun = false, false, false
	forceSubprocess = true
	cfg, env = nil, nil
	t.Cleanup(func() {
		os.Chdir(oldWd)
		cfg, env = nil, nil
		debug, jsonOutput, dryRun, forceSubprocess = oldDebug, oldJSON, oldDryRun, oldForce
	})

	return func() []string {
		data, err := os.ReadFile(logPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			t.Fatalf("read argv log: %v", err)
		}
		return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	}
}

// TestPassthroughDoesNotForwardRootFlags is TASK-092's criterion test. Each row drives a real
// RunE and asserts on the argv docker actually received.
//
// TASK-092 measured the leak on `dva stack log`, which is where the name it carried until now
// came from. That command is gone and `dva logs` is the surviving path — the same one, not
// merely a similar one: stack log stripped root flags with consumeRootPersistentFlags rather
// than parseDvaFlags, "because compose owns --dry-run on this path", and logsCmd does the same
// at compose.go:645. The row below that pins --dry-run surviving is the assertion that keeps
// that choice honest, so it had to move with the rest rather than be dropped as stack-specific.
func TestPassthroughDoesNotForwardRootFlags(t *testing.T) {
	cases := []struct {
		name string
		run  func(args []string) error
		args []string
		// absent: must not reach docker — these are DVA's flags.
		absent []string
		// present: must still reach docker, or the fix broke the passthrough it protects.
		present []string
	}{
		{
			// A name between DVA's flag and docker's, which the "logs strips --debug" row
			// below does not have. `infra` is a compose service here — the fixture declares
			// no plans — where under stack log it was an entry name resolved into -p/-f and
			// so never appeared in the argv. Asserted present rather than dropped: a strip
			// that overshot into the positional slot is the neighbouring bug to this one.
			name:    "logs strips --debug from between a name and docker's flags",
			run:     func(a []string) error { return logsCmd.RunE(logsCmd, a) },
			args:    []string{"--debug", "infra", "--tail=5", "--since=1h"},
			absent:  []string{"--debug"},
			present: []string{"logs", "infra", "--tail=5", "--since=1h"},
		},
		{
			name:    "compose strips --debug and --json",
			run:     func(a []string) error { return composeCmd.RunE(composeCmd, a) },
			args:    []string{"--debug", "--json", "logs", "--tail=5"},
			absent:  []string{"--debug", "--json"},
			present: []string{"logs", "--tail=5"},
		},
		{
			name:    "logs strips --debug",
			run:     func(a []string) error { return logsCmd.RunE(logsCmd, a) },
			args:    []string{"--debug", "--tail=5"},
			absent:  []string{"--debug"},
			present: []string{"logs", "--tail=5"},
		},
		{
			// The carve-out root.go documents: compose has its own --dry-run, so unlike
			// --debug/--json it must survive into the passthrough. This is why the fix uses
			// consumeRootPersistentFlags and not parseDvaFlags, which eats all three.
			//
			// Driven on `dva compose`, which is where the carve-out is observable, and
			// measured to be the only such place. Retargeting this row to `dva logs` — the
			// obvious heir, and the command consumeRootPersistentFlags' own comment named —
			// failed: logs is a hookable built-in, so wrapWithHooks replaces its RunE and
			// calls consumeDryRunFlag (hooks.go:28) ahead of the body, and the token is
			// DVA's before the passthrough sees it. That was already true of `dva logs` at
			// the time the comment was written; `dva stack log` was not hookable, so it was
			// the only command that ever demonstrated the claim, and deleting it would have
			// taken the claim's only evidence with it. root.go's comment is corrected to
			// name `dva compose` alone.
			name:    "--dry-run is still forwarded",
			run:     func(a []string) error { return composeCmd.RunE(composeCmd, a) },
			args:    []string{"logs", "--dry-run"},
			absent:  nil,
			present: []string{"logs", "--dry-run"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			argv := composePassthroughFixture(t)

			if err := tc.run(tc.args); err != nil {
				t.Fatalf("%v returned %v", tc.args, err)
			}

			got := argv()
			t.Logf("docker invocations (%d): %v", len(got), got)
			if len(got) != 1 {
				t.Fatalf("docker invoked %d times, want 1 — no argv to assert on", len(got))
			}
			line := got[0]

			for _, a := range tc.absent {
				// Word-boundary matching: the argv holds a temp path, and a bare
				// strings.Contains would also be satisfied by an unrelated substring.
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

// TestPassthroughRootFlagsStillTakeEffect is the other half: a fix that merely deleted --debug
// from the argv would pass every assertion above, so this asserts it is applied as it is
// consumed. (TASK-092 named it after `dva stack log`; see the note on the test above.)
//
// Note what this does and does not claim. In production `--debug` also survives a naive strip,
// because PersistentPreRun calls applyRootPersistentFlagsFromArgs on os.Args before any RunE
// runs — that is why the pre-fix binary still printed its `[debug] compose:` trace while
// leaking the flag. These tests call RunE directly, so PersistentPreRun never runs and the
// passthrough is judged on its own. That is deliberately stricter than production: the
// passthrough should not depend on something upstream having already read the flag.
func TestPassthroughRootFlagsStillTakeEffect(t *testing.T) {
	t.Run("--debug", func(t *testing.T) {
		composePassthroughFixture(t)
		if debug {
			t.Fatal("control failed: debug was already on before the command ran")
		}
		if err := logsCmd.RunE(logsCmd, []string{"--debug", "infra"}); err != nil {
			t.Fatalf("logs: %v", err)
		}
		if !debug {
			t.Error("--debug was consumed without being applied by the passthrough itself")
		}
	})

	t.Run("--json", func(t *testing.T) {
		composePassthroughFixture(t)
		if jsonOutput {
			t.Fatal("control failed: jsonOutput was already on before the command ran")
		}
		if err := logsCmd.RunE(logsCmd, []string{"--json"}); err != nil {
			t.Fatalf("logs: %v", err)
		}
		if !jsonOutput {
			t.Error("--json was consumed without being applied by the passthrough itself")
		}
	})
}

// TestConsumeRootPersistentFlags pins the helper's contract directly, including the parts
// that are not obvious: --dry-run is left alone, the `=value` form is applied and stripped
// (TASK-145), and the `--` terminator is consumed here rather than forwarded.
//
// It used to say "consumed here and nowhere else", which TASK-207 falsified: restartCmd now
// consumes one too, since it takes positional names and the token separates them from flags.
// The two do not overlap — this one strips the terminator from an argv handed to an external
// tool, restart's strips it from a list of entry names — but the uniqueness half was
// load-bearing for a reader deciding where to look, so the claim is now about this helper
// rather than about the repo.
func TestConsumeRootPersistentFlags(t *testing.T) {
	oldDebug, oldJSON := debug, jsonOutput
	t.Cleanup(func() { debug, jsonOutput = oldDebug, oldJSON })

	cases := []struct {
		name      string
		in        []string
		want      []string
		wantDebug bool
		wantJSON  bool
	}{
		{"nothing to consume", []string{"logs", "-f"}, []string{"logs", "-f"}, false, false},
		{"debug consumed", []string{"--debug", "logs"}, []string{"logs"}, true, false},
		{"json consumed", []string{"logs", "--json"}, []string{"logs"}, false, true},
		{"both consumed", []string{"--debug", "--json", "logs"}, []string{"logs"}, true, true},
		{"dry-run survives", []string{"logs", "--dry-run"}, []string{"logs", "--dry-run"}, false, false},
		// Was "=true form is left alone" until TASK-145, which is the defect this row now
		// forbids: the token reached docker and debug stayed off.
		{"=true form applied and stripped", []string{"--debug=true", "logs"}, []string{"logs"}, true, false},
		{"=false form applied and stripped", []string{"--debug=false", "logs"}, []string{"logs"}, false, false},
		{"=1 form, pflag's own spelling", []string{"--json=1", "logs"}, []string{"logs"}, false, true},
		// The terminator ends DVA's flags and is eaten with them, so the tail arrives as the
		// external program's own argv rather than behind a `--` it would read as positional.
		{"terminator ends the scan", []string{"--debug", "logs", "--", "--json", "--tail=5"},
			[]string{"logs", "--json", "--tail=5"}, true, false},
		{"a literal -- needs -- --", []string{"logs", "--", "--"}, []string{"logs", "--"}, false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			debug, jsonOutput = false, false
			got, err := consumeRootPersistentFlags(tc.in)
			if err != nil {
				t.Fatalf("consumeRootPersistentFlags(%v) = %v", tc.in, err)
			}
			if strings.Join(got, " ") != strings.Join(tc.want, " ") {
				t.Errorf("args = %v, want %v", got, tc.want)
			}
			if debug != tc.wantDebug {
				t.Errorf("debug = %v, want %v", debug, tc.wantDebug)
			}
			if jsonOutput != tc.wantJSON {
				t.Errorf("jsonOutput = %v, want %v", jsonOutput, tc.wantJSON)
			}
		})
	}
}

// hasArg reports whether the space-joined argv contains arg as a whole argument, so a check
// for "--debug" is not satisfied by "--debug-something" or by a path that embeds it.
func hasArg(line, arg string) bool {
	// Written by hand, not by `golangci-lint --fix`: modernize produced two overlapping edits
	// here (slicescontains and stringsseq) and golangci-lint skipped the file rather than merge
	// them. One candidate rewrite it printed reduced this body to a bare `return false`. That
	// would not have passed silently — the `present` assertions at the call site are positive,
	// and substituting `return false` fails 4 of 4 subtests — but it is why this one line was
	// written by hand and the other 46 findings were not.
	return slices.Contains(strings.Fields(line), arg)
}
