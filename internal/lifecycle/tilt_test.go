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

func TestTiltPlugin_Name(t *testing.T) {
	p := &TiltPlugin{}
	if p.Name() != "tilt" {
		t.Errorf("expected 'tilt', got %q", p.Name())
	}
}

func TestTiltPlugin_Up_NilConfig(t *testing.T) {
	p := &TiltPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Tilt: nil},
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

func TestTiltPlugin_Down_NilConfig(t *testing.T) {
	p := &TiltPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Tilt: nil},
		Logger: slog.Default(),
	}

	err := p.Down(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTiltPlugin_Stop_NilConfig(t *testing.T) {
	p := &TiltPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Tilt: nil},
		Logger: slog.Default(),
	}

	err := p.Stop(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTiltPlugin_Status_NilConfig(t *testing.T) {
	tmpDir := t.TempDir()

	p := &TiltPlugin{}
	pctx := &PluginContext{
		Entry:     &config.LifecycleEntry{Name: "tilt-svc"},
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

func TestTiltPlugin_DryRun_Up(t *testing.T) {
	p := &TiltPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Name: "tilt-svc",
			Tilt: &config.TiltPluginConfig{Dir: "."},
		},
		Env:       env,
		ConfigDir: "/tmp",
		DryRun:    true,
		Logger:    slog.Default(),
	}

	result, err := p.Up(context.Background(), pctx)
	if err != nil {
		t.Fatalf("dry-run up should not fail: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result from dry-run")
	}
}

func TestTiltPlugin_DryRun_Down(t *testing.T) {
	p := &TiltPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Name: "tilt-svc",
			Tilt: &config.TiltPluginConfig{Dir: "."},
		},
		Env:       env,
		ConfigDir: "/tmp",
		DryRun:    true,
		Logger:    slog.Default(),
	}

	err := p.Down(context.Background(), pctx)
	if err != nil {
		t.Fatalf("dry-run down should not fail: %v", err)
	}
}

func TestTiltPlugin_Status_NoPidFile(t *testing.T) {
	tmpDir := t.TempDir()

	p := &TiltPlugin{}
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

func TestTiltPlugin_Status_StalePid(t *testing.T) {
	tmpDir := t.TempDir()
	pidDir := filepath.Join(tmpDir, config.DotDirName, config.PidsDirName)
	os.MkdirAll(pidDir, 0755)

	// Write a PID that doesn't exist (very high PID)
	os.WriteFile(filepath.Join(pidDir, "stale.pid"), []byte("999999999"), 0644)

	p := &TiltPlugin{}
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

func TestTiltPlugin_Status_RunningPid(t *testing.T) {
	tmpDir := t.TempDir()
	pidDir := filepath.Join(tmpDir, config.DotDirName, config.PidsDirName)
	os.MkdirAll(pidDir, 0755)

	// Write our own PID — we are definitely running
	pid := os.Getpid()
	os.WriteFile(filepath.Join(pidDir, "self.pid"), []byte(strconv.Itoa(pid)), 0644)

	p := &TiltPlugin{}
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
