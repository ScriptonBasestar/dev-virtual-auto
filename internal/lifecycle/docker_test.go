package lifecycle

import (
	"context"
	"log/slog"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestDockerPlugin_Name(t *testing.T) {
	p := &DockerPlugin{}
	if p.Name() != "docker" {
		t.Errorf("expected 'docker', got %q", p.Name())
	}
}

func TestDockerPlugin_Up_NilConfig(t *testing.T) {
	p := &DockerPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Docker: nil},
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

func TestDockerPlugin_Down_NilConfig(t *testing.T) {
	p := &DockerPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Docker: nil},
		Logger: slog.Default(),
	}

	err := p.Down(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDockerPlugin_Stop_NilConfig(t *testing.T) {
	p := &DockerPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Docker: nil},
		Logger: slog.Default(),
	}

	err := p.Stop(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDockerPlugin_Status_NilConfig(t *testing.T) {
	p := &DockerPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Docker: nil},
		Logger: slog.Default(),
	}

	services, err := p.Status(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if services != nil {
		t.Errorf("expected nil services for nil config, got %v", services)
	}
}

func TestDockerPlugin_DryRun_Up(t *testing.T) {
	p := &DockerPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Name: "test-container",
			Docker: &config.DockerPluginConfig{
				Image: "nginx:latest",
				Name:  "test-container",
				Ports: []string{"8080:80"},
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

func TestDockerPlugin_DryRun_Down(t *testing.T) {
	p := &DockerPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Name: "test-container",
			Docker: &config.DockerPluginConfig{
				Image: "nginx:latest",
				Name:  "test-container",
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

func TestDockerPlugin_BuildArgs_Basic(t *testing.T) {
	p := &DockerPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Name: "test-container",
			Docker: &config.DockerPluginConfig{
				Image:   "postgres:15",
				Name:    "test-db",
				Ports:   []string{"5432:5432"},
				Volumes: []string{"/data:/var/lib/postgresql/data"},
				Env: map[string]string{
					"POSTGRES_PASSWORD": "secret",
				},
			},
		},
		Env:       env,
		ConfigDir: "/project",
		Logger:    slog.Default(),
	}

	args := p.buildRunArgs(pctx)

	assertContainsFlag(t, args, "--name", "test-db")
	assertContainsFlag(t, args, "-p", "5432:5432")
	assertContainsFlag(t, args, "-v", "/data:/var/lib/postgresql/data")
	assertContainsFlag(t, args, "-e", "POSTGRES_PASSWORD=secret")

	// Image should be the last argument
	last := args[len(args)-1]
	if last != "postgres:15" {
		t.Errorf("expected last arg to be image 'postgres:15', got %q", last)
	}
}

func TestDockerPlugin_BuildArgs_WithOptions(t *testing.T) {
	p := &DockerPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Name: "test-container",
			Docker: &config.DockerPluginConfig{
				Image:   "redis:7",
				Name:    "test-redis",
				Ports:   []string{"6379:6379"},
				Options: []string{"--restart", "unless-stopped", "--memory", "512m"},
			},
		},
		Env:       env,
		ConfigDir: "/project",
		Logger:    slog.Default(),
	}

	args := p.buildRunArgs(pctx)

	// Check that options appear in the args
	foundRestart := false
	foundMemory := false
	for i, a := range args {
		if a == "--restart" && i+1 < len(args) && args[i+1] == "unless-stopped" {
			foundRestart = true
		}
		if a == "--memory" && i+1 < len(args) && args[i+1] == "512m" {
			foundMemory = true
		}
	}
	if !foundRestart {
		t.Errorf("expected --restart unless-stopped in args: %v", args)
	}
	if !foundMemory {
		t.Errorf("expected --memory 512m in args: %v", args)
	}
}

// assertContainsFlag checks that args contain flag followed by the expected value.
func assertContainsFlag(t *testing.T, args []string, flag, value string) {
	t.Helper()
	for i, a := range args {
		if a == flag && i+1 < len(args) && args[i+1] == value {
			return
		}
	}
	t.Errorf("expected %s %s in args: %v", flag, value, args)
}
