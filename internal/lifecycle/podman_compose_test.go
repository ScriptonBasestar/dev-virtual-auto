package lifecycle

import (
	"context"
	"log/slog"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestPodmanComposePlugin_Name(t *testing.T) {
	p := &PodmanComposePlugin{}
	if p.Name() != "podman-compose" {
		t.Errorf("expected 'podman-compose', got %q", p.Name())
	}
}

func TestPodmanComposePlugin_Up_NilConfig(t *testing.T) {
	p := &PodmanComposePlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{PodmanCompose: nil},
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

func TestPodmanComposePlugin_Down_NilConfig(t *testing.T) {
	p := &PodmanComposePlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{PodmanCompose: nil},
		Logger: slog.Default(),
	}

	err := p.Down(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPodmanComposePlugin_Stop_NilConfig(t *testing.T) {
	p := &PodmanComposePlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{PodmanCompose: nil},
		Logger: slog.Default(),
	}

	err := p.Stop(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPodmanComposePlugin_DryRun_Up(t *testing.T) {
	p := &PodmanComposePlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			PodmanCompose: &config.PodmanComposePluginConfig{},
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

func TestPodmanComposePlugin_DryRun_Down(t *testing.T) {
	p := &PodmanComposePlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			PodmanCompose: &config.PodmanComposePluginConfig{},
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

func TestPodmanComposePlugin_BuildArgs_Default(t *testing.T) {
	p := &PodmanComposePlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			PodmanCompose: &config.PodmanComposePluginConfig{},
		},
		Env:       env,
		ConfigDir: "/project",
		Logger:    slog.Default(),
	}

	cmd, args := p.buildArgs(pctx, []string{"up", "-d"})

	if cmd != "podman-compose" {
		t.Errorf("expected command 'podman-compose', got %q", cmd)
	}
	if len(args) < 2 {
		t.Fatalf("expected at least 2 args, got %v", args)
	}
	if args[0] != "up" {
		t.Errorf("expected first arg 'up', got %q", args[0])
	}
	if args[1] != "-d" {
		t.Errorf("expected second arg '-d', got %q", args[1])
	}
}

func TestPodmanComposePlugin_BuildArgs_WithFiles(t *testing.T) {
	p := &PodmanComposePlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			PodmanCompose: &config.PodmanComposePluginConfig{
				Files: []string{"docker-compose.yml", "docker-compose.override.yml"},
			},
		},
		Env:       env,
		ConfigDir: "/project",
		Logger:    slog.Default(),
	}

	_, args := p.buildArgs(pctx, []string{"up"})

	// Should contain -f flags for each file
	foundFiles := 0
	for i, a := range args {
		if a == "-f" && i+1 < len(args) {
			foundFiles++
		}
	}
	if foundFiles != 2 {
		t.Errorf("expected 2 file flags, got %d (args: %v)", foundFiles, args)
	}
}

func TestPodmanComposePlugin_BuildArgs_WithProjectName(t *testing.T) {
	p := &PodmanComposePlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			PodmanCompose: &config.PodmanComposePluginConfig{
				ProjectName: "myproject",
			},
		},
		Env:       env,
		ConfigDir: "/project",
		Logger:    slog.Default(),
	}

	_, args := p.buildArgs(pctx, []string{"up"})

	found := false
	for i, a := range args {
		if a == "--project-name" && i+1 < len(args) && args[i+1] == "myproject" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected --project-name myproject in args: %v", args)
	}
}
