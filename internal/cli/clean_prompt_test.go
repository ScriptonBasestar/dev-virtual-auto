package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stdinEOF points os.Stdin at /dev/null for the test.
//
// fmt.Scanln resolves os.Stdin at call time, so this reproduces exactly what a pipe or a CI
// runner hands the prompt: immediate EOF, `answer` left empty, which is not "y". It also
// removes the only way these tests could hang — `go test` usually supplies an empty stdin,
// but "usually" is not a property worth betting a suite on.
func stdinEOF(t *testing.T) {
	t.Helper()
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	old := os.Stdin
	os.Stdin = f
	t.Cleanup(func() { os.Stdin = old; _ = f.Close() })
}

// captureCleanOutput collects both streams for the duration of fn.
//
// Both are needed because the defect and its fix straddle them: the prompt is written to
// stderr, "Aborted." used to go to stdout, and the assertion that they now travel together
// can only be made by watching the stream that should have fallen silent.
func captureCleanOutput(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()

	grab := func(target **os.File) (restore func(), read func() string) {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("pipe: %v", err)
		}
		old := *target
		*target = w
		done := make(chan string, 1)
		go func() {
			var b bytes.Buffer
			_, _ = b.ReadFrom(r)
			done <- b.String()
		}()
		return func() { _ = w.Close(); *target = old }, func() string { return <-done }
	}

	restoreOut, readOut := grab(&os.Stdout)
	restoreErr, readErr := grab(&os.Stderr)
	fn()
	restoreOut()
	restoreErr()
	return readOut(), readErr()
}

// setCleanFlags sets flags on the package-level cleanCmd and restores them after. Cobra
// flags live on the command, not on the invocation, so they outlast the test that set them.
func setCleanFlags(t *testing.T, names ...string) {
	t.Helper()
	for _, f := range names {
		if err := cleanCmd.Flags().Set(f, "true"); err != nil {
			t.Fatalf("set --%s: %v", f, err)
		}
		t.Cleanup(func() { _ = cleanCmd.Flags().Set(f, "false") })
	}
}

// setDryRun drives the global rather than the flag because RunE is invoked here without
// cobra's Execute, which is what would normally populate the persistent flag.
func setDryRun(t *testing.T, v bool) {
	t.Helper()
	old := dryRun
	dryRun = v
	t.Cleanup(func() { dryRun = old })
}

// cleanMarkerFixture builds the stand-in app plus a provision marker, and returns the marker
// path. The marker is the load-bearing artefact in both directions: a dry run must leave it
// and say it would delete it, and a declined real run must leave it untouched.
func cleanMarkerFixture(t *testing.T) string {
	t.Helper()
	dir, _ := standInApp(t)
	marker := filepath.Join(dir, ".sb", "dva", "provisioned-default")
	if err := os.WriteFile(marker, []byte("x"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	return marker
}

// TestCleanDryRunSkipsTheDestructionPrompt covers TASK-170.
//
// The prompt asks consent for a deletion; a dry run deletes nothing, so there is nothing to
// consent to. The gate never consulted dryRun, and answering the prompt's own documented
// default (N) returned before the preview ran. Non-interactively there was no default to
// override: Scanln gets EOF, `answer` stays empty, and `dva clean --volumes --dry-run`
// printed "Aborted." and nothing else at rc 0, so no script noticed either.
//
// Note the absence of --force. That was the only way to reach the preview from a script, and
// on a real run it is the flag that removes the safety.
func TestCleanDryRunSkipsTheDestructionPrompt(t *testing.T) {
	marker := cleanMarkerFixture(t)
	setCleanFlags(t, "volumes")
	setDryRun(t, true)
	stdinEOF(t)

	_, out := captureCleanOutput(t, func() { _ = cleanCmd.RunE(cleanCmd, nil) })

	for _, unwanted := range []string{"Continue?", "Aborted."} {
		if strings.Contains(out, unwanted) {
			t.Errorf("clean --volumes --dry-run still emits %q; a preview must not ask consent "+
				"for a deletion it will not perform. Output:\n%s", unwanted, out)
		}
	}
	// Assert the preview and not merely the prompt's absence: a RunE that returned early for
	// some unrelated reason would also print no prompt, and would pass a test that only
	// checked for silence.
	if !strings.Contains(out, "would delete provision marker") {
		t.Errorf("the preview never ran, so skipping the prompt bought nothing:\n%s", out)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("dry run deleted the provision marker: %v", err)
	}
}

// TestCleanWithoutDryRunStillPrompts is the safety control, and the half of TASK-170 that must
// not move: exempting the preview is only defensible if the real path is untouched.
//
// EOF stdin stands in for the pipe or the CI runner. The marker assertion is what proves the
// abort stopped the work rather than merely printing about it — clearProvisionMarkers runs a
// few lines below the prompt, so a gate that fell through would delete this file for real
// before anything reached docker.
//
// The stdout assertion pins the stream move. The prompt went to stderr and "Aborted." to
// stdout, so `2>/dev/null` showed a verdict with nothing saying what had been aborted, and a
// script reading stdout for the preview got the verdict mixed into it.
func TestCleanWithoutDryRunStillPrompts(t *testing.T) {
	marker := cleanMarkerFixture(t)
	setCleanFlags(t, "volumes")
	setDryRun(t, false)
	stdinEOF(t)

	stdout, stderr := captureCleanOutput(t, func() { _ = cleanCmd.RunE(cleanCmd, nil) })

	for _, want := range []string{"VOLUMES (data loss!)", "Continue?", "Aborted."} {
		if !strings.Contains(stderr, want) {
			t.Errorf("a real `clean --volumes` must still ask before destroying; stderr is "+
				"missing %q:\n%s", want, stderr)
		}
	}
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("the run proceeded past a declined prompt and deleted the marker: %v", err)
	}
	if strings.Contains(stdout, "Aborted.") {
		t.Errorf("Aborted. is on stdout, split from the prompt it answers on stderr:\n%s", stdout)
	}
}
