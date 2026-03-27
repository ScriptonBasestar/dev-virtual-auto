package lifecycle

import (
	"context"
	"log/slog"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestComposePlugin_Name(t *testing.T) {
	p := &ComposePlugin{}
	if p.Name() != "compose" {
		t.Errorf("expected 'compose', got %q", p.Name())
	}
}

func TestComposePlugin_Up_NilConfig(t *testing.T) {
	p := &ComposePlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Compose: nil},
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

func TestComposePlugin_Down_NilConfig(t *testing.T) {
	p := &ComposePlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Compose: nil},
		Logger: slog.Default(),
	}

	err := p.Down(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestComposePlugin_Stop_NilConfig(t *testing.T) {
	p := &ComposePlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Compose: nil},
		Logger: slog.Default(),
	}

	err := p.Stop(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestComposePlugin_BuildArgs_Default(t *testing.T) {
	p := &ComposePlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Compose: &config.ComposePluginConfig{},
		},
		Env:       env,
		ConfigDir: "/project",
		Logger:    slog.Default(),
	}

	cmd, args := p.buildArgs(pctx, []string{"up", "-d"})

	if cmd != "docker" {
		t.Errorf("expected command 'docker', got %q", cmd)
	}
	if len(args) < 3 {
		t.Fatalf("expected at least 3 args, got %v", args)
	}
	if args[0] != "compose" {
		t.Errorf("expected first arg 'compose', got %q", args[0])
	}
}

func TestComposePlugin_BuildArgs_CustomCommand(t *testing.T) {
	p := &ComposePlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Compose: &config.ComposePluginConfig{
				Command: "podman compose",
			},
		},
		Env:       env,
		ConfigDir: "/project",
		Logger:    slog.Default(),
	}

	cmd, args := p.buildArgs(pctx, []string{"up"})

	if cmd != "podman" {
		t.Errorf("expected command 'podman', got %q", cmd)
	}
	if args[0] != "compose" {
		t.Errorf("expected first arg 'compose', got %q", args[0])
	}
}

func TestComposePlugin_BuildArgs_WithFiles(t *testing.T) {
	p := &ComposePlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Compose: &config.ComposePluginConfig{
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

func TestComposePlugin_BuildArgs_WithProjectName(t *testing.T) {
	p := &ComposePlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Compose: &config.ComposePluginConfig{
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

func TestComposePlugin_DryRun_Up(t *testing.T) {
	p := &ComposePlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Compose: &config.ComposePluginConfig{},
		},
		Env:       env,
		ConfigDir: "/project",
		DryRun:    true,
		Wait:      true,
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

func TestComposePlugin_DryRun_Down(t *testing.T) {
	p := &ComposePlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Compose: &config.ComposePluginConfig{},
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

func TestComposePlugin_UpOptions_NoWait(t *testing.T) {
	p := &ComposePlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Compose: &config.ComposePluginConfig{
				UpOptions: []string{"-d", "--wait"},
			},
		},
		Env:       env,
		ConfigDir: "/project",
		DryRun:    true,
		Wait:      false, // no-wait should filter out --wait
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
