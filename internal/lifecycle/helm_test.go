package lifecycle

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// helmNotInstalledShim puts a fake `helm` first on PATH that reports every release as
// not installed: `helm status` exits non-zero (mirroring helm's real "release: not
// found" behavior) and `helm uninstall` also exits non-zero, so a test relying on this
// shim only passes if Stop never reaches uninstall for a not-installed release. `helm
// upgrade --install` exits 0 so Up can still succeed. Mirrors installShims in
// docker_daemon_test.go: PATH is replaced, not prepended, so a real helm on the machine
// cannot decide the result.
func helmNotInstalledShim(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	script := "#!/bin/sh\n" +
		"case \"$1\" in\n" +
		"  status) exit 1 ;;\n" +
		"  uninstall) exit 1 ;;\n" +
		"  *) exit 0 ;;\n" +
		"esac\n"
	if err := os.WriteFile(filepath.Join(dir, "helm"), []byte(script), 0o755); err != nil {
		t.Fatalf("write helm shim: %v", err)
	}
	t.Setenv("PATH", dir)
}

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

// TestHelmPlugin_Stop_ReleaseNotInstalled covers TASK-300. Before the fix, Stop
// delegated straight to Down (`helm uninstall`), which errors with "release: not
// found" for a release that was never installed — e.g. a first-ever restart, or after
// a manual `helm uninstall` outside dva. Stop must treat that as nothing-to-tear-down
// and return nil, the same way ProcessPlugin.haltProcess treats a missing PID file as
// already-stopped rather than a failure.
func TestHelmPlugin_Stop_ReleaseNotInstalled(t *testing.T) {
	helmNotInstalledShim(t)

	p := &HelmPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Name: "test-release",
			Helm: &config.HelmPluginConfig{
				Chart:   "bitnami/redis",
				Release: "never-installed-release",
			},
		},
		Env:       env,
		ConfigDir: "/project",
		Logger:    slog.Default(),
	}

	if err := p.Stop(context.Background(), pctx); err != nil {
		t.Fatalf("Stop on a not-installed release should be a no-op success, got: %v", err)
	}
}
