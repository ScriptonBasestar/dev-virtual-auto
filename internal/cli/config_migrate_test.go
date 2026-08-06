package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// captureOutput redirects both os.Stdout and os.Stderr for the duration of fn and
// returns what was written, combined. config_migrate.go's RunE writes across both —
// the migrated YAML goes to stdout, everything about the run goes to stderr — so a
// caller checking only one stream would miss the other.
// (plan_resolution_test.go's captureStreams keeps the two apart, for the one property
// here that depends on the split: `dva config migrate > dva.yml` must produce a file
// DVA can read.)
func captureOutput(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout, oldStderr := os.Stdout, os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	os.Stderr = w

	fn()

	w.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func writeConfigMigrateFixture(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", config.FileName, err)
	}
}

// runConfigMigrate invokes the command against a fixture directory.
func runConfigMigrate(t *testing.T, dir string) string {
	t.Helper()
	return captureOutput(t, func() {
		if err := configMigrateCmd.RunE(configMigrateCmd, []string{dir}); err != nil {
			t.Fatalf("RunE error = %v", err)
		}
	})
}

// TestConfigMigrateNoOpMessage covers TASK-069: a config with nothing to convert must
// say so and point at 'dva validate', instead of a bare "nothing found" that reads as
// "nothing to do" when validate has deprecation warnings of its own.
func TestConfigMigrateNoOpMessage(t *testing.T) {
	tmpDir := t.TempDir()
	writeConfigMigrateFixture(t, tmpDir, `version: "0.1.44"
stack:
  core:
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
`)

	output := runConfigMigrate(t, tmpDir)

	if !strings.Contains(output, "nothing to convert") {
		t.Errorf("output should state what was checked, got: %q", output)
	}
	if !strings.Contains(output, "dva validate") {
		t.Errorf("output should point at 'dva validate', got: %q", output)
	}
}

// TestConfigMigrateNoOpStillReportsWhatNeedsHands.
//
// 'modes' has no mechanical target, so a file of nothing but modes is a no-op rewrite —
// and the least useful thing the command could do is print "nothing to convert" over it.
// The guidance is also what replaced TASK-069's hand-maintained list of deprecated
// section names: it is derived from the modes the config actually declares, so it cannot
// fall out of step with the sections the code knows about.
func TestConfigMigrateNoOpStillReportsWhatNeedsHands(t *testing.T) {
	tmpDir := t.TempDir()
	writeConfigMigrateFixture(t, tmpDir, `version: "0.1.44"
stack:
  core:
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
modes:
  dev:
    description: "local development"
    stack: [core]
    compose_services: [db]
`)

	output := runConfigMigrate(t, tmpDir)

	for _, want := range []string{"modes.dev", "plans.dev", "entries[<compose entry>].services"} {
		if !strings.Contains(output, want) {
			t.Errorf("the mode's split must be spelled out; missing %q in: %q", want, output)
		}
	}
}

// TestConfigMigrateNoOpMessage_AbsentWhenEntriesMigrated ensures the no-op message
// never appears alongside actual migration output — the success path already tells the
// user to run validate, so printing both would be confusing and redundant.
func TestConfigMigrateNoOpMessage_AbsentWhenEntriesMigrated(t *testing.T) {
	tmpDir := t.TempDir()
	writeConfigMigrateFixture(t, tmpDir, `version: "0.1.44"
stack:
  core:
    plugin: compose
    files: [compose.yml]
`)

	output := runConfigMigrate(t, tmpDir)

	if strings.Contains(output, "nothing to convert") {
		t.Errorf("no-op message must not appear when entries were migrated, got: %q", output)
	}
	for _, want := range []string{"stack.core → runners.compose", "not written"} {
		if !strings.Contains(output, want) {
			t.Errorf("expected preview output naming the conversion; missing %q in: %q", want, output)
		}
	}
}

// TestConfigMigrateConvertsEverySectionInOnePass.
//
// The three conversions are chained, so each reads the previous one's output. The one
// that matters here is order-after-applications: 'api' only becomes a stack entry
// during the run, and its order is only movable because the later step sees it.
func TestConfigMigrateConvertsEverySectionInOnePass(t *testing.T) {
	tmpDir := t.TempDir()
	writeConfigMigrateFixture(t, tmpDir, `version: "0.1.44"
stack:
  infra:
    order: 1
    plugin: compose
    files: [compose.yml]
applications:
  api:
    run: "cargo run"
plans:
  full:
    entries:
      - name: infra
      - name: api
`)

	stdout, stderr := captureStreams(t, func() {
		if err := configMigrateCmd.RunE(configMigrateCmd, []string{tmpDir}); err != nil {
			t.Fatalf("RunE error = %v", err)
		}
	})

	for _, want := range []string{
		"stack.infra → runners.compose",
		"applications.api → stack.api",
		"stack.infra.order: 1 → plans.full.entries[infra].order",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("the report must name every conversion; missing %q in: %q", want, stderr)
		}
	}

	// The preview is a config, not a transcript: redirecting stdout has to yield a
	// file DVA can read, which is only true if nothing about the run leaks into it.
	if err := config.VerifyMigrated([]byte(stdout)); err != nil {
		t.Errorf("the previewed config does not load: %v\n%s", err, stdout)
	}
}

// TestConfigMigrateWritesOnlyWithTheFlag: the preview is the default because migration
// rewrites a file the operator did not hand us a backup of.
func TestConfigMigrateWritesOnlyWithTheFlag(t *testing.T) {
	src := `version: "0.1.44"
stack:
  core:
    plugin: compose
    files: [compose.yml]
`
	tmpDir := t.TempDir()
	writeConfigMigrateFixture(t, tmpDir, src)
	path := filepath.Join(tmpDir, config.FileName)

	runConfigMigrate(t, tmpDir)

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(after) != src {
		t.Fatalf("the preview rewrote the file:\n%s", after)
	}

	configMigrateWrite = true
	defer func() { configMigrateWrite = false }()
	runConfigMigrate(t, tmpDir)

	after, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(after) == src {
		t.Fatal("--write left the file unchanged")
	}
	if err := config.VerifyMigrated(after); err != nil {
		t.Errorf("the written config does not load: %v\n%s", err, after)
	}
}
