package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// TASK-267 item 4: cli.Execute discarded every loadConfig() error on the dynamic-routing
// branch unconditionally, so a `subprojects:` entry with a missing path made the parent's
// own local interactions fail as `unknown command "hello" for "dva"` — naming the wrong
// problem — while `dva run hello` (which goes through mustLoadConfig instead) already
// surfaced the true cause. shouldSurfaceDynamicRoutingConfigError is the predicate Execute
// now consults before falling through to cobra; these tests pin it against real
// config.Load() errors rather than hand-written strings, so a change to findConfig's or
// resolveSubprojectImports' wording cannot silently desync the predicate from what it is
// supposed to recognize.

// TestShouldSurfaceDynamicRoutingConfigError_NoConfigFile pins the exempted case: a
// directory with no dva.yml anywhere in its ancestry must NOT switch to the new error path.
// That case carries no subproject or path to name, and cobra's own "unknown command" plus
// suggestion list was already the right answer for it — this must stay unchanged.
func TestShouldSurfaceDynamicRoutingConfigError_NoConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := config.Load(tmpDir)
	if err == nil {
		t.Fatal("expected config.Load to fail in a directory with no dva.yml")
	}
	if !strings.Contains(err.Error(), "could not find dva.yml") {
		t.Fatalf("Load error = %q, want it to contain %q (the string both this predicate and the `dva init` hint below match on)", err.Error(), "could not find dva.yml")
	}
	if shouldSurfaceDynamicRoutingConfigError(err) {
		t.Errorf("shouldSurfaceDynamicRoutingConfigError(%v) = true, want false — this is the ordinary no-config-file case and must keep cobra's unknown-command behavior", err)
	}
}

// TestShouldSurfaceDynamicRoutingConfigError_BrokenSubproject pins the repaired case:
// a dva.yml that exists but fails to load — here, a subprojects: entry whose import: is
// declared but whose path does not exist — must switch to the new error path, and the
// underlying error must name the failing subproject and its path (not just "something
// failed"), because that is the whole point of not masking it behind "unknown command".
//
// Measured against ./bin/dva on this exact fixture: `dva hello` used to print `ERROR:
// unknown command "hello" for "dva"`; `dva run hello` already printed the message this test
// asserts config.Load returns.
func TestShouldSurfaceDynamicRoutingConfigError_BrokenSubproject(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, config.FileName), []byte(`
version: "0.1.0"
interaction:
  hello:
    description: "local hello"
    command: "echo hello"
plans:
  whatever:
    entries:
      - name: hello
subprojects:
  broken:
    path: ./nope
    import:
      plans: [{name: whatever}]
`), 0o644); err != nil {
		t.Fatalf("write dva.yml: %v", err)
	}

	_, err := config.Load(tmpDir)
	if err == nil {
		t.Fatal("expected config.Load to fail: subprojects.broken.path does not exist")
	}
	if !shouldSurfaceDynamicRoutingConfigError(err) {
		t.Errorf("shouldSurfaceDynamicRoutingConfigError(%v) = false, want true — this is a real config problem, not the no-dva.yml case", err)
	}
	if !strings.Contains(err.Error(), `"broken"`) {
		t.Errorf("Load error = %q, want it to name the failing subproject %q", err.Error(), "broken")
	}
	if !strings.Contains(err.Error(), filepath.Join(tmpDir, "nope")) {
		t.Errorf("Load error = %q, want it to name the missing path %q", err.Error(), filepath.Join(tmpDir, "nope"))
	}
}

// TestShouldSurfaceDynamicRoutingConfigError_NilError guards the zero value: Execute's
// dynamic-routing branch only calls this predicate inside the `err == nil` else-branch, but
// the predicate itself must not panic or misreport if ever called with a nil error.
func TestShouldSurfaceDynamicRoutingConfigError_NilError(t *testing.T) {
	if shouldSurfaceDynamicRoutingConfigError(nil) {
		t.Error("shouldSurfaceDynamicRoutingConfigError(nil) = true, want false")
	}
}
