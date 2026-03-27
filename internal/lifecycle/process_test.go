package lifecycle

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"testing"

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
	pidDir := filepath.Join(tmpDir, config.DotDirName, "pids")
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
	pidDir := filepath.Join(tmpDir, config.DotDirName, "pids")
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
