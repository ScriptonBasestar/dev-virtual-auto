package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// Exit codes the shim below reads, so one fake binary can model both halves of the
// condition under test: the compose command that failed, and the daemon that does or
// does not answer `docker info`.
const (
	shimInfoExitVar    = "DVA_TEST_SHIM_INFO_EXIT"
	shimComposeExitVar = "DVA_TEST_SHIM_COMPOSE_EXIT"
)

// installShims writes an executable for each name into a fresh directory and makes that
// directory the entire PATH.
//
// PATH is replaced rather than prepended so a real docker on the machine — running or not
// — cannot decide the result. The shim body uses only shell builtins for the same reason:
// with PATH reduced to one directory it cannot call anything it does not ship. Mirrors
// composeShims in internal/cli.
func installShims(t *testing.T, names ...string) {
	t.Helper()

	dir := t.TempDir()
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"info\" ]; then exit \"${" + shimInfoExitVar + ":-0}\"; fi\n" +
		"exit \"${" + shimComposeExitVar + ":-0}\"\n"

	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(script), 0o755); err != nil {
			t.Fatalf("write %s shim: %v", n, err)
		}
	}
	t.Setenv("PATH", dir)
}

// shimmedCompose returns a plugin context whose compose entry has no files, so Up skips
// preflightConfig and reaches the subprocess this test is about.
func shimmedCompose(t *testing.T, command string) (*ComposePlugin, *PluginContext) {
	t.Helper()

	dir := t.TempDir()
	return &ComposePlugin{}, &PluginContext{
		Entry: &config.LifecycleEntry{
			Name:    "compose",
			Compose: &config.ComposePluginConfig{Command: command},
		},
		Env:       config.NewEnvironment(nil, dir, dir),
		ConfigDir: dir,
		Logger:    slog.Default(),
	}
}

// Given a compose command that fails while `docker info` also fails, When up runs, Then
// the error names the daemon instead of relaying a bare exit status.
//
// This is the condition a user hits with Docker Desktop or colima stopped: `compose
// config` needs no daemon so preflight has nothing to say, and before this the only
// output was docker's raw stderr followed by "exit status 1".
func TestComposeUp_DaemonUnreachable_ReportsDaemon(t *testing.T) {
	installShims(t, "docker")
	t.Setenv(shimComposeExitVar, "1")
	t.Setenv(shimInfoExitVar, "1")

	p, pctx := shimmedCompose(t, "")

	_, err := p.Up(context.Background(), pctx)

	var daemonErr *DockerDaemonError
	if !errors.As(err, &daemonErr) {
		t.Fatalf("Up() error = %v, want a *DockerDaemonError", err)
	}
	if daemonErr.Op != "up" {
		t.Errorf("Op = %q, want %q", daemonErr.Op, "up")
	}
	if !strings.Contains(err.Error(), "dva doctor") {
		t.Errorf("error does not point at doctor: %q", err.Error())
	}
}

// Given the same failure while the daemon answers, When up runs, Then the original error
// survives unchanged.
//
// The control that keeps the check honest: a compose command fails far more often for
// reasons that are not the daemon — an unhealthy service under --wait, a failed build, a
// bound port — and blaming the daemon for those would be worse than the bare exit status
// it replaced.
func TestComposeUp_DaemonReachable_KeepsOriginalError(t *testing.T) {
	installShims(t, "docker")
	t.Setenv(shimComposeExitVar, "1")
	t.Setenv(shimInfoExitVar, "0")

	p, pctx := shimmedCompose(t, "")

	_, err := p.Up(context.Background(), pctx)

	if err == nil {
		t.Fatal("Up() error = nil, want the compose failure")
	}
	var daemonErr *DockerDaemonError
	if errors.As(err, &daemonErr) {
		t.Fatalf("Up() blamed the daemon while it was reachable: %v", err)
	}
	if !strings.Contains(err.Error(), "compose up") {
		t.Errorf("error lost its compose context: %q", err.Error())
	}
}

// Given a replaced compose runner, When it fails with the Docker daemon down, Then Docker
// is not blamed for a runner that never used it.
//
// `docker` is installed alongside and reports unreachable, so a probe that ignored which
// binary actually ran would answer here and be wrong.
func TestComposeUp_ReplacedRunner_DoesNotBlameDockerDaemon(t *testing.T) {
	installShims(t, "docker", "podman-compose")
	t.Setenv(shimComposeExitVar, "1")
	t.Setenv(shimInfoExitVar, "1")

	p, pctx := shimmedCompose(t, "podman-compose")

	_, err := p.Up(context.Background(), pctx)

	if err == nil {
		t.Fatal("Up() error = nil, want the podman-compose failure")
	}
	var daemonErr *DockerDaemonError
	if errors.As(err, &daemonErr) {
		t.Fatalf("Up() blamed the Docker daemon for a podman-compose failure: %v", err)
	}
}

// Given the daemon is unreachable, When down or stop runs, Then each reports the daemon
// with its own operation named.
//
// Symmetry is the reason the probe sits in runSubprocess: the failure is identical for
// up, down and stop, so a user who cannot start the stack and then cannot tear it down
// must not get an explanation for one and a bare exit status for the other.
func TestComposeDownStop_DaemonUnreachable_ReportSymmetrically(t *testing.T) {
	installShims(t, "docker")
	t.Setenv(shimComposeExitVar, "1")
	t.Setenv(shimInfoExitVar, "1")

	p, pctx := shimmedCompose(t, "")

	for _, tc := range []struct {
		name string
		run  func() error
		op   string
	}{
		{"down", func() error { return p.Down(context.Background(), pctx) }, "down"},
		{"stop", func() error { return p.Stop(context.Background(), pctx) }, "stop"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run()

			var daemonErr *DockerDaemonError
			if !errors.As(err, &daemonErr) {
				t.Fatalf("%s error = %v, want a *DockerDaemonError", tc.name, err)
			}
			if daemonErr.Op != tc.op {
				t.Errorf("Op = %q, want %q", daemonErr.Op, tc.op)
			}
		})
	}
}

// Given a reachable daemon, When the probe runs, Then it says so — and says the opposite
// when `docker info` fails. Locks the one behaviour doctor and the lifecycle path share.
func TestDockerDaemonReachable(t *testing.T) {
	installShims(t, "docker")

	t.Setenv(shimInfoExitVar, "0")
	if !DockerDaemonReachable(nil) {
		t.Error("DockerDaemonReachable() = false while `docker info` exits 0")
	}

	t.Setenv(shimInfoExitVar, "1")
	if DockerDaemonReachable(nil) {
		t.Error("DockerDaemonReachable() = true while `docker info` exits 1")
	}
}

func TestDockerDaemonError_Error(t *testing.T) {
	e := &DockerDaemonError{Op: "up", cause: errors.New("exit status 1")}

	msg := e.Error()

	// Names the condition, and points at the command that confirms it. Before this, the
	// diagnosis existed only behind `dva doctor --strict`, which nothing here mentioned.
	for _, want := range []string{
		"Docker daemon is not reachable",
		"docker compose up",
		"DOCKER_HOST",
		"dva doctor",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("Error() = %q, want it to contain %q", msg, want)
		}
	}
}

func TestDockerDaemonError_Unwrap(t *testing.T) {
	cause := errors.New("boom")
	e := &DockerDaemonError{Op: "up", cause: cause}

	if !errors.Is(e, cause) {
		t.Errorf("errors.Is(e, cause) = false, want true")
	}

	var target *DockerDaemonError
	if !errors.As(fmt.Errorf("wrapped: %w", e), &target) {
		t.Errorf("errors.As did not recover *DockerDaemonError through a wrap")
	}
}
