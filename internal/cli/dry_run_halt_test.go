package cli

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/logger"
	"github.com/spf13/cobra"
)

// No version: key. It declares the *minimum dva version*, not a schema version, so a value
// like "1.0" fails the load — and that failure is fatal inside mustLoadConfig (os.Exit(1)),
// which takes the whole test binary down with no output to explain it.
//
// A script entry, because it is the one plugin whose teardown is a command the test can
// observe without a container runtime: the markers below exist if and only if the real
// path ran.
const dryRunHaltConfig = `stack:
  sleeper:
    default_runner: script
    runners:
      script:
        up: "echo up"
        stop: "touch STOPPED"
        down: "touch DOWNED"
`

// captureCommandStderr runs fn with os.Stderr redirected to a pipe.
// Do not use t.Parallel in callers: stderr is process-global.
//
// logger.Init is called *inside* the redirect because it binds os.Stderr into the slog
// handler at construction (logger.go:23), so a logger built before the swap keeps writing to
// the real stderr and the dry-run line never reaches the buffer. Production has the same
// ordering — PersistentPreRun runs logger.Init (root.go:46) and then RunE — which the direct
// RunE call here skips. Both halves of the output matter and land in different places: the
// "[lifecycle] stopping" banner is fmt.Fprintf(os.Stderr) (orchestrator.go:210) and reads the
// variable each time, while the "dry-run" line is pctx.Logger and does not.
func captureCommandStderr(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	oldErr, oldLog, oldDefault := os.Stderr, logger.Log, slog.Default()
	os.Stderr = w
	logger.Init(false, false)

	runErr := fn()

	_ = w.Close()
	os.Stderr, logger.Log = oldErr, oldLog
	slog.SetDefault(oldDefault)
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String(), runErr
}

// standInStack writes the config in a temp cwd and returns it.
func standInStack(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "dva.yml"), []byte(dryRunHaltConfig), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Chdir(dir)

	// loadConfig and loadEnv memoise into the cfg/env package globals (root.go:301, :357),
	// which live for the whole test binary. Without this reset the second subtest gets the
	// first one's Config, whose FileDir() points at a TempDir that t.Cleanup has already
	// removed — so every path lookup lands in a directory that no longer exists, nothing is
	// found, nothing is printed, and the failure reads as "the command said nothing" rather
	// than "the command was looking somewhere else". Restored rather than merely cleared, so
	// this does not disturb tests that populate the globals deliberately.
	oldCfg, oldEnv := cfg, env
	cfg, env = nil, nil
	t.Cleanup(func() { cfg, env = oldCfg, oldEnv })

	return dir
}

// TestDryRunHaltPathsDoNotSignal drives the commands that accepted --dry-run and tore down
// anyway (TASK-166), through their RunE rather than through the lifecycle layer.
//
// The point is the same one TASK-153's review (M2) made: a lifecycle-level test of the
// dry-run branch proves the simulator works, not that any command reaches it. Here that is
// the `DryRun: dryRun` field in the StopOptions/DownOptions literal — drop it and every
// plugin-level dry-run test stays green while `dva stop --dry-run` tears the stack down.
//
// This was an application-manager test until docs/43. `dva stop`/`dva down` used to halt
// apps through their own RunE branch, separately from the orchestrator, which is what let
// the stack half preview while the app half sent a real SIGTERM in one command. That second
// half is gone with `applications:`, so the stand-in is now a script entry and the evidence
// is a marker file rather than a live pid.
//
// dryRun is set directly instead of relying on flag parsing because RunE is invoked here
// without cobra's Execute, which is what would normally populate the persistent flag. That is
// faithful to the defect: the flag always reached the global (the task measured it doing so);
// what was missing was the branch reading it.
func TestDryRunHaltPathsDoNotSignal(t *testing.T) {
	cases := []struct {
		name string
		cmd  *cobra.Command
		args []string
		// The marker the real path would leave behind. Absence is the load-bearing
		// assertion; the "dry-run" line below only proves the command said something.
		marker string
	}{
		// stop/down set DisableFlagParsing, so they receive the raw argv and parse it
		// themselves via parseDvaFlags.
		{"dva stop", stopCmd, []string{"--dry-run"}, "STOPPED"},
		{"dva down", downCmd, []string{"--dry-run"}, "DOWNED"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := standInStack(t)

			old := dryRun
			dryRun = true
			t.Cleanup(func() { dryRun = old })

			out, err := captureCommandStderr(t, func() error {
				return tc.cmd.RunE(tc.cmd, tc.args)
			})

			// Asserted, not discarded: an early return makes every assertion below pass
			// vacuously — the marker is trivially absent if nothing ran. The first draft
			// swallowed this and reported "said nothing about being a preview" for a
			// command that never reached the teardown at all.
			if err != nil {
				t.Fatalf("%s --dry-run: %v\n%s", tc.name, err, out)
			}
			// The load-bearing assertion.
			if _, statErr := os.Stat(filepath.Join(dir, tc.marker)); statErr == nil {
				t.Fatalf("%s --dry-run ran the teardown script (%s exists); output was:\n%s",
					tc.name, tc.marker, out)
			}
			if !strings.Contains(out, "dry-run") {
				t.Errorf("%s --dry-run said nothing about being a preview:\n%s", tc.name, out)
			}
		})
	}
}

// TestDryRunHaltPathsWithoutTheFlagDoAct is the non-vacuity control for the test above.
//
// Every assertion there is an absence, and absences pass for the wrong reason: a script
// entry that never runs, a marker written somewhere else, a teardown the orchestrator
// skipped for an unrelated reason. This runs the same two commands with dryRun off and
// requires the markers to appear, so "no marker" means "the preview held" rather than
// "this fixture cannot produce one".
func TestDryRunHaltPathsWithoutTheFlagDoAct(t *testing.T) {
	cases := []struct {
		name   string
		cmd    *cobra.Command
		marker string
	}{
		{"dva stop", stopCmd, "STOPPED"},
		{"dva down", downCmd, "DOWNED"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := standInStack(t)

			old := dryRun
			dryRun = false
			t.Cleanup(func() { dryRun = old })

			if err := tc.cmd.RunE(tc.cmd, nil); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if _, err := os.Stat(filepath.Join(dir, tc.marker)); err != nil {
				t.Fatalf("%s did not run its teardown script (%s absent) — the dry-run test "+
					"above is passing vacuously: %v", tc.name, tc.marker, err)
			}
		})
	}
}
