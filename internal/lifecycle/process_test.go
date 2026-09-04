package lifecycle

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestProcessPlugin_Name(t *testing.T) {
	p := &ProcessPlugin{}
	if p.Name() != "process" {
		t.Errorf("expected 'process', got %q", p.Name())
	}
}

func TestProcessPlugin_Up_NilConfig(t *testing.T) {
	p := &ProcessPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Process: nil},
		Logger: slog.Default(),
	}

	result, err := p.Up(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestProcessPlugin_DryRun(t *testing.T) {
	p := &ProcessPlugin{}
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Name:    "test-proc",
			Process: &config.ProcessPluginConfig{Command: "sleep 999"},
		},
		DryRun: true,
		Logger: slog.Default(),
	}

	result, err := p.Up(context.Background(), pctx)
	if err != nil {
		t.Fatalf("dry-run should not fail: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestProcessPlugin_Status_NoPidFile(t *testing.T) {
	tmpDir := t.TempDir()

	p := &ProcessPlugin{}
	pctx := &PluginContext{
		Entry:     &config.LifecycleEntry{Name: "ghost"},
		ConfigDir: tmpDir,
		Logger:    slog.Default(),
	}

	services, err := p.Status(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	if services[0].State != "stopped" {
		t.Errorf("expected 'stopped', got %q", services[0].State)
	}
}

func TestProcessPlugin_Status_StalePidFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidDir := filepath.Join(tmpDir, config.DotDirName, config.PidsDirName)
	os.MkdirAll(pidDir, 0755)

	// Write a PID that doesn't exist (very high PID)
	os.WriteFile(filepath.Join(pidDir, "stale.pid"), []byte("999999999"), 0644)

	p := &ProcessPlugin{}
	pctx := &PluginContext{
		Entry:     &config.LifecycleEntry{Name: "stale"},
		ConfigDir: tmpDir,
		Logger:    slog.Default(),
	}

	services, err := p.Status(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	if services[0].State != "stopped" {
		t.Errorf("expected 'stopped' for stale pid, got %q", services[0].State)
	}
}

func TestProcessPlugin_Status_RunningProcess(t *testing.T) {
	tmpDir := t.TempDir()
	pidDir := filepath.Join(tmpDir, config.DotDirName, config.PidsDirName)
	os.MkdirAll(pidDir, 0755)

	// Write our own PID — we are definitely running
	pid := os.Getpid()
	os.WriteFile(filepath.Join(pidDir, "self.pid"), []byte(strconv.Itoa(pid)), 0644)

	p := &ProcessPlugin{}
	pctx := &PluginContext{
		Entry:     &config.LifecycleEntry{Name: "self"},
		ConfigDir: tmpDir,
		Logger:    slog.Default(),
	}

	services, err := p.Status(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	if services[0].State != "running" {
		t.Errorf("expected 'running' for own PID, got %q", services[0].State)
	}
}

func TestProcessPlugin_StopProcess_NoPidFile(t *testing.T) {
	tmpDir := t.TempDir()

	p := &ProcessPlugin{}
	pctx := &PluginContext{
		Entry:     &config.LifecycleEntry{Name: "gone"},
		ConfigDir: tmpDir,
		Logger:    slog.Default(),
	}

	err := p.Down(context.Background(), pctx)
	if err != nil {
		t.Fatalf("stopping non-existent process should not error: %v", err)
	}
}

// TestProcessPlugin_Restart_LeavesProcessRunning reproduces TASK-294: Orchestrator.Restart
// (Stop then Up) stopping a process-plugin entry and reporting success while the entry is
// actually left stopped, because Up's "already running" PID-file check can observe the
// original process mid-shutdown and skip starting a replacement.
//
// The stack entry traps SIGTERM and takes ~0.3s to actually exit, faithfully modeling a
// process with a short graceful-shutdown delay — the case the bug report identifies as most
// likely to lose the race. Pre-fix, Restart returns almost immediately without waiting for
// the original process to exit, so by the time this test's fixed delay elapses the original
// process is dead and no replacement was ever started, leaving a dead PID on record. Post-fix
// this always observes a live process, because Stop/haltProcess now blocks until the original
// process has actually exited before Up runs.
func TestProcessPlugin_Restart_LeavesProcessRunning(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process plugin requires Unix process groups")
	}

	dir := t.TempDir()
	writeImportedPlanConfig(t, dir, `
version: "0.1.0"
stack:
  api:
    default_runner: process
    runners:
      process:
        command: "trap 'sleep 0.3; exit 0' TERM; while true; do sleep 0.05; done"
`)

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	env := config.NewEnvironment(nil, cfg.FileDir(), cfg.FileDir())
	orch := NewOrchestrator(cfg, env)
	ctx := context.Background()
	t.Cleanup(func() { _ = orch.Down(ctx, DownOptions{}) })

	if err := orch.Up(ctx, UpOptions{}); err != nil {
		t.Fatalf("Up: %v", err)
	}

	pidFile := filepath.Join(cfg.FileDir(), config.DotDirName, config.PidsDirName, "api.pid")
	origPID := readPIDFile(t, pidFile)
	if !IsProcessRunning(origPID) {
		t.Fatalf("process not running after Up (pid %d)", origPID)
	}

	if err := orch.Restart(ctx, UpOptions{}); err != nil {
		t.Fatalf("Restart: %v", err)
	}

	// Give the original process (which takes ~0.3s to honor SIGTERM) time to actually
	// finish exiting, so a false "already running" report from Up's PID-file check isn't
	// masked by the original process still being alive when this check runs.
	time.Sleep(500 * time.Millisecond)

	newPID := readPIDFile(t, pidFile)
	if !IsProcessRunning(newPID) {
		t.Fatalf("restart left the entry stopped: pid %d recorded in %s (was %d before restart) is not running",
			newPID, pidFile, origPID)
	}
}

func readPIDFile(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pid file %s: %v", path, err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("parse pid file %s: %v", path, err)
	}
	return pid
}

func TestIsProcessRunning(t *testing.T) {
	// Our own PID should be running
	if !IsProcessRunning(os.Getpid()) {
		t.Error("expected own PID to be running")
	}

	// Very high PID should not be running
	if IsProcessRunning(999999999) {
		t.Error("expected non-existent PID to not be running")
	}
}
