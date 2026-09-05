package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// TASK-314: `dva build <plan>` handed every selected service to `compose build`, image-only
// ones included, which compose refuses. The plan's subset is narrowed to the services whose
// compose file gives them a build: — and only those; a service the file does not declare
// stays, because "not found here" is not "image-only".
func buildScopeFixture(t *testing.T, composeYAML string) (*config.Config, *config.Environment) {
	t.Helper()
	c := loadTestConfig(t, `version: "0.1.44"
stack:
  infra:
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
plans:
  hybrid:
    entries:
      - name: infra
        services: [postgres, redis, api, worker]
`)
	if err := os.WriteFile(filepath.Join(c.FileDir(), "compose.yml"), []byte(composeYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	return c, config.NewEnvironment(nil, c.FileDir(), c.FileDir())
}

func TestRunPlanBuildOnlyPassesBuildableServices(t *testing.T) {
	enableDryRun(t)
	c, e := buildScopeFixture(t, `services:
  postgres: {image: postgres:16}
  redis: {image: redis:7}
  api: {build: ./api}
  worker:
    build:
      context: ./worker
`)
	var err error
	stderr := captureBothStreams(t, func() { err = runPlanBuild(c, planEnv(e), "hybrid", nil) })
	if err != nil {
		t.Fatalf("runPlanBuild: %v", err)
	}
	if !strings.Contains(stderr, "build api worker") {
		t.Errorf("preview must build only the services with build:, got:\n%s", stderr)
	}
	for _, imageOnly := range []string{"postgres", "redis"} {
		if strings.Contains(stderr, " "+imageOnly) {
			t.Errorf("image-only service %q reached compose build:\n%s", imageOnly, stderr)
		}
	}
}

func TestRunPlanBuildSaysSoWhenNothingInThePlanBuilds(t *testing.T) {
	enableDryRun(t)
	c, e := buildScopeFixture(t, `services:
  postgres: {image: postgres:16}
  redis: {image: redis:7}
  api: {image: ghcr.io/x/api}
  worker: {image: ghcr.io/x/worker}
`)
	var err error
	stderr := captureBothStreams(t, func() { err = runPlanBuild(c, planEnv(e), "hybrid", nil) })
	if err != nil {
		t.Fatalf("runPlanBuild: %v", err)
	}
	if !strings.Contains(stderr, "nothing to build") || strings.Contains(stderr, "[dry-run]") {
		t.Errorf("an all-image plan must report and not run compose build:\n%s", stderr)
	}
}

func TestRunPlanBuildKeepsUndeclaredServicesWhenTheFileIsUnreadable(t *testing.T) {
	enableDryRun(t)
	c, e := buildScopeFixture(t, "")
	if err := os.Remove(filepath.Join(c.FileDir(), "compose.yml")); err != nil {
		t.Fatal(err)
	}
	var err error
	stderr := captureBothStreams(t, func() { err = runPlanBuild(c, planEnv(e), "hybrid", nil) })
	if err != nil {
		t.Fatalf("runPlanBuild: %v", err)
	}
	if !strings.Contains(stderr, "build postgres redis api worker") {
		t.Errorf("with no readable compose file the subset must pass through unchanged:\n%s", stderr)
	}
}

func TestExtractComposeBuildable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "compose.yml")
	if err := os.WriteFile(path, []byte("include:\n  - extra.yml\nservices:\n  a: {build: .}\n  b: {image: x}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "extra.yml"), []byte("services:\n  c:\n    build:\n      context: ./c\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok := extractComposeBuildable(path)
	if !ok {
		t.Fatal("file was readable")
	}
	want := map[string]bool{"a": true, "b": false, "c": true}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: buildable=%v, want %v (all: %v)", k, got[k], v, got)
		}
	}
	if _, ok := extractComposeBuildable(filepath.Join(dir, "missing.yml")); ok {
		t.Error("a missing file must report ok=false")
	}
}
