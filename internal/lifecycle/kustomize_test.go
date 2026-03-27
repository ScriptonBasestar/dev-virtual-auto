package lifecycle

import (
	"context"
	"log/slog"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestKustomizePlugin_Name(t *testing.T) {
	p := &KustomizePlugin{}
	if p.Name() != "kustomize" {
		t.Errorf("expected 'kustomize', got %q", p.Name())
	}
}

func TestKustomizePlugin_Up_NilConfig(t *testing.T) {
	p := &KustomizePlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Kustomize: nil},
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

func TestKustomizePlugin_Down_NilConfig(t *testing.T) {
	p := &KustomizePlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Kustomize: nil},
		Logger: slog.Default(),
	}

	err := p.Down(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKustomizePlugin_Stop_NilConfig(t *testing.T) {
	p := &KustomizePlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Kustomize: nil},
		Logger: slog.Default(),
	}

	err := p.Stop(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKustomizePlugin_Status_NilConfig(t *testing.T) {
	p := &KustomizePlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Kustomize: nil},
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

func TestKustomizePlugin_DryRun_Up(t *testing.T) {
	p := &KustomizePlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Kustomize: &config.KustomizePluginConfig{
				Dir: "overlays/dev",
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

func TestKustomizePlugin_DryRun_Down(t *testing.T) {
	p := &KustomizePlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Kustomize: &config.KustomizePluginConfig{
				Dir: "overlays/dev",
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

func TestKustomizePlugin_BuildArgs_WithContext(t *testing.T) {
	p := &KustomizePlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Kustomize: &config.KustomizePluginConfig{
				Dir:       "overlays/dev",
				Context:   "minikube",
				Namespace: "my-ns",
			},
		},
		Env:       env,
		ConfigDir: "/project",
		Logger:    slog.Default(),
	}

	_, args := p.buildArgs(pctx, []string{"apply", "-k", "/project/overlays/dev"})

	foundContext := false
	foundNamespace := false
	for i, a := range args {
		if a == "--context" && i+1 < len(args) && args[i+1] == "minikube" {
			foundContext = true
		}
		if a == "-n" && i+1 < len(args) && args[i+1] == "my-ns" {
			foundNamespace = true
		}
	}
	if !foundContext {
		t.Errorf("expected --context minikube in args: %v", args)
	}
	if !foundNamespace {
		t.Errorf("expected -n my-ns in args: %v", args)
	}
}

func TestKustomizePlugin_ResolveDir(t *testing.T) {
	p := &KustomizePlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")

	// Test relative path resolution
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Kustomize: &config.KustomizePluginConfig{
				Dir: "overlays/dev",
			},
		},
		Env:       env,
		ConfigDir: "/project",
		Logger:    slog.Default(),
	}

	dir := p.resolveDir(pctx)
	expected := "/project/overlays/dev"
	if dir != expected {
		t.Errorf("expected %q, got %q", expected, dir)
	}

	// Test absolute path stays unchanged
	pctx.Entry.Kustomize.Dir = "/absolute/path/overlays"
	dir = p.resolveDir(pctx)
	if dir != "/absolute/path/overlays" {
		t.Errorf("expected absolute path unchanged, got %q", dir)
	}
}
