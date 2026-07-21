package lifecycle

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// TestResolvePortOwnership covers the case the previous implementation could not
// see: a PID that is alive but is NOT the process listening on the port. The
// listener here is owned by the test process; a different tracked PID must be
// reported as a foreign owner, not as the owner.
func TestResolvePortOwnership(t *testing.T) {
	if !portOwnershipSupported() {
		t.Skip("lsof not available on this host")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*net.TCPAddr).Port
	self := os.Getpid()

	// We own the listener → resolved to our own PID, owned=true.
	if pid, owned := resolvePortOwnership(self, port); pid != self || !owned {
		t.Errorf("resolvePortOwnership(self) = (%d, %v), want (%d, true)", pid, owned, self)
	}

	// A different, non-owning PID → the port is reported as held by a foreign
	// process. This is the "alive but not the port owner" case.
	if pid, owned := resolvePortOwnership(999999999, port); pid != self || owned {
		t.Errorf("resolvePortOwnership(foreign) = (%d, %v), want (%d, false)", pid, owned, self)
	}

	// trackedPID 0 means "we have no process" → every listener is foreign.
	if pid, owned := resolvePortOwnership(0, port); pid != self || owned {
		t.Errorf("resolvePortOwnership(0) = (%d, %v), want (%d, false)", pid, owned, self)
	}
}

func TestEffectivePort(t *testing.T) {
	cases := []struct {
		name string
		app  *config.ApplicationConfig
		want int
	}{
		{"explicit port", &config.ApplicationConfig{Port: 8080}, 8080},
		{"env PORT", &config.ApplicationConfig{Environment: map[string]string{"PORT": "10200"}}, 10200},
		{"health url", &config.ApplicationConfig{Health: &config.HealthCheckConfig{URL: "http://localhost:10202/health"}}, 10202},
		{"health address", &config.ApplicationConfig{Health: &config.HealthCheckConfig{Address: "127.0.0.1:6543"}}, 6543},
		{"explicit beats env", &config.ApplicationConfig{Port: 1, Environment: map[string]string{"PORT": "2"}}, 1},
		{"url without port", &config.ApplicationConfig{Health: &config.HealthCheckConfig{URL: "http://localhost/health"}}, 0},
		{"none", &config.ApplicationConfig{}, 0},
	}
	for _, tc := range cases {
		if got := effectivePort(tc.app); got != tc.want {
			t.Errorf("%s: effectivePort = %d, want %d", tc.name, got, tc.want)
		}
	}
}

// TestResolveOwnership_ForeignPortOwnerIsNotHealthy reproduces the original
// incident: the tracked process is not the one serving the port (it crashed on
// bind, or never was the listener), while a foreign process answers the health
// probe. The probe passes, but health must be reported false.
func TestResolveOwnership_ForeignPortOwnerIsNotHealthy(t *testing.T) {
	if !portOwnershipSupported() {
		t.Skip("lsof not available on this host")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	port := srv.Listener.Addr().(*net.TCPAddr).Port

	am := NewAppManager(&config.Config{}, config.NewEnvironment(nil, "/tmp", "/tmp"))
	app := &config.ApplicationConfig{
		Port:   port,
		Health: &config.HealthCheckConfig{Type: "http", URL: srv.URL, Timeout: 2},
	}
	// PID that is not the listener (the httptest server runs under our own PID).
	status := AppStatus{Name: "web", Running: true, PID: 999999999}
	am.resolveOwnership(&status, app)

	if status.PortPID != os.Getpid() {
		t.Errorf("PortPID = %d, want %d (test process owns the port)", status.PortPID, os.Getpid())
	}
	if status.PortOwned {
		t.Error("PortOwned = true, want false: listener is outside the tracked group")
	}
	if status.Healthy {
		t.Error("Healthy = true, want false: a green probe answered by a foreign process must not be healthy")
	}
}

// TestResolveOwnership_OwnedPortIsHealthy is the positive counterpart: the
// process dva tracks IS the listener, the probe passes → healthy.
func TestResolveOwnership_OwnedPortIsHealthy(t *testing.T) {
	if !portOwnershipSupported() {
		t.Skip("lsof not available on this host")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	port := srv.Listener.Addr().(*net.TCPAddr).Port

	am := NewAppManager(&config.Config{}, config.NewEnvironment(nil, "/tmp", "/tmp"))
	app := &config.ApplicationConfig{
		Port:   port,
		Health: &config.HealthCheckConfig{Type: "http", URL: srv.URL, Timeout: 2},
	}
	// The httptest server listens under this process, so our PID owns the port.
	status := AppStatus{Name: "web", Running: true, PID: os.Getpid()}
	am.resolveOwnership(&status, app)

	if !status.PortOwned {
		t.Error("PortOwned = false, want true: the tracked PID owns the port")
	}
	if !status.Healthy {
		t.Error("Healthy = false, want true: probe passes and the port is owned")
	}
}

// TestResolveOwnership_UnmanagedAppSkipsOwnership guards the docker regression:
// an app dva does not run natively (no pidfile, PID 0) whose port is held by a
// foreign process — the shape of a docker app whose published port belongs to
// docker-proxy — must NOT be flagged as foreign. Ownership reasoning applies
// only to native, dva-managed processes.
func TestResolveOwnership_UnmanagedAppSkipsOwnership(t *testing.T) {
	if !portOwnershipSupported() {
		t.Skip("lsof not available on this host")
	}
	// Hermetic: FileDir() resolves to "." so pidFileExists looks under an empty
	// temp dir and reliably returns false.
	t.Chdir(t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	port := srv.Listener.Addr().(*net.TCPAddr).Port

	am := NewAppManager(&config.Config{}, config.NewEnvironment(nil, "/tmp", "/tmp"))
	app := &config.ApplicationConfig{
		Port:   port,
		Health: &config.HealthCheckConfig{Type: "http", URL: srv.URL, Timeout: 2},
	}
	status := AppStatus{Name: "dockerish", Running: false, PID: 0}
	am.resolveOwnership(&status, app)

	if status.PortPID != 0 || status.PortOwned {
		t.Errorf("ownership computed for unmanaged app: PortPID=%d PortOwned=%v, want 0/false",
			status.PortPID, status.PortOwned)
	}
}

// TestPortConflicts_SkipsUnmanagedApps is the PortConflicts counterpart: a
// foreign process on an app's port must not be reported as a conflict when dva
// has no pidfile for that app (docker app started via compose, or never started
// by dva). Otherwise `dva app up`/`dva doctor` would false-alarm in docker modes.
func TestPortConflicts_SkipsUnmanagedApps(t *testing.T) {
	if !portOwnershipSupported() {
		t.Skip("lsof not available on this host")
	}
	t.Chdir(t.TempDir())

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*net.TCPAddr).Port

	cfg := &config.Config{Applications: map[string]*config.ApplicationConfig{
		"dockerish": {Port: port},
	}}
	am := NewAppManager(cfg, config.NewEnvironment(nil, "/tmp", "/tmp"))

	if got := am.PortConflicts(); len(got) != 0 {
		t.Errorf("PortConflicts = %+v, want empty for an app with no pidfile", got)
	}
}

// TestPortConflicts_FlagsManagedAppWithForeignOwner is the positive control /
// original incident: dva recorded a pidfile (native app) but its process is
// gone and a foreign listener now holds the port. This MUST be reported even
// though the tracked PID is dead — that is the stale-orphan case.
func TestPortConflicts_FlagsManagedAppWithForeignOwner(t *testing.T) {
	if !portOwnershipSupported() {
		t.Skip("lsof not available on this host")
	}
	dir := t.TempDir()
	t.Chdir(dir)

	// Record a pidfile pointing at a dead PID so the app looks native-managed
	// (pidPath uses FileDir() == "." under the temp cwd).
	pidDir := filepath.Join(config.DotDirName, config.PidsDirName)
	if err := os.MkdirAll(pidDir, 0o755); err != nil {
		t.Fatalf("mkdir pids: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pidDir, "app-web.pid"), []byte("999999999"), 0o644); err != nil {
		t.Fatalf("write pidfile: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*net.TCPAddr).Port

	cfg := &config.Config{Applications: map[string]*config.ApplicationConfig{
		"web": {Port: port},
	}}
	am := NewAppManager(cfg, config.NewEnvironment(nil, dir, dir))

	conflicts := am.PortConflicts()
	if len(conflicts) != 1 || conflicts[0].App != "web" {
		t.Fatalf("PortConflicts = %+v, want exactly one conflict for \"web\"", conflicts)
	}
	if conflicts[0].Port != port {
		t.Errorf("conflict Port = %d, want %d", conflicts[0].Port, port)
	}
}
