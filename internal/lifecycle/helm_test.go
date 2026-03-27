package lifecycle

import (
	"context"
	"log/slog"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestHelmPlugin_Name(t *testing.T) {
	p := &HelmPlugin{}
	if p.Name() != "helm" {
		t.Errorf("expected 'helm', got %q", p.Name())
	}
}

func TestHelmPlugin_Up_NilConfig(t *testing.T) {
	p := &HelmPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Helm: nil},
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

func TestHelmPlugin_Down_NilConfig(t *testing.T) {
	p := &HelmPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Helm: nil},
		Logger: slog.Default(),
	}

	err := p.Down(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHelmPlugin_Stop_NilConfig(t *testing.T) {
	p := &HelmPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Helm: nil},
		Logger: slog.Default(),
	}

	err := p.Stop(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHelmPlugin_Status_NilConfig(t *testing.T) {
	p := &HelmPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Helm: nil},
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

func TestHelmPlugin_DryRun_Up(t *testing.T) {
	p := &HelmPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Name: "test-release",
			Helm: &config.HelmPluginConfig{
				Chart:     "bitnami/redis",
				Release:   "my-redis",
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

func TestHelmPlugin_DryRun_Down(t *testing.T) {
	p := &HelmPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Name: "test-release",
			Helm: &config.HelmPluginConfig{
				Chart:     "bitnami/redis",
				Release:   "my-redis",
				Namespace: "default",
				Context:   "minikube",
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

func TestHelmPlugin_BuildArgs_Basic(t *testing.T) {
	p := &HelmPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Name: "test-release",
			Helm: &config.HelmPluginConfig{
				Chart:     "bitnami/redis",
				Release:   "my-redis",
				Namespace: "default",
				Context:   "minikube",
			},
		},
		Env:       env,
		ConfigDir: "/project",
		Logger:    slog.Default(),
	}

	cmd, args := p.buildArgs(pctx, []string{"upgrade", "--install", "my-redis", "bitnami/redis"})

	if cmd != "helm" {
		t.Errorf("expected command 'helm', got %q", cmd)
	}

	// Verify --kube-context is present
	foundContext := false
	for i, a := range args {
		if a == "--kube-context" && i+1 < len(args) && args[i+1] == "minikube" {
			foundContext = true
			break
		}
	}
	if !foundContext {
		t.Errorf("expected --kube-context minikube in args: %v", args)
	}

	// Verify -n namespace is present
	foundNs := false
	for i, a := range args {
		if a == "-n" && i+1 < len(args) && args[i+1] == "default" {
			foundNs = true
			break
		}
	}
	if !foundNs {
		t.Errorf("expected -n default in args: %v", args)
	}
}

func TestHelmPlugin_BuildArgs_WithValues(t *testing.T) {
	p := &HelmPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Name: "test-release",
			Helm: &config.HelmPluginConfig{
				Chart:     "bitnami/redis",
				Release:   "my-redis",
				Namespace: "default",
				Context:   "minikube",
				Values:    []string{"values.yaml", "values-prod.yaml"},
			},
		},
		Env:       env,
		ConfigDir: "/project",
		Logger:    slog.Default(),
	}

	// Simulate what Up() does: build args with -f for each values file
	args := []string{"upgrade", "--install", "my-redis", "bitnami/redis"}
	for _, vf := range pctx.Entry.Helm.Values {
		vf = pctx.Env.Interpolate(vf)
		if len(vf) > 0 && vf[0] != '/' {
			vf = pctx.ConfigDir + "/" + vf
		}
		args = append(args, "-f", vf)
	}

	cmd, finalArgs := p.buildArgs(pctx, args)

	if cmd != "helm" {
		t.Errorf("expected command 'helm', got %q", cmd)
	}

	// Count -f flags
	foundFiles := 0
	for i, a := range finalArgs {
		if a == "-f" && i+1 < len(finalArgs) {
			foundFiles++
		}
	}
	if foundFiles != 2 {
		t.Errorf("expected 2 -f flags, got %d (args: %v)", foundFiles, finalArgs)
	}
}

func TestHelmPlugin_BuildArgs_WithSet(t *testing.T) {
	p := &HelmPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Name: "test-release",
			Helm: &config.HelmPluginConfig{
				Chart:   "bitnami/redis",
				Release: "my-redis",
				Set: map[string]string{
					"replica.replicaCount": "3",
				},
			},
		},
		Env:       env,
		ConfigDir: "/project",
		Logger:    slog.Default(),
	}

	// Simulate what Up() does: build args with --set flags
	args := []string{"upgrade", "--install", "my-redis", "bitnami/redis"}
	for k, v := range pctx.Entry.Helm.Set {
		args = append(args, "--set", k+"="+v)
	}

	cmd, finalArgs := p.buildArgs(pctx, args)

	if cmd != "helm" {
		t.Errorf("expected command 'helm', got %q", cmd)
	}

	// Verify --set flag is present
	foundSet := false
	for i, a := range finalArgs {
		if a == "--set" && i+1 < len(finalArgs) && finalArgs[i+1] == "replica.replicaCount=3" {
			foundSet = true
			break
		}
	}
	if !foundSet {
		t.Errorf("expected --set replica.replicaCount=3 in args: %v", finalArgs)
	}
}
