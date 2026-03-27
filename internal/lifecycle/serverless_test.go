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

func TestServerlessPlugin_Name(t *testing.T) {
	p := &ServerlessPlugin{}
	if p.Name() != "serverless" {
		t.Errorf("expected 'serverless', got %q", p.Name())
	}
}

func TestServerlessPlugin_Up_NilConfig(t *testing.T) {
	p := &ServerlessPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Serverless: nil},
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

func TestServerlessPlugin_Down_NilConfig(t *testing.T) {
	p := &ServerlessPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Serverless: nil},
		Logger: slog.Default(),
	}

	err := p.Down(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServerlessPlugin_Stop_NilConfig(t *testing.T) {
	p := &ServerlessPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Serverless: nil},
		Logger: slog.Default(),
	}

	err := p.Stop(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServerlessPlugin_Status_NilConfig(t *testing.T) {
	tmpDir := t.TempDir()

	p := &ServerlessPlugin{}
	pctx := &PluginContext{
		Entry:     &config.LifecycleEntry{Name: "sls-api", Serverless: nil},
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

func TestServerlessPlugin_DryRun_Up(t *testing.T) {
	p := &ServerlessPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Name: "sls-api",
			Serverless: &config.ServerlessPluginConfig{
				Dir:  "functions",
				Port: 4000,
			},
		},
		Env:       env,
		ConfigDir: "/project",
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

func TestServerlessPlugin_Status_NoPidFile(t *testing.T) {
	tmpDir := t.TempDir()

	p := &ServerlessPlugin{}
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

func TestServerlessPlugin_Status_StalePid(t *testing.T) {
	tmpDir := t.TempDir()
	pidDir := filepath.Join(tmpDir, config.DotDirName, "pids")
	os.MkdirAll(pidDir, 0755)

	// Write a PID that doesn't exist (very high PID)
	os.WriteFile(filepath.Join(pidDir, "stale.pid"), []byte("999999999"), 0644)

	p := &ServerlessPlugin{}
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

func TestServerlessPlugin_Status_RunningPid(t *testing.T) {
	tmpDir := t.TempDir()
	pidDir := filepath.Join(tmpDir, config.DotDirName, "pids")
	os.MkdirAll(pidDir, 0755)

	// Write our own PID — we are definitely running
	pid := os.Getpid()
	os.WriteFile(filepath.Join(pidDir, "self.pid"), []byte(strconv.Itoa(pid)), 0644)

	p := &ServerlessPlugin{}
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
