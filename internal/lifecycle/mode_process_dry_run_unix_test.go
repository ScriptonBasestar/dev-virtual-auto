//go:build !windows

package lifecycle

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// captureStderr runs fn with os.Stderr redirected to a pipe.
// Do not use t.Parallel in callers: stderr is process-global.
//
// Moved here from app_health_required_test.go, which was deleted with the AppManager
// (docs/43). This file is now its only caller.
func captureStderr(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	old := os.Stderr
	os.Stderr = w
	runErr := fn()
	_ = w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String(), runErr
}

// modeStandIn starts a live process and writes its pid where signalModeProcesses looks.
//
// Group leader (Setpgid) on purpose: the pidfile is signalled as a process *group*
// (Kill(-pid, ...)), so a stand-in that inherited the test binary's group would send
// SIGTERM to the test runner rather than fail an assertion.
//
// The returned channel closes once the stand-in has exited and been reaped, and it is the
// only trustworthy liveness signal here. IsProcessRunning is Signal(0), which succeeds
// against a zombie, and the stand-in is a direct child of the test binary — so a stand-in
// that really was SIGTERMed still reads as "running" until something calls Wait. That is
// not hypothetical: the first version of this test asserted with IsProcessRunning alone,
// and its central "dry-run killed the stand-in" line stayed silent when the dry-run branch
// was reverted, leaving only the weaker output assertions to catch the regression.
// Production never sees the zombie — the process signalled there was started by an earlier
// `dva up` and re-parented to init, which reaps it — so this is a cost of the harness, paid
// with one short negative window per case.
//
// FileDir() is filepath.Dir("") == "." for a Config that was not loaded from a file, and
// there is no exported setter for the path, so the fixture is anchored with t.Chdir —
// the same shape portowner_test.go and app_start_exit_test.go use.
func modeStandIn(t *testing.T, name string) (pidFile string, pid int, reaped <-chan struct{}) {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)

	helper := exec.Command("sleep", "300")
	helper.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := helper.Start(); err != nil {
		t.Fatalf("start stand-in: %v", err)
	}
	done := make(chan struct{})
	go func() {
		_, _ = helper.Process.Wait()
		close(done)
	}()
	t.Cleanup(func() {
		_ = syscall.Kill(-helper.Process.Pid, syscall.SIGKILL)
		<-done
	})

	pidDir := filepath.Join(dir, config.DotDirName, config.PidsDirName)
	if err := os.MkdirAll(pidDir, 0o755); err != nil {
		t.Fatalf("mkdir pids: %v", err)
	}
	pidFile = filepath.Join(pidDir, name+".pid")
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(helper.Process.Pid)), 0o644); err != nil {
		t.Fatalf("write pidfile: %v", err)
	}
	return pidFile, helper.Process.Pid, done
}

// assertStillAlive fails if the stand-in exited, which is the whole claim of --dry-run.
//
// Proving an event did *not* happen needs a window: SIGTERM against `sleep` is delivered
// and reaped in well under a millisecond, so 300ms is generous while costing the suite
// about a second across the cases that use it. Checked before the window too, so a kill
// already reaped fails immediately rather than after the wait.
func assertStillAlive(t *testing.T, pid int, reaped <-chan struct{}, out string) {
	t.Helper()
	select {
	case <-reaped:
		t.Fatalf("dry-run killed the stand-in (pid %d); output was:\n%s", pid, out)
	case <-time.After(300 * time.Millisecond):
	}
	if !IsProcessRunning(pid) {
		t.Fatalf("dry-run killed the stand-in (pid %d); output was:\n%s", pid, out)
	}
}

func modeOrchestrator() *Orchestrator {
	cfg := &config.Config{
		HealthChecks: map[string]config.HealthCheckConfig{"worker": {Start: "sleep 300"}},
		Modes:        map[string]config.ModeConfig{"dev": {HealthChecks: []string{"worker"}}},
	}
	return NewOrchestrator(cfg, config.NewEnvironment(nil, ".", "."))
}

// TestSignalModeProcessesDryRun covers the halt half of the mode health_check processes —
// a path distinct from applications:, reached by `dva stop --mode X` and `dva down --mode X`
// before either touches the stack or the apps.
//
// It had no dry-run branch and no test of any kind, so `dva stop --mode dev --dry-run` sent
// a real SIGTERM and printed "[-] stopped worker (pid N)": output byte-comparable with the
// run carrying no flag at all. Its sibling startModeProcesses has honoured opts.DryRun since
// it was written, which is what let the omission survive — the `up` half looked done, so the
// pair did too. Found by TASK-166's review after the six sites it had already fixed.
func TestSignalModeProcessesDryRun(t *testing.T) {
	for _, tc := range []struct {
		name string
		// haltModeProcesses (stop semantics) keeps the pidfile; stopModeProcesses (down
		// semantics) removes it. Only the second should preview a deletion.
		removePID bool
	}{
		{"halt: stop semantics, pidfile kept", false},
		{"stop: down semantics, pidfile removed", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pidFile, pid, reaped := modeStandIn(t, "worker")
			o := modeOrchestrator()

			out, _ := captureStderr(t, func() error {
				o.signalModeProcesses("dev", tc.removePID, true)
				return nil
			})

			// The load-bearing assertion, and the reason modeStandIn returns a channel.
			assertStillAlive(t, pid, reaped, out)
			if !strings.Contains(out, "would stop worker") {
				t.Errorf("dry-run did not announce the stop it withheld:\n%s", out)
			}
			// "[-] stopped" is what the real path prints. A preview reporting a completed
			// kill is the symptom the task opened with.
			if strings.Contains(out, "[-] stopped") {
				t.Errorf("dry-run reported a completed stop:\n%s", out)
			}
			if _, err := os.Stat(pidFile); err != nil {
				t.Errorf("dry-run deleted the pidfile")
			}
			if got := strings.Contains(out, "would delete"); got != tc.removePID {
				t.Errorf("delete preview = %v, want %v:\n%s", got, tc.removePID, out)
			}
		})
	}

	// The negative control. Without it the assertions above would also pass against a
	// signalModeProcesses that had been gutted to do nothing at all — the dry-run branch
	// has to be what withholds the signal, not the absence of a signal anywhere.
	t.Run("no dry-run still signals", func(t *testing.T) {
		pidFile, pid, reaped := modeStandIn(t, "worker")
		o := modeOrchestrator()

		out, _ := captureStderr(t, func() error {
			o.signalModeProcesses("dev", true, false)
			return nil
		})
		select {
		case <-reaped:
		case <-time.After(5 * time.Second):
			t.Fatalf("the real path left the stand-in running (pid %d):\n%s", pid, out)
		}
		if IsProcessRunning(pid) {
			t.Errorf("the real path left the stand-in running:\n%s", out)
		}
		if !strings.Contains(out, "[-] stopped worker") {
			t.Errorf("the real path did not report the stop:\n%s", out)
		}
		if _, err := os.Stat(pidFile); err == nil {
			t.Error("the real path kept the pidfile it was asked to remove")
		}
	})
}

// TestHaltAndStopModeProcessesForwardDryRun pins the two named wrappers to the shared body.
//
// signalModeProcesses is unexported and called from nowhere but these two, so a dry-run
// branch inside it is only reachable if both wrappers forward the flag. Down and Stop call
// the wrappers as their first statement, before the entry filtering that can return early,
// which is why the flag is threaded rather than checked at the call sites.
func TestHaltAndStopModeProcessesForwardDryRun(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(o *Orchestrator)
	}{
		{"haltModeProcesses", func(o *Orchestrator) { o.haltModeProcesses("dev", true) }},
		{"stopModeProcesses", func(o *Orchestrator) { o.stopModeProcesses("dev", true) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, pid, reaped := modeStandIn(t, "worker")
			o := modeOrchestrator()

			out, _ := captureStderr(t, func() error {
				tc.call(o)
				return nil
			})
			assertStillAlive(t, pid, reaped, out)
			if !strings.Contains(out, "would stop worker") {
				t.Errorf("%s produced no preview:\n%s", tc.name, out)
			}
		})
	}
}
