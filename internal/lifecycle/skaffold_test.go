package lifecycle

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestSkaffoldPlugin_Name(t *testing.T) {
	p := &SkaffoldPlugin{}
	if p.Name() != "skaffold" {
		t.Errorf("expected 'skaffold', got %q", p.Name())
	}
}

func TestSkaffoldPlugin_Up_NilConfig(t *testing.T) {
	p := &SkaffoldPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Skaffold: nil},
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

func TestSkaffoldPlugin_Down_NilConfig(t *testing.T) {
	p := &SkaffoldPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Skaffold: nil},
		Logger: slog.Default(),
	}

	err := p.Down(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSkaffoldPlugin_Stop_NilConfig(t *testing.T) {
	p := &SkaffoldPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Skaffold: nil},
		Logger: slog.Default(),
	}

	err := p.Stop(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSkaffoldPlugin_Status_NilConfig(t *testing.T) {
	p := &SkaffoldPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Name: "skaffold-svc"},
		Logger: slog.Default(),
	}

	services, err := p.Status(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	if services[0].State != "unknown" {
		t.Errorf("expected 'unknown', got %q", services[0].State)
	}
	if services[0].Health != "unknown" {
		t.Errorf("expected health 'unknown', got %q", services[0].Health)
	}
}

func TestSkaffoldPlugin_DryRun_Up(t *testing.T) {
	p := &SkaffoldPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Name:     "skaffold-svc",
			Skaffold: &config.SkaffoldPluginConfig{},
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

func TestSkaffoldPlugin_DryRun_Down(t *testing.T) {
	p := &SkaffoldPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Name:     "skaffold-svc",
			Skaffold: &config.SkaffoldPluginConfig{},
		},
		Env:       env,
		ConfigDir: "/project",
		DryRun:    true,
		Logger:    slog.Default(),
	}

	err := p.Down(context.Background(), pctx)
	if err != nil {
		t.Fatalf("dry-run down should not fail: %v", err)
	}
}

func TestSkaffoldPlugin_BuildArgs_Basic(t *testing.T) {
	p := &SkaffoldPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Skaffold: &config.SkaffoldPluginConfig{},
		},
		Env:       env,
		ConfigDir: "/project",
		Logger:    slog.Default(),
	}

	cmd, args := p.buildArgs(pctx, []string{"run"})

	if cmd != "skaffold" {
		t.Errorf("expected command 'skaffold', got %q", cmd)
	}
	if len(args) != 1 || args[0] != "run" {
		t.Errorf("expected args ['run'], got %v", args)
	}
}

func TestSkaffoldPlugin_BuildArgs_WithConfig(t *testing.T) {
	p := &SkaffoldPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Skaffold: &config.SkaffoldPluginConfig{
				Config: "skaffold.yaml",
			},
		},
		Env:       env,
		ConfigDir: "/project",
		Logger:    slog.Default(),
	}

	_, args := p.buildArgs(pctx, []string{"run"})

	foundConfig := false
	for i, a := range args {
		if a == "-f" && i+1 < len(args) {
			expected := filepath.Join("/project", "skaffold.yaml")
			if args[i+1] != expected {
				t.Errorf("expected config path %q, got %q", expected, args[i+1])
			}
			foundConfig = true
			break
		}
	}
	if !foundConfig {
		t.Errorf("expected -f flag in args: %v", args)
	}
}

func TestSkaffoldPlugin_BuildArgs_WithProfile(t *testing.T) {
	p := &SkaffoldPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Skaffold: &config.SkaffoldPluginConfig{
				Profile: "dev",
			},
		},
		Env:       env,
		ConfigDir: "/project",
		Logger:    slog.Default(),
	}

	_, args := p.buildArgs(pctx, []string{"run"})

	foundProfile := false
	for i, a := range args {
		if a == "-p" && i+1 < len(args) && args[i+1] == "dev" {
			foundProfile = true
			break
		}
	}
	if !foundProfile {
		t.Errorf("expected -p dev in args: %v", args)
	}
}

func TestSkaffoldPlugin_BuildArgs_WithExtraArgs(t *testing.T) {
	p := &SkaffoldPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Skaffold: &config.SkaffoldPluginConfig{
				Args: []string{"--tail", "--cleanup"},
			},
		},
		Env:       env,
		ConfigDir: "/project",
		Logger:    slog.Default(),
	}

	_, args := p.buildArgs(pctx, []string{"run"})

	// Should contain the extra args at the end
	if len(args) < 3 {
		t.Fatalf("expected at least 3 args, got %v", args)
	}
	if args[len(args)-2] != "--tail" || args[len(args)-1] != "--cleanup" {
		t.Errorf("expected trailing args [--tail --cleanup], got %v", args)
	}
}
