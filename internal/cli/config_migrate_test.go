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
// the no-op branch and the migrated-YAML preview go to stdout, the "would migrate"
// summary goes to stderr — so a caller checking only one stream would miss the other.
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

// TestConfigMigrateNoOpMessage covers TASK-069: a config with no legacy compose
// declarations must say what migrate checked and point at 'dva validate', instead
// of the bare "no legacy compose declarations found" that reads as "nothing to do"
// even when validate has deprecation warnings of its own.
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

	output := captureOutput(t, func() {
		if err := configMigrateCmd.RunE(configMigrateCmd, []string{tmpDir}); err != nil {
			t.Fatalf("RunE error = %v", err)
		}
	})

	if !strings.Contains(output, "no legacy compose declarations found") {
		t.Errorf("output should state what was checked, got: %q", output)
	}
	if !strings.Contains(output, "dva validate") {
		t.Errorf("output should point at 'dva validate', got: %q", output)
	}
	for _, section := range []string{"modes", "stack.*.order", "applications"} {
		if !strings.Contains(output, section) {
			t.Errorf("output should name deprecated section %q, got: %q", section, output)
		}
	}
}

// TestConfigMigrateNoOpMessage_AbsentWhenEntriesMigrated ensures the no-op message
// introduced for TASK-069 never appears alongside actual migration output — the
// success path already tells the user to run validate (config_migrate.go's --write
// and preview branches), so printing both would be confusing and redundant.
func TestConfigMigrateNoOpMessage_AbsentWhenEntriesMigrated(t *testing.T) {
	tmpDir := t.TempDir()
	writeConfigMigrateFixture(t, tmpDir, `version: "0.1.44"
stack:
  core:
    plugin: compose
    files: [compose.yml]
`)

	output := captureOutput(t, func() {
		if err := configMigrateCmd.RunE(configMigrateCmd, []string{tmpDir}); err != nil {
			t.Fatalf("RunE error = %v", err)
		}
	})

	if strings.Contains(output, "no legacy compose declarations found") {
		t.Errorf("no-op message must not appear when entries were migrated, got: %q", output)
	}
	if !strings.Contains(output, "would migrate") {
		t.Errorf("expected preview output for a config with legacy compose, got: %q", output)
	}
}
