package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/lifecycle"
)

// No version: key. It declares the *minimum dva version*, not a schema version, so a value
// like "1.0" fails the load — and that failure is fatal inside mustLoadConfig (os.Exit(1)),
// which takes the whole test binary down with no output to explain it.
const dryRunHaltConfig = `applications:
  sleeper:
    description: stand-in for a running app
    default_runner: native
    runners:
      native:
        run: sleep 300
`

// standInApp writes a config and a pidfile pointing at a live process, in a temp cwd.
//
// The process is made a group leader (Setpgid) because HaltApps signals the process *group*
// (syscall.Kill(-pid, ...)). A plain exec.Command inherits the test binary's group, so a
// regression would signal the test runner itself rather than the stand-in — the assertion
// would not fail, it would take the suite with it.
func standInApp(t *testing.T) (dir string, pid int) {
	t.Helper()
	dir = t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "dva.yml"), []byte(dryRunHaltConfig), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".sb", "dva", "pids"), 0o755); err != nil {
		t.Fatalf("mkdir pids: %v", err)
	}

	helper := exec.Command("sleep", "300")
	helper.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := helper.Start(); err != nil {
		t.Fatalf("start stand-in: %v", err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(-helper.Process.Pid, syscall.SIGKILL)
		_ = helper.Process.Kill()
		_, _ = helper.Process.Wait()
	})

	pidFile := filepath.Join(dir, ".sb", "dva", "pids", "app-sleeper.pid")
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(helper.Process.Pid)), 0o644); err != nil {
		t.Fatalf("write pidfile: %v", err)
	}
	logFile := filepath.Join(dir, ".sb", "dva", "logs", "app-sleeper.log")
	if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}
	if err := os.WriteFile(logFile, []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write logfile: %v", err)
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

	return dir, helper.Process.Pid
}

// TestDryRunHaltPathsDoNotSignal drives the four commands that accepted --dry-run and halted
// the app anyway (TASK-166), through their RunE rather than through the lifecycle layer.
//
// TASK-153 fixed `app restart` and its review (M2) noted the gap this closes: a lifecycle-level
// test of HaltAppsDryRun proves the simulator works, not that any command calls it. Deleting
// the `if dryRun` branch in app.go or compose.go leaves every such test green.
//
// dryRun is set directly instead of relying on flag parsing because RunE is invoked here
// without cobra's Execute, which is what would normally populate the persistent flag. That is
// faithful to the defect: the flag always reached the global (the task measured it doing so):
// what was missing was the branch reading it.
func TestDryRunHaltPathsDoNotSignal(t *testing.T) {
	cases := []struct {
		name string
		cmd  *cobra.Command
		args []string
		// down and clean delete the pidfile for real; stop leaves it either way, so it is
		// only a signal for the paths that would remove it.
		deletesState bool
	}{
		// app stop/down leave flag parsing to cobra, so by the time RunE runs, --dry-run has
		// been consumed into the global and args holds app names only. Passing it here
		// instead makes it an unknown app name: validateAppNames rejects it and RunE returns
		// before reaching the branch under test. That mistake produced four passing-looking
		// empty-output failures on the first run of this test, which is why the error is
		// asserted below rather than discarded.
		{"app stop", appStopCmd, nil, false},
		{"app down", appDownCmd, nil, true},
		// stop/down set DisableFlagParsing, so they receive the raw argv and parse it
		// themselves via parseDvaFlags.
		{"dva stop", stopCmd, []string{"--dry-run"}, false},
		{"dva down", downCmd, []string{"--dry-run"}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir, pid := standInApp(t)

			old := dryRun
			dryRun = true
			t.Cleanup(func() { dryRun = old })

			oldErr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w
			err := tc.cmd.RunE(tc.cmd, tc.args)
			w.Close()
			os.Stderr = oldErr
			var buf bytes.Buffer
			buf.ReadFrom(r)
			out := buf.String()

			// Asserted, not discarded: an early return makes every assertion below pass
			// vacuously — the process is trivially alive if nothing ran. The first draft
			// swallowed this and reported "said nothing about being a preview" for a
			// command that never reached the halt at all.
			if err != nil {
				t.Fatalf("%s --dry-run: %v\n%s", tc.name, err, out)
			}
			// The load-bearing assertion.
			if !lifecycle.IsProcessRunning(pid) {
				t.Fatalf("%s --dry-run killed the stand-in process (pid %d); output was:\n%s",
					tc.name, pid, out)
			}
			if !strings.Contains(out, "dry-run") {
				t.Errorf("%s --dry-run said nothing about being a preview:\n%s", tc.name, out)
			}
			// "stopped app sleeper" is what the real path prints. A preview claiming a
			// completed action is the symptom the task opened with.
			if strings.Contains(out, "stopped app sleeper") {
				t.Errorf("%s --dry-run reported a completed stop:\n%s", tc.name, out)
			}
			if tc.deletesState {
				for _, f := range []string{
					filepath.Join(dir, ".sb", "dva", "pids", "app-sleeper.pid"),
					filepath.Join(dir, ".sb", "dva", "logs", "app-sleeper.log"),
				} {
					if _, err := os.Stat(f); err != nil {
						t.Errorf("%s --dry-run deleted %s", tc.name, filepath.Base(f))
					}
				}
			}
		})
	}
}

// TestCleanDryRunKeepsProvisionMarkers covers the one non-halt site in the same command.
//
// `dva clean --volumes` calls clearProvisionMarkers, which had no dry-run branch, so the
// command previewed its stack and app halves while deleting these files for real.
//
// It drives cleanCmd.RunE, not the helper pair. The first draft asserted on provisionMarkers
// and clearProvisionMarkers directly and would have stayed green with the `if dryRun` guard
// in compose.go deleted — the same registration gap TASK-140 hit, found the same way, by
// reverting the change and watching the test pass. clearProvisionMarkers runs before
// anything reaches docker, so RunE's later error is irrelevant here and is discarded.
func TestCleanDryRunKeepsProvisionMarkers(t *testing.T) {
	dir, _ := standInApp(t)
	markerDir := filepath.Join(dir, ".sb", "dva")
	marker := filepath.Join(markerDir, "provisioned-default")
	if err := os.WriteFile(marker, []byte("x"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	// --volumes only. This test set --force until TASK-170, not because the case needed it
	// but because the confirmation prompt stood between --dry-run and its own preview and
	// --force was the only way past. Adding it back to make this test pass again would mean
	// the prompt had returned.
	if err := cleanCmd.Flags().Set("volumes", "true"); err != nil {
		t.Fatalf("set --volumes: %v", err)
	}
	// Cobra flags live on the package-level command, so they outlast this test.
	t.Cleanup(func() { _ = cleanCmd.Flags().Set("volumes", "false") })

	old := dryRun
	dryRun = true
	t.Cleanup(func() { dryRun = old })

	oldErr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	done := make(chan string, 1)
	go func() {
		var b bytes.Buffer
		b.ReadFrom(r)
		done <- b.String()
	}()
	_ = cleanCmd.RunE(cleanCmd, nil)
	w.Close()
	os.Stderr = oldErr
	out := <-done

	if _, err := os.Stat(marker); err != nil {
		t.Errorf("clean --volumes --dry-run deleted the provision marker; output was:\n%s", out)
	}
	if !strings.Contains(out, "would delete provision marker") {
		t.Errorf("clean --dry-run did not name the marker it would delete:\n%s", out)
	}
}

// TestProvisionMarkersMatchesWhatClearDeletes pins the probe-only variant to the real one.
//
// provisionMarkers exists so the dry-run preview does not re-derive the "provisioned-"
// prefix and drift from the deletion it describes; this fails if either side changes alone.
func TestProvisionMarkersMatchesWhatClearDeletes(t *testing.T) {
	dir := t.TempDir()
	markerDir := filepath.Join(dir, ".sb", "dva")
	if err := os.MkdirAll(markerDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	names := []string{"provisioned-default", "provisioned-ci", "not-a-marker"}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(markerDir, n), []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", n, err)
		}
	}

	listed := provisionMarkers(dir)
	if len(listed) != 2 {
		t.Fatalf("provisionMarkers = %v, want the 2 provisioned-* entries", listed)
	}
	// Listing must not delete — the whole point of the probe-only variant.
	for _, m := range listed {
		if _, err := os.Stat(m); err != nil {
			t.Fatalf("provisionMarkers removed %s, which it was only asked to name", m)
		}
	}

	clearProvisionMarkers(dir)
	for _, m := range listed {
		if _, err := os.Stat(m); err == nil {
			t.Errorf("clearProvisionMarkers kept %s, which provisionMarkers named", m)
		}
	}
	// The unrelated file survives: neither half may widen to everything in .sb/dva.
	if _, err := os.Stat(filepath.Join(markerDir, "not-a-marker")); err != nil {
		t.Error("clearProvisionMarkers deleted a file that is not a provision marker")
	}
	if got := provisionMarkers(dir); len(got) != 0 {
		t.Errorf("provisionMarkers after clear = %v, want none", got)
	}
}
