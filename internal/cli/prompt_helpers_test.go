package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Helpers for the tests that drive a confirmation prompt or a --dry-run preview.
//
// These lived in clean_prompt_test.go alongside the four `dva clean` tests they were written
// for. The command is gone — teardown is `dva down <plan> --purge` — and plan_purge_test.go
// asserts the same four properties on the new path, so only the helpers moved here. The
// TASK-166/170/171 reasoning they encode is unchanged; it is the invocation that moved.

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
	useStdin(t, f)
}

// stdinFrom points os.Stdin at a file holding exactly content, for the arms of the prompt
// where somebody does answer.
//
// A regular file rather than an os.Pipe: a pipe whose write end stays open never reaches EOF,
// so a fix that stopped consuming stdin would hang the suite instead of failing it. A file
// ends on its own.
func stdinFrom(t *testing.T, content string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stdin")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write stdin fixture: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open stdin fixture: %v", err)
	}
	useStdin(t, f)
}

// useStdin swaps os.Stdin for the duration of the test. fmt.Scanln resolves os.Stdin at call
// time, which is what makes the swap reach the prompt at all.
func useStdin(t *testing.T, f *os.File) {
	t.Helper()
	old := os.Stdin
	os.Stdin = f
	t.Cleanup(func() { os.Stdin = old; _ = f.Close() })
}

// captureBothStreams redirects both streams for the duration of fn and returns stderr.
//
// Both are redirected because the defect and its fix straddle them: the prompt is written to
// stderr and "Aborted." used to go to stdout, so a run that still wrote to stdout would
// scatter output into the test log instead of being contained. Nothing asserts on stdout
// today, so it is drained and discarded rather than returned.
//
// One goroutine per stream, not one draining both in turn: a caller that writes more than a
// pipe buffer to stderr while the reader is still blocked on stdout would deadlock, and the
// prompt path writes to both.
func captureBothStreams(t *testing.T, fn func()) (stderr string) {
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
	readOut()
	return readErr()
}

// enableDryRun drives the global rather than the flag because RunE is invoked here without
// cobra's Execute, which is what would normally populate the persistent flag.
func enableDryRun(t *testing.T) {
	t.Helper()
	old := dryRun
	dryRun = true
	t.Cleanup(func() { dryRun = old })
}
