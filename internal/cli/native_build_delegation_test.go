// Package cli — regression tests for TASK-093.
//
// buildCmd's `build: native` branch ran interaction.build.replace itself, on the same
// len(ic.Replace) > 0 condition wrapWithHooks fires on. The wrapper always won, so the copy in
// compose.go was reachable only at DVA_HOOK_DEPTH>0 — `dva build` from inside another hook step,
// where the wrapper defers. The two disagreed: the copy printed to stdout with a four-space
// indent and the note before the commands, and it never consulted dryRun, so a nested
// `dva build --dry-run` executed for real. Measured at 2e6b89f.
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// writeNativeBuildConfig creates a build: native mode whose replace steps are observable without
// docker: a note-only step, an echo, and a `touch` whose marker file answers "did this actually
// run" independently of anything printed.
func writeNativeBuildConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	body := `version: "0.1.44"
modes:
  nativemode:
    build: native
interaction:
  build:
    replace:
      - step: note-only step
        note: BUILD-NOTE-VISIBLE
      - step: side effect step
        run: touch SIDE-EFFECT-HAPPENED
`
	if err := os.WriteFile(filepath.Join(dir, "dva.yml"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	// Same reset as writeStackArgsConfig: loadConfig/loadEnv memoize into package globals and
	// parseDvaFlags writes dryRun, so without this a test reuses the previous test's removed dir.
	oldDryRun := dryRun
	cfg, env = nil, nil
	t.Cleanup(func() {
		os.Chdir(oldWd)
		cfg, env = nil, nil
		dryRun = oldDryRun
	})
	return dir
}

// TestNativeBuildDelegatesToTheHookExecutor pins the two invocations to one renderer. The nested
// row is the one that used to take compose.go's copy; asserting both produce the same stderr is
// what makes a reintroduced second implementation fail here rather than in a user's terminal.
func TestNativeBuildDelegatesToTheHookExecutor(t *testing.T) {
	capture := func(t *testing.T, nested bool) (string, string) {
		t.Helper()
		writeNativeBuildConfig(t)
		if nested {
			t.Setenv(config.EnvHookDepthKey, "1")
		} else {
			// Explicitly empty rather than merely unset: a leaked value from an earlier test
			// would silently turn the control row into a second copy of the nested row.
			t.Setenv(config.EnvHookDepthKey, "")
		}

		stdout, stderr, err := captureValidateOutput(t, func() error {
			return buildCmd.RunE(buildCmd, []string{"--mode", "nativemode"})
		})
		if err != nil {
			t.Fatalf("build --mode nativemode: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}
		return stdout, stderr
	}

	// Pointers, not strings: empty stderr is a real outcome here — it is exactly what the
	// deleted copy produced, having written to stdout instead — so "" must not be readable as
	// "the subtest never ran".
	var normalErr, nestedErr *string
	t.Run("the normal path renders through runHookSteps", func(t *testing.T) {
		stdout, stderr := capture(t, false)
		normalErr = &stderr
		if !strings.Contains(stderr, "[hook:replace:build]") {
			t.Errorf("stderr does not carry the hook executor's label:\n%s", stderr)
		}
		if !strings.Contains(stderr, "BUILD-NOTE-VISIBLE") {
			t.Errorf("the note did not reach stderr:\n%s", stderr)
		}
		if strings.Contains(stdout, "BUILD-NOTE-VISIBLE") {
			t.Errorf("the note reached stdout as well — two renderers again:\n%s", stdout)
		}
	})

	t.Run("the nested path renders identically", func(t *testing.T) {
		_, stderr := capture(t, true)
		nestedErr = &stderr
		if !strings.Contains(stderr, "[hook:replace:build]") {
			t.Errorf("DVA_HOOK_DEPTH=1 did not reach the hook executor:\n%s", stderr)
		}
	})

	t.Run("byte-identical", func(t *testing.T) {
		if normalErr == nil || nestedErr == nil {
			t.Fatal("a preceding subtest did not run; nothing to compare")
		}
		if *normalErr != *nestedErr {
			t.Errorf("the two paths still render differently\nnormal (%dB):\n%s\nnested (%dB):\n%s",
				len(*normalErr), *normalErr, len(*nestedErr), *nestedErr)
		}
	})
}

// TestNativeBuildHonoursDryRunWhenNested is the half that was not cosmetic. compose.go's copy had
// no dryRun branch at all, so with DVA_HOOK_DEPTH>0 the steps executed however the flag was set.
// The marker file is the assertion — printed output cannot distinguish "described" from "ran".
func TestNativeBuildHonoursDryRunWhenNested(t *testing.T) {
	cases := []struct {
		name   string
		nested bool
	}{
		// The normal path is the control: it honoured dryRun before this change and must
		// continue to, or a test that only checks the nested path would pass on a fix that
		// broke both.
		{"normal path", false},
		{"nested path", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeNativeBuildConfig(t)
			if tc.nested {
				t.Setenv(config.EnvHookDepthKey, "1")
			} else {
				t.Setenv(config.EnvHookDepthKey, "")
			}

			_, stderr, err := captureValidateOutput(t, func() error {
				return buildCmd.RunE(buildCmd, []string{"--dry-run", "--mode", "nativemode"})
			})
			if err != nil {
				t.Fatalf("build --dry-run --mode nativemode: %v\nstderr: %s", err, stderr)
			}

			marker := filepath.Join(dir, "SIDE-EFFECT-HAPPENED")
			if _, statErr := os.Stat(marker); statErr == nil {
				t.Errorf("--dry-run executed the step for real (%s exists)\nstderr: %s", marker, stderr)
			}
			if !strings.Contains(stderr, "[dry-run]") {
				t.Errorf("nothing was described as a dry run, so the flag may not have been read at all:\n%s", stderr)
			}
		})
	}
}
