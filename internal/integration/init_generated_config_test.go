//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// TestGeneratedInitConfigResolvesBareLifecycleDefault pins TASK-250 completion
// criterion 3: a config shaped like `dva init`'s compose-only output (one
// verified, self-contained stack entry, no plans:) must load, validate, and
// resolve the decided bare lifecycle default — "no plans configured" runs
// every declared stack entry (DefaultPlanSource "none"), per TASK-249's
// single-plan-implicit-default ruling. It also pins that the compose runner
// is an explicit lifecycle selection (default_runner: compose), not a guess.
//
// This mirrors internal/cli's generateConfigIn("minimal") output rather than
// importing it directly: internal/cli is not importable here, and the
// generator's own unexported internals are already pinned in
// internal/cli/init_test.go (TestInitPublicSurfaceCompatibility).
func TestGeneratedInitConfigResolvesBareLifecycleDefault(t *testing.T) {
	dir := t.TempDir()
	content := `version: "0.1.44"

stack:
  compose:
    default_runner: compose
    runners:
      compose:
        files:
          - docker-compose.yml

interaction:
  shell:
    description: "Open shell in app container"
    service: app
    command: /bin/bash
`
	if err := os.WriteFile(filepath.Join(dir, "dva.yml"), []byte(content), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("services:\n  app:\n    image: busybox\n"), 0644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	c, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if len(c.Plans) != 0 {
		t.Fatalf("expected no plans: declared for a single verified closure, got %d", len(c.Plans))
	}
	if got := c.DefaultPlanSource(); got != "none" {
		t.Fatalf("DefaultPlanSource() = %q, want %q (bare lifecycle commands run every declared stack entry when no plans: exist)", got, "none")
	}

	entry, ok := c.Stack["compose"]
	if !ok {
		t.Fatalf("expected verified stack entry %q", "compose")
	}
	if entry.DefaultRunner != "compose" {
		t.Fatalf("DefaultRunner = %q, want explicit %q (compose evidence, not a guess)", entry.DefaultRunner, "compose")
	}
}
