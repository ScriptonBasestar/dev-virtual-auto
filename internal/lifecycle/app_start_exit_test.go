// Package lifecycle — regression tests for TASK-117.
//
// startWave ends in `errors.Join(errs...)`, so a failure only reaches the exit code if it
// was appended to errs. Three post-start branches printed `[FAIL]` and appended nothing:
// DVA waited the full timeout, correctly concluded the process never listened, printed a
// precise message with a log path — and returned nil, so `dva up` and `dva app up` exited 0
// and any `dva up && next-step` chain carried on.
//
// These tests assert the returned error, not the printed line, because the printed line was
// never the broken half.
package lifecycle

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// listenerHelperEnv turns this test binary into a stand-in application: when it is set,
// TestAppListenerHelper binds the named port and blocks. Using the test binary rather than
// `nc` keeps the healthy control hermetic — no assumption about which netcat is installed
// (BSD, openbsd, nmap-ncat and GNU netcat do not agree on how to spell "listen on a port").
const listenerHelperEnv = "DVA_TEST_LISTEN_PORT"

// TestAppListenerHelper is not a test. It is the process the healthy control starts, and it
// skips unless invoked as one.
func TestAppListenerHelper(t *testing.T) {
	port := os.Getenv(listenerHelperEnv)
	if port == "" {
		t.Skip("not invoked as a listener helper")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err != nil {
		t.Fatalf("helper could not listen on %s: %v", port, err)
	}
	defer func() { _ = ln.Close() }()

	// Accept in the background so the runtime has a netpoll waiter (a bare block here would
	// trip the deadlock detector), and bound the process's life so a helper that outlives a
	// failed cleanup cannot hold the port indefinitely.
	go func() {
		for {
			conn, aerr := ln.Accept()
			if aerr != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	time.Sleep(2 * time.Minute)
}

// freePort returns a port nothing is listening on. The listener is closed before returning:
// startWave's preflight skips an app whose port is already held, which would take the test
// down a different path than the one under test.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		t.Fatalf("release port: %v", err)
	}
	return port
}

// newStartAppsTest builds an AppManager rooted in a temp cwd, so pidfiles and logs land
// under t.TempDir() rather than in the repo.
func newStartAppsTest(t *testing.T, apps map[string]*config.ApplicationConfig) *AppManager {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	return NewAppManager(&config.Config{Applications: apps}, config.NewEnvironment(nil, dir, dir))
}

// TestStartAppsErrorsWhenProcessNeverListens is the case reported in TASK-117: a run command
// that exits without binding. The port-ownership check detected it and printed [FAIL] before
// the fix; what it did not do was tell the caller.
func TestStartAppsErrorsWhenProcessNeverListens(t *testing.T) {
	if !portOwnershipSupported() {
		t.Skip("lsof not available on this host")
	}
	port := freePort(t)
	am := newStartAppsTest(t, map[string]*config.ApplicationConfig{
		"web": {Port: port, Run: config.AppExecPaths{Native: "exit 0"}},
	})
	t.Cleanup(func() { am.DownApps("web") })

	err := am.StartApps(context.Background(), AppStartOptions{Wait: true})
	if err == nil {
		t.Fatal("StartApps returned nil for an app that never bound its port; `dva up` exits 0 and reports success to the shell")
	}
	if want := "did not listen on port " + strconv.Itoa(port); !strings.Contains(err.Error(), want) {
		t.Errorf("error does not name the failure %q:\n%s", want, err.Error())
	}
}

// TestStartAppsErrorsWhenAppExitsDuringHealthCheck covers the sibling branch, reached when a
// health check is configured: the probe never passes and the process is already gone.
func TestStartAppsErrorsWhenAppExitsDuringHealthCheck(t *testing.T) {
	port := freePort(t)
	addr := "127.0.0.1:" + strconv.Itoa(port)
	am := newStartAppsTest(t, map[string]*config.ApplicationConfig{
		"web": {
			Port: port,
			Run:  config.AppExecPaths{Native: "exit 1"},
			Health: &config.HealthCheckConfig{
				Type: "tcp", Address: addr, Timeout: 1, ReadyTimeout: 5,
			},
		},
	})
	t.Cleanup(func() { am.DownApps("web") })

	err := am.StartApps(context.Background(), AppStartOptions{Wait: true})
	if err == nil {
		t.Fatal("StartApps returned nil for an app that exited during startup")
	}
	if !strings.Contains(err.Error(), "exited during startup") {
		t.Errorf("error does not name the failure:\n%s", err.Error())
	}
}

// TestStartAppsReturnsNilWhenProcessOwnsItsPort is the control that keeps the two above from
// being satisfied by a StartApps that fails everything. It starts a process that really does
// bind the declared port and asserts the same code path still returns nil.
func TestStartAppsReturnsNilWhenProcessOwnsItsPort(t *testing.T) {
	if !portOwnershipSupported() {
		t.Skip("lsof not available on this host")
	}
	// Resolve the test binary before newStartAppsTest changes the working directory.
	self, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Skipf("cannot locate the test binary to use as a listener: %v", err)
	}

	port := freePort(t)
	am := newStartAppsTest(t, map[string]*config.ApplicationConfig{
		"web": {
			Port:        port,
			Run:         config.AppExecPaths{Native: "'" + self + "' -test.run '^TestAppListenerHelper$'"},
			Environment: map[string]string{listenerHelperEnv: strconv.Itoa(port)},
		},
	})
	t.Cleanup(func() { am.DownApps("web") })

	if err := am.StartApps(context.Background(), AppStartOptions{Wait: true}); err != nil {
		t.Fatalf("StartApps failed for an app that owns its port; the fix turned a false success into a false failure:\n%v", err)
	}
}
