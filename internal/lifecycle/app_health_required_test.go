// Package lifecycle — red/green contract for TASK-118 health.required.
//
// An alive process that owns its port but never answers its readiness probe
// currently prints [warn] and returns nil. required:true must promote that
// branch to [FAIL] + recordErr; omitted/false must keep the advisory line.
package lifecycle

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// Exact advisory line emitted by the alive/not-ready else arm today.
// Byte-for-byte stability is the compatibility contract for omitted/false.
const healthNotReadyWarnLine = "[warn] app web not ready after 1s\n"

// captureStderr runs fn with os.Stderr redirected to a pipe.
// Do not use t.Parallel in callers: stderr is process-global.
func captureStderr(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	old := os.Stderr
	os.Stderr = w
	runErr := fn()
	_ = w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String(), runErr
}

// listenerNativeCommand starts this test binary as TestAppListenerHelper on port.
func listenerNativeCommand(t *testing.T, self string, port int) (cmd string, env map[string]string) {
	t.Helper()
	return "'" + self + "' -test.run '^TestAppListenerHelper$'",
		map[string]string{listenerHelperEnv: strconv.Itoa(port)}
}

// unhealthyCommandHealth is a probe that never passes while the process stays up.
func unhealthyCommandHealth(required bool) *config.HealthCheckConfig {
	return &config.HealthCheckConfig{
		Type:         "command",
		Command:      "exit 1",
		Timeout:      1,
		ReadyTimeout: 1,
		Required:     required,
	}
}

func TestStartAppsHealthRequiredContract(t *testing.T) {
	// Resolve before newStartAppsTest chdirs into a temp dir.
	self, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Skipf("cannot locate the test binary to use as a listener: %v", err)
	}

	t.Run("omitted_defaults_advisory", func(t *testing.T) {
		// Zero-value Required is the post-parse shape of an omitted field.
		port := freePort(t)
		cmd, env := listenerNativeCommand(t, self, port)
		am := newStartAppsTest(t, map[string]*config.ApplicationConfig{
			"web": {
				Port:        port,
				Run:         config.AppExecPaths{Native: cmd},
				Environment: env,
				Health:      unhealthyCommandHealth(false),
			},
		})
		t.Cleanup(func() { am.DownApps("web") })

		stderr, err := captureStderr(t, func() error {
			return am.StartApps(context.Background(), AppStartOptions{Wait: true})
		})
		if err != nil {
			t.Fatalf("omitted required must stay advisory (nil err); got: %v\nstderr:\n%s", err, stderr)
		}
		if !strings.Contains(stderr, healthNotReadyWarnLine) {
			t.Fatalf("missing exact warn line %q\nstderr:\n%s", healthNotReadyWarnLine, stderr)
		}
		if n := strings.Count(stderr, healthNotReadyWarnLine); n != 1 {
			t.Fatalf("want exactly one warn line, got %d\nstderr:\n%s", n, stderr)
		}
	})

	t.Run("required_false_advisory", func(t *testing.T) {
		port := freePort(t)
		cmd, env := listenerNativeCommand(t, self, port)
		am := newStartAppsTest(t, map[string]*config.ApplicationConfig{
			"web": {
				Port:        port,
				Run:         config.AppExecPaths{Native: cmd},
				Environment: env,
				Health:      unhealthyCommandHealth(false),
			},
		})
		t.Cleanup(func() { am.DownApps("web") })

		stderr, err := captureStderr(t, func() error {
			return am.StartApps(context.Background(), AppStartOptions{Wait: true})
		})
		if err != nil {
			t.Fatalf("required:false must stay advisory (nil err); got: %v\nstderr:\n%s", err, stderr)
		}
		if !strings.Contains(stderr, healthNotReadyWarnLine) {
			t.Fatalf("missing exact warn line %q\nstderr:\n%s", healthNotReadyWarnLine, stderr)
		}
	})

	t.Run("required_true_unhealthy_alive_and_port_owned", func(t *testing.T) {
		// Process owns the port; command probe never passes — the TASK-118 hole.
		port := freePort(t)
		cmd, env := listenerNativeCommand(t, self, port)
		am := newStartAppsTest(t, map[string]*config.ApplicationConfig{
			"web": {
				Port:        port,
				Run:         config.AppExecPaths{Native: cmd},
				Environment: env,
				Health:      unhealthyCommandHealth(true),
			},
		})
		t.Cleanup(func() { am.DownApps("web") })

		stderr, err := captureStderr(t, func() error {
			return am.StartApps(context.Background(), AppStartOptions{Wait: true})
		})
		if err == nil {
			t.Fatalf("required:true unhealthy must return error; stderr:\n%s", stderr)
		}
		if !strings.Contains(err.Error(), "app web not ready after 1s") {
			t.Errorf("error must name app and timeout:\n%s", err.Error())
		}
		if !strings.Contains(stderr, "[FAIL]") {
			t.Errorf("required:true must print [FAIL]; stderr:\n%s", stderr)
		}
		if strings.Contains(stderr, healthNotReadyWarnLine) {
			t.Errorf("required:true must not print advisory warn line; stderr:\n%s", stderr)
		}
	})

	t.Run("required_true_healthy", func(t *testing.T) {
		port := freePort(t)
		addr := "127.0.0.1:" + strconv.Itoa(port)
		cmd, env := listenerNativeCommand(t, self, port)
		am := newStartAppsTest(t, map[string]*config.ApplicationConfig{
			"web": {
				Port:        port,
				Run:         config.AppExecPaths{Native: cmd},
				Environment: env,
				Health: &config.HealthCheckConfig{
					Type:         "tcp",
					Address:      addr,
					Timeout:      1,
					ReadyTimeout: 5,
					Required:     true,
				},
			},
		})
		t.Cleanup(func() { am.DownApps("web") })

		stderr, err := captureStderr(t, func() error {
			return am.StartApps(context.Background(), AppStartOptions{Wait: true})
		})
		if err != nil {
			t.Fatalf("required:true with healthy probe must succeed; got: %v\nstderr:\n%s", err, stderr)
		}
		if strings.Contains(stderr, "not ready after") {
			t.Errorf("healthy path must not emit not-ready; stderr:\n%s", stderr)
		}
	})

	t.Run("wait_false_skips_required_health", func(t *testing.T) {
		port := freePort(t)
		cmd, env := listenerNativeCommand(t, self, port)
		am := newStartAppsTest(t, map[string]*config.ApplicationConfig{
			"web": {
				Port:        port,
				Run:         config.AppExecPaths{Native: cmd},
				Environment: env,
				Health:      unhealthyCommandHealth(true),
			},
		})
		t.Cleanup(func() { am.DownApps("web") })

		stderr, err := captureStderr(t, func() error {
			return am.StartApps(context.Background(), AppStartOptions{Wait: false})
		})
		if err != nil {
			t.Fatalf("Wait:false must skip readiness even when required; got: %v\nstderr:\n%s", err, stderr)
		}
		if strings.Contains(stderr, "not ready after") {
			t.Errorf("Wait:false must not run readiness wait; stderr:\n%s", stderr)
		}
	})
}
