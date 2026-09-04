package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/lifecycle"
)

// TASK-180: type: docker_socket hard-coded /var/run/docker.sock and ignored DOCKER_HOST,
// so a healthy Colima host failed the user check while the built-in daemon check passed.

func TestResolveDockerSocketPath(t *testing.T) {
	tests := []struct {
		host    string
		want    string
		fromEnv bool
	}{
		{"", "/var/run/docker.sock", false},
		{"  ", "/var/run/docker.sock", false},
		{"unix:///Users/me/.colima/default/docker.sock", "/Users/me/.colima/default/docker.sock", true},
		{"unix:///var/run/docker.sock", "/var/run/docker.sock", true},
		{"tcp://127.0.0.1:2375", "", true},
		{"ssh://user@host", "", true},
	}
	for _, tt := range tests {
		got, fromEnv := resolveDockerSocketPath(tt.host)
		if got != tt.want || fromEnv != tt.fromEnv {
			t.Errorf("resolveDockerSocketPath(%q) = (%q, %v), want (%q, %v)",
				tt.host, got, fromEnv, tt.want, tt.fromEnv)
		}
	}
}

func TestEvaluateDockerSocket_UsesDOCKER_HOSTUnixPath(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "docker.sock")
	// A regular file is enough for Stat+Open; we are not talking to a daemon.
	if err := os.WriteFile(sock, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	daemonCalled := false
	res := evaluateDockerSocket("unix://"+sock, func() bool {
		daemonCalled = true
		return false
	})
	if !res.Passed {
		t.Fatalf("passed=false finding=%q, want pass on an openable DOCKER_HOST socket", res.Finding)
	}
	if daemonCalled {
		t.Fatal("daemon probe must not run when the resolved socket is openable")
	}
}

func TestEvaluateDockerSocket_NamesMissingExplicitPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such.sock")
	res := evaluateDockerSocket("unix://"+missing, func() bool {
		t.Fatal("daemon probe must not paper over an explicit missing unix:// path")
		return true
	})
	if res.Passed {
		t.Fatal("want fail when DOCKER_HOST points at a missing socket")
	}
	if !strings.Contains(res.Finding, missing) {
		t.Errorf("finding = %q, want it to name the path that was checked", res.Finding)
	}
	if strings.Contains(res.Finding, "NOT accessible") && !strings.Contains(res.Finding, missing) {
		t.Errorf("finding is the old generic line: %q", res.Finding)
	}
}

func TestEvaluateDockerSocket_DefaultPathMissingFallsThroughToDaemon(t *testing.T) {
	// Empty DOCKER_HOST → default /var/run/docker.sock. That path almost never exists in
	// this test's environment as an openable socket we own; force the fall-through by
	// only asserting the daemon callback when the default is absent.
	if _, err := os.Stat("/var/run/docker.sock"); err == nil {
		// Openable default would short-circuit — still a valid pass, just not this branch.
		t.Skip("default socket exists on this host; fall-through branch not reachable")
	}

	res := evaluateDockerSocket("", func() bool { return true })
	if !res.Passed {
		t.Fatalf("passed=false finding=%q, want pass when default path is absent but daemon is reachable", res.Finding)
	}

	res = evaluateDockerSocket("", func() bool { return false })
	if res.Passed {
		t.Fatal("want fail when default path is absent and daemon is unreachable")
	}
	if !strings.Contains(res.Finding, "/var/run/docker.sock") {
		t.Errorf("finding = %q, want it to name the default path that was checked", res.Finding)
	}
}

func TestEvaluateDockerSocket_UnopenableFallsThroughWhenDaemonReachable(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "docker.sock")
	if err := os.WriteFile(sock, []byte{}, 0o000); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sock, 0o000); err != nil {
		t.Fatal(err)
	}
	if f, err := os.Open(sock); err == nil {
		_ = f.Close()
		t.Skip("this user can open a 000 file; unopenable branch is not reachable")
	}

	daemonCalled := false
	res := evaluateDockerSocket("unix://"+sock, func() bool {
		daemonCalled = true
		return true
	})
	if !res.Passed {
		t.Fatalf("passed=false finding=%q, want pass when the socket is unopenable but the daemon is reachable", res.Finding)
	}
	if !daemonCalled {
		t.Fatal("daemon probe must run when the resolved socket exists but cannot be opened")
	}

	res = evaluateDockerSocket("unix://"+sock, func() bool { return false })
	if res.Passed {
		t.Fatal("want fail when the socket is unopenable and the daemon is unreachable")
	}
	if !strings.Contains(res.Finding, sock) {
		t.Errorf("finding = %q, want it to name the unopenable path", res.Finding)
	}
	if !strings.Contains(res.Finding, "cannot open") {
		t.Errorf("finding = %q, want the permission finding", res.Finding)
	}
}

func TestEvaluateDockerSocket_NonUnixDOCKER_HOSTUsesDaemon(t *testing.T) {
	res := evaluateDockerSocket("tcp://127.0.0.1:2375", func() bool { return true })
	if !res.Passed {
		t.Fatalf("passed=false finding=%q, want pass when non-unix DOCKER_HOST has a reachable daemon", res.Finding)
	}

	res = evaluateDockerSocket("tcp://127.0.0.1:2375", func() bool { return false })
	if res.Passed {
		t.Fatal("want fail when non-unix DOCKER_HOST and daemon is down")
	}
	if !strings.Contains(res.Finding, "tcp://127.0.0.1:2375") {
		t.Errorf("finding = %q, want DOCKER_HOST named", res.Finding)
	}
}

// Opposite-verdict guard: when the portable probe says the daemon is up, docker_socket
// must not fail solely because the default path is missing or unopenable. That is the
// Colima/Desktop/GitHub-hosted-Linux shape that made doctor print one pass and one fail
// for the same daemon (TASK-180).
func TestDockerSocketAndDaemonAgreeWhenDaemonReachable(t *testing.T) {
	if !lifecycle.DockerDaemonReachable(nil) {
		t.Skip("docker info failed; cannot assert agreement on a live daemon")
	}

	// Force the default-path branch by clearing DOCKER_HOST for this process view.
	t.Setenv("DOCKER_HOST", "")
	// Even if the real env had a working unix socket elsewhere, empty DOCKER_HOST means
	// we resolve the default; evaluateDockerSocket with daemonOK=true covers the case
	// the default is missing. Also run the real checkDockerSocketPermissions.
	if !checkDocker().Passed {
		t.Fatal("precondition: checkDocker should pass when DockerDaemonReachable is true")
	}
	sock := checkDockerSocketPermissions()
	if !sock.Passed {
		t.Fatalf("docker_socket failed while daemon is reachable: finding=%q", sock.Finding)
	}
}

func TestRunSingleCheck_DockerSocket_UsesPathInFinding(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "gone.sock")
	t.Setenv("DOCKER_HOST", "unix://"+missing)

	res := runSingleCheck(config.DoctorCheck{
		Name: "Docker socket accessible",
		Type: "docker_socket",
	}, t.TempDir())
	if res.Passed {
		t.Fatal("want fail for missing explicit DOCKER_HOST socket")
	}
	if !strings.Contains(res.Finding, missing) {
		t.Errorf("finding = %q, want the checked path (not the generic NOT accessible line)", res.Finding)
	}
	if res.Finding == "Docker socket is NOT accessible" {
		t.Error("generic finding returned — path detail was discarded")
	}
}
