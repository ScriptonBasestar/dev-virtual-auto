// Package cli — TASK-168 regression tests.
//
// `executeParallelBatch` gave each step a bytes.Buffer and flushed the buffers in declaration
// order after the batch joined, which read as "the output is serialised" and was not: the
// buffer received only the lines dva composed, while the commands ran with os.Stdout wired
// straight to the terminal. Measured 2026-08-03 on v0.1.44, two steps of five lines each:
//
//	  ⚡ Running 2 steps in parallel...
//	BETA-1                       ← children, interleaved and unattributable
//	ALPHA-1
//	…
//	  [1/3] alpha                ← the labels, after the output they name
//	    $ for i in 1 2 3 4 5; do echo ALPHA-$i; …
//
// These tests assert the two properties that were violated: every line is attributable, and no
// label follows its own output.
package cli

import (
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// splitWriter delivers every Write in two pieces with a scheduling point between them, so a
// caller that does not hold a lock across its own Write can be interrupted mid-line. Guarding
// the Builder is the writer-under-test's job — that is the whole point — so a data race here
// under `go test -race` is the same finding as an interleaved line below.
type splitWriter struct{ sb *strings.Builder }

func (s *splitWriter) Write(p []byte) (int, error) {
	half := len(p) / 2
	_, _ = s.sb.Write(p[:half])
	runtime.Gosched()
	_, _ = s.sb.Write(p[half:])
	return len(p), nil
}

// lineIndex returns the index of the first line containing want, or -1.
func lineIndex(lines []string, want string) int {
	for i, l := range lines {
		if strings.Contains(l, want) {
			return i
		}
	}
	return -1
}

// TestParallelBatchAttributesEveryCommandLine is the criterion test: a child's output must
// carry the step that produced it. Before the fix these lines arrived bare.
func TestParallelBatchAttributesEveryCommandLine(t *testing.T) {
	e := makeEnv(t)
	c := makeConfig(t)
	batch := []config.ProvisionItem{
		{Step: "alpha", Parallel: true, Cmd: "echo ALPHA-1; echo ALPHA-2"},
		{Step: "beta", Parallel: true, Cmd: "echo BETA-1; echo BETA-2"},
	}

	out := captureStdout(t, func() {
		if err := executeParallelBatch(e, c, batch, 0, 2, false); err != nil {
			t.Errorf("executeParallelBatch: %v", err)
		}
	})
	t.Logf("batch output:\n%s", out)

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	for _, l := range lines {
		// Only the children's own output is under test; the batch header is dva's and is
		// printed once, before any step exists to attribute it to.
		if !strings.Contains(l, "ALPHA-") && !strings.Contains(l, "BETA-") {
			continue
		}
		want := "alpha"
		if strings.Contains(l, "BETA-") {
			want = "beta"
		}
		// The step name and the separator together: a line that merely happened to contain
		// the word would satisfy a looser check, and `echo ALPHA-1` does contain "alpha".
		if !strings.Contains(l, want+" │ ") && !strings.Contains(l, want+"  │ ") {
			t.Errorf("unattributed output line %q — it names no step", l)
		}
	}

	// Both steps ran; a fix that attributed one and swallowed the other would otherwise pass
	// the loop above vacuously.
	for _, want := range []string{"ALPHA-1", "ALPHA-2", "BETA-1", "BETA-2"} {
		if lineIndex(lines, want) < 0 {
			t.Errorf("%s missing from the batch output entirely", want)
		}
	}
}

// TestParallelBatchLabelPrecedesItsOutput is the other half of TASK-168, and the half the old
// buffering got exactly backwards: the `$ …` echo was buffered until the batch joined while the
// command it echoed had already printed.
func TestParallelBatchLabelPrecedesItsOutput(t *testing.T) {
	e := makeEnv(t)
	c := makeConfig(t)
	batch := []config.ProvisionItem{
		{Step: "alpha", Parallel: true, Cmd: "echo ALPHA-1"},
		{Step: "beta", Parallel: true, Cmd: "echo BETA-1"},
	}

	out := captureStdout(t, func() {
		if err := executeParallelBatch(e, c, batch, 0, 2, false); err != nil {
			t.Errorf("executeParallelBatch: %v", err)
		}
	})
	t.Logf("batch output:\n%s", out)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")

	for _, tc := range []struct{ echo, output string }{
		{"$ echo ALPHA-1", "ALPHA-1"},
		{"$ echo BETA-1", "BETA-1"},
	} {
		echoAt := lineIndex(lines, tc.echo)
		if echoAt < 0 {
			t.Fatalf("no line echoes %q — nothing to order against", tc.echo)
		}
		// The echo line itself contains the output string, so search after it.
		outAt := lineIndex(lines[echoAt+1:], tc.output)
		if outAt < 0 {
			t.Fatalf("%q never appeared after its own echo", tc.output)
		}
	}
}

// TestParallelDryRunStaysInDeclarationOrder pins the deliberate asymmetry. A dry run keeps the
// buffered shape, because no child runs — nothing can escape the buffer — and a *plan* read in
// goroutine-completion order would be a worse answer to "what will happen" than one read in the
// order the steps are written.
func TestParallelDryRunStaysInDeclarationOrder(t *testing.T) {
	e := makeEnv(t)
	c := makeConfig(t)
	batch := []config.ProvisionItem{
		{Step: "alpha", Parallel: true, Cmd: "echo ALPHA"},
		{Step: "beta", Parallel: true, Cmd: "echo BETA"},
		{Step: "gamma", Parallel: true, Cmd: "echo GAMMA"},
	}

	// Repeated, because a scheduling-order bug passes a single run roughly half the time.
	for i := range 20 {
		out := captureStdout(t, func() {
			if err := executeParallelBatch(e, c, batch, 0, 3, true); err != nil {
				t.Errorf("executeParallelBatch: %v", err)
			}
		})
		lines := strings.Split(out, "\n")
		a, b, g := lineIndex(lines, "alpha"), lineIndex(lines, "beta"), lineIndex(lines, "gamma")
		if a < 0 || b < 0 || g < 0 {
			t.Fatalf("run %d: a step is missing from the plan:\n%s", i, out)
		}
		if a >= b || b >= g {
			t.Fatalf("run %d: plan out of declaration order (alpha=%d beta=%d gamma=%d):\n%s", i, a, b, g, out)
		}
	}
}

// TestStepPrefixWriter covers the writer itself, including the two cases a line-splitting
// writer gets wrong: output that does not end in a newline, and a line arriving in pieces.
func TestStepPrefixWriter(t *testing.T) {
	t.Run("splits and prefixes complete lines", func(t *testing.T) {
		var sb strings.Builder
		w := newStepPrefixWriters(&sb, []string{"one"})[0]
		if _, err := w.Write([]byte("a\nb\n")); err != nil {
			t.Fatal(err)
		}
		if got, want := sb.String(), "one │ a\none │ b\n"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("a partial line waits for its newline", func(t *testing.T) {
		var sb strings.Builder
		w := newStepPrefixWriters(&sb, []string{"one"})[0]
		_, _ = w.Write([]byte("half"))
		if sb.String() != "" {
			t.Errorf("emitted %q before the line was complete", sb.String())
		}
		_, _ = w.Write([]byte("-line\n"))
		if got, want := sb.String(), "one │ half-line\n"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("Flush emits a line with no trailing newline", func(t *testing.T) {
		var sb strings.Builder
		w := newStepPrefixWriters(&sb, []string{"one"})[0]
		_, _ = w.Write([]byte("no newline"))
		w.Flush()
		if got, want := sb.String(), "one │ no newline\n"; got != want {
			t.Errorf("got %q, want %q — the last line of a command was dropped", got, want)
		}
	})

	t.Run("blank lines stay blank", func(t *testing.T) {
		var sb strings.Builder
		w := newStepPrefixWriters(&sb, []string{"one"})[0]
		_, _ = w.Write([]byte("a\n\nb\n"))
		if got, want := sb.String(), "one │ a\n\none │ b\n"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("CRLF loses only the carriage return", func(t *testing.T) {
		var sb strings.Builder
		w := newStepPrefixWriters(&sb, []string{"one"})[0]
		_, _ = w.Write([]byte("windows\r\n"))
		if got, want := sb.String(), "one │ windows\n"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("labels are padded to a common width", func(t *testing.T) {
		ws := newStepPrefixWriters(&strings.Builder{}, []string{"short", "much-longer"})
		if len(ws[0].prefix) != len(ws[1].prefix) {
			t.Errorf("prefixes %q and %q are not the same width — the column is broken",
				ws[0].prefix, ws[1].prefix)
		}
	})

	t.Run("padding counts runes, not bytes", func(t *testing.T) {
		// "가나다" is 3 runes and 9 bytes. Padded by byte length the two prefixes would
		// differ by 6 columns on screen while measuring equal in len().
		ws := newStepPrefixWriters(&strings.Builder{}, []string{"가나다", "abc"})
		var sb strings.Builder
		for _, w := range ws {
			w.out = &sb
			_, _ = w.Write([]byte("x\n"))
		}
		lines := strings.Split(strings.TrimRight(sb.String(), "\n"), "\n")
		if len([]rune(lines[0])) != len([]rune(lines[1])) {
			t.Errorf("rune widths differ: %q vs %q", lines[0], lines[1])
		}
	})

	t.Run("concurrent steps never interleave mid-line", func(t *testing.T) {
		// The first version of this subtest wrote to a strings.Builder and passed even with
		// the shared lock removed, because emit issues exactly one Write per line and one
		// Write to a Builder cannot be split. That made it a test of fmt, not of the lock.
		//
		// splitWriter restores the property the lock is actually for: an underlying writer
		// that does not deliver a Write whole. os.Stdout is that writer as soon as a line
		// exceeds PIPE_BUF and stdout is a pipe (`dva provision | tee log`), where the kernel
		// stops promising atomicity.
		var sb strings.Builder
		ws := newStepPrefixWriters(&splitWriter{sb: &sb}, []string{"one", "two"})

		var wg sync.WaitGroup
		for _, w := range ws {
			wg.Add(1)
			go func(w *stepPrefixWriter) {
				defer wg.Done()
				for range 200 {
					_, _ = w.Write([]byte("0123456789\n"))
				}
			}(w)
		}
		wg.Wait()

		for l := range strings.SplitSeq(strings.TrimRight(sb.String(), "\n"), "\n") {
			if !strings.HasSuffix(l, "0123456789") {
				t.Fatalf("interleaved line: %q", l)
			}
		}
	})
}
