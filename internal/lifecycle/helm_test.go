package lifecycle

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// helmNotInstalledShim puts a fake `helm` first on PATH that reports every release as
// not installed: `helm status` exits non-zero on stderr with helm's own "release: not
// found" wording, and `helm uninstall` also exits non-zero, so a test relying on this
// shim only passes if Stop never reaches uninstall for a not-installed release. `helm
// upgrade --install` exits 0 so Up can still succeed. Mirrors installShims in
// docker_daemon_test.go: PATH is replaced, not prepended, so a real helm on the machine
// cannot decide the result.
//
// The exact stderr wording matters: Stop only treats a probe failure as
// nothing-to-tear-down when helm says the release is missing, so a shim that merely
// exited non-zero would no longer model the not-installed case at all.
func helmNotInstalledShim(t *testing.T) {
	t.Helper()
	helmShim(t, "  status) echo \"Error: release: not found\" >&2; exit 1 ;;\n"+
		"  uninstall) exit 1 ;;\n")
}

// helmShim writes a fake `helm` whose `case "$1"` body is the caller's, with every
// unlisted subcommand (notably `upgrade --install`) exiting 0, and replaces PATH with
// its directory.
func helmShim(t *testing.T, cases string) {
	t.Helper()

	dir := t.TempDir()
	script := "#!/bin/sh\ncase \"$1\" in\n" + cases + "  *) exit 0 ;;\nesac\n"
	if err := os.WriteFile(filepath.Join(dir, "helm"), []byte(script), 0o755); err != nil {
		t.Fatalf("write helm shim: %v", err)
	}
	t.Setenv("PATH", dir)
}

// helmStopContext builds the PluginContext the Stop tests below drive, pointed at
// release and carrying vars as the dva.yml-supplied environment.
func helmStopContext(release string, vars map[string]string) *PluginContext {
	return &PluginContext{
		Entry: &config.LifecycleEntry{
			Name: "svc",
			Helm: &config.HelmPluginConfig{Chart: "bitnami/redis", Release: release},
		},
		Env:       config.NewEnvironment(vars, "/tmp", "/tmp"),
		ConfigDir: "/project",
		Logger:    slog.Default(),
	}
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

// TestHelmPlugin_Stop_ProbeFailureIsNotTreatedAsNotInstalled is the negative half of
// TASK-300, added after independent review. The not-installed no-op is keyed on helm's
// own "release: not found" message, not on "the probe exited non-zero": an unreachable
// cluster, an auth failure or a bad kube-context also exit 1, and classifying those as
// not-installed would skip the uninstall of a release that IS deployed and hand
// Orchestrator.Stop a success for a teardown that never ran — reopening the swallowed-
// teardown-failure class of bug TASK-295 closed. The shim's uninstall exits 0 here, so
// this fails loudly if Stop ever silently returns nil.
func TestHelmPlugin_Stop_ProbeFailureIsNotTreatedAsNotInstalled(t *testing.T) {
	helmShim(t, "  status) echo \"Error: Kubernetes cluster unreachable\" >&2; exit 1 ;;\n")

	err := (&HelmPlugin{}).Stop(context.Background(), helmStopContext("really-installed", nil))
	if err == nil {
		t.Fatal("Stop must surface a probe failure that is not helm's not-found signal, got nil")
	}
	if !strings.Contains(err.Error(), "helm status") {
		t.Errorf("error = %q, want it to name the failed probe", err)
	}
}

// TestHelmPlugin_Stop_ProbeUsesConfiguredEnv pins the probe to the same environment the
// uninstall it gates runs under. dva.yml's `environment:` vars reach helm through
// dvaexec.ExecSubprocess (which sets Env from the config environment) but are never
// exported into dva's own process, so a probe reading only os.Environ() would miss a
// KUBECONFIG supplied that way, fail, and silently skip tearing down an installed
// release. The shim's status only succeeds when it sees that KUBECONFIG.
func TestHelmPlugin_Stop_ProbeUsesConfiguredEnv(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "uninstall-ran")
	helmShim(t, "  status) [ \"$KUBECONFIG\" = \"/from/dva/yml\" ] && exit 0 || exit 1 ;;\n"+
		"  uninstall) : > "+marker+"; exit 0 ;;\n")

	pctx := helmStopContext("really-installed", map[string]string{"KUBECONFIG": "/from/dva/yml"})
	if err := (&HelmPlugin{}).Stop(context.Background(), pctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("Stop returned success without running `helm uninstall`: the probe did not " +
			"see the dva.yml-supplied KUBECONFIG that the uninstall itself would have seen")
	}
}

// TestHelmPlugin_Stop_ProbeRespectsCancellation covers the cancellation half of the same
// rule: a probe that never completed says nothing about whether the release exists, so a
// cancelled or timed-out context must surface as an error rather than be read as
// not-installed and skip the teardown.
func TestHelmPlugin_Stop_ProbeRespectsCancellation(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "uninstall-ran")
	helmShim(t, "  uninstall) : > "+marker+"; exit 0 ;;\n")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := (&HelmPlugin{}).Stop(ctx, helmStopContext("really-installed", nil))
	if err == nil {
		t.Fatal("Stop must surface a cancelled probe as an error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %q, want it to wrap context.Canceled", err)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Error("Stop ran `helm uninstall` under a cancelled context")
	}
}
