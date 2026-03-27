package lifecycle

import (
	"context"
	"log/slog"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestVagrantPlugin_Name(t *testing.T) {
	p := &VagrantPlugin{}
	if p.Name() != "vagrant" {
		t.Errorf("expected 'vagrant', got %q", p.Name())
	}
}

func TestVagrantPlugin_Up_NilConfig(t *testing.T) {
	p := &VagrantPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Vagrant: nil},
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

func TestVagrantPlugin_Down_NilConfig(t *testing.T) {
	p := &VagrantPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Vagrant: nil},
		Logger: slog.Default(),
	}

	err := p.Down(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVagrantPlugin_Stop_NilConfig(t *testing.T) {
	p := &VagrantPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Vagrant: nil},
		Logger: slog.Default(),
	}

	err := p.Stop(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVagrantPlugin_Status_NilConfig(t *testing.T) {
	p := &VagrantPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Vagrant: nil},
		Logger: slog.Default(),
	}

	services, err := p.Status(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if services != nil {
		t.Error("expected nil services for nil config")
	}
}

func TestVagrantPlugin_DryRun_Up(t *testing.T) {
	p := &VagrantPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Vagrant: &config.VagrantPluginConfig{
				Dir: "vm",
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

func TestVagrantPlugin_DryRun_Down(t *testing.T) {
	p := &VagrantPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Vagrant: &config.VagrantPluginConfig{
				Dir: "vm",
			},
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

func TestVagrantPlugin_BuildArgs_NoMachine(t *testing.T) {
	p := &VagrantPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Vagrant: &config.VagrantPluginConfig{
				Dir: "vm",
			},
		},
		Env:       env,
		ConfigDir: "/project",
		Logger:    slog.Default(),
	}

	args := p.buildArgs(pctx, []string{"up"})
	if len(args) != 1 {
		t.Errorf("expected 1 arg, got %v", args)
	}
	if args[0] != "up" {
		t.Errorf("expected 'up', got %q", args[0])
	}
}

func TestVagrantPlugin_BuildArgs_WithMachine(t *testing.T) {
	p := &VagrantPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Vagrant: &config.VagrantPluginConfig{
				Dir:     "vm",
				Machine: "web",
			},
		},
		Env:       env,
		ConfigDir: "/project",
		Logger:    slog.Default(),
	}

	args := p.buildArgs(pctx, []string{"up"})
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %v", args)
	}
	if args[0] != "up" {
		t.Errorf("expected 'up', got %q", args[0])
	}
	if args[1] != "web" {
		t.Errorf("expected 'web', got %q", args[1])
	}
}

func TestVagrantPlugin_ResolveDir(t *testing.T) {
	p := &VagrantPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")

	// Test relative path resolution
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Vagrant: &config.VagrantPluginConfig{
				Dir: "vm",
			},
		},
		Env:       env,
		ConfigDir: "/project",
		Logger:    slog.Default(),
	}

	dir := p.resolveDir(pctx)
	expected := "/project/vm"
	if dir != expected {
		t.Errorf("expected %q, got %q", expected, dir)
	}

	// Test absolute path stays unchanged
	pctx.Entry.Vagrant.Dir = "/absolute/vm"
	dir = p.resolveDir(pctx)
	if dir != "/absolute/vm" {
		t.Errorf("expected absolute path unchanged, got %q", dir)
	}
}
