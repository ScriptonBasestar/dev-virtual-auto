package lifecycle

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestKubectlPlugin_Name(t *testing.T) {
	p := &KubectlPlugin{}
	if p.Name() != "kubectl" {
		t.Errorf("expected 'kubectl', got %q", p.Name())
	}
}

func TestKubectlPlugin_Up_NilConfig(t *testing.T) {
	p := &KubectlPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Kubectl: nil},
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

func TestKubectlPlugin_Down_NilConfig(t *testing.T) {
	p := &KubectlPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Kubectl: nil},
		Logger: slog.Default(),
	}

	err := p.Down(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKubectlPlugin_Stop_NilConfig(t *testing.T) {
	p := &KubectlPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Kubectl: nil},
		Logger: slog.Default(),
	}

	err := p.Stop(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKubectlPlugin_Status_NilConfig(t *testing.T) {
	p := &KubectlPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Kubectl: nil},
		Logger: slog.Default(),
	}

	services, err := p.Status(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if services != nil {
		t.Errorf("expected nil services, got %v", services)
	}
}

func TestKubectlPlugin_DryRun_Up(t *testing.T) {
	p := &KubectlPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Kubectl: &config.KubectlPluginConfig{
				Manifests: []string{"deploy.yaml"},
				Namespace: "default",
				Context:   "minikube",
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

func TestKubectlPlugin_DryRun_Down(t *testing.T) {
	p := &KubectlPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Kubectl: &config.KubectlPluginConfig{
				Manifests: []string{"deploy.yaml"},
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

func TestKubectlPlugin_BuildArgs_Basic(t *testing.T) {
	p := &KubectlPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Kubectl: &config.KubectlPluginConfig{
				Namespace: "staging",
				Context:   "prod-cluster",
			},
		},
		Env:       env,
		ConfigDir: "/project",
		Logger:    slog.Default(),
	}

	cmd, args := p.buildArgs(pctx, []string{"apply"})

	if cmd != "kubectl" {
		t.Errorf("expected command 'kubectl', got %q", cmd)
	}

	// Should contain: apply --context prod-cluster -n staging
	if len(args) < 5 {
		t.Fatalf("expected at least 5 args, got %v", args)
	}
	if args[0] != "apply" {
		t.Errorf("expected first arg 'apply', got %q", args[0])
	}

	foundContext := false
	foundNamespace := false
	for i, a := range args {
		if a == "--context" && i+1 < len(args) && args[i+1] == "prod-cluster" {
			foundContext = true
		}
		if a == "-n" && i+1 < len(args) && args[i+1] == "staging" {
			foundNamespace = true
		}
	}
	if !foundContext {
		t.Errorf("expected --context prod-cluster in args: %v", args)
	}
	if !foundNamespace {
		t.Errorf("expected -n staging in args: %v", args)
	}
}

func TestKubectlPlugin_ManifestArgs(t *testing.T) {
	p := &KubectlPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Kubectl: &config.KubectlPluginConfig{
				Manifests: []string{"deploy.yaml", "/abs/path/svc.yaml"},
			},
		},
		Env:       env,
		ConfigDir: "/project",
		Logger:    slog.Default(),
	}

	args := p.manifestArgs(pctx)

	// Expected: [-f /project/deploy.yaml -f /abs/path/svc.yaml]
	if len(args) != 4 {
		t.Fatalf("expected 4 args, got %v", args)
	}
	if args[0] != "-f" {
		t.Errorf("expected '-f', got %q", args[0])
	}
	expected := filepath.Join("/project", "deploy.yaml")
	if args[1] != expected {
		t.Errorf("expected %q (resolved relative path), got %q", expected, args[1])
	}
	if args[2] != "-f" {
		t.Errorf("expected '-f', got %q", args[2])
	}
	if args[3] != "/abs/path/svc.yaml" {
		t.Errorf("expected absolute path '/abs/path/svc.yaml', got %q", args[3])
	}
}
