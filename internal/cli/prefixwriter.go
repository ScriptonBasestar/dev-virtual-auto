package cli

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"unicode/utf8"
)

// stepPrefixWriter attributes one parallel step's output by tagging every line it emits.
//
// It exists because `executeParallelBatch` used to buffer each step and flush the buffers in
// declaration order, which looked like it serialised the batch but did not: the buffer only
// ever received the lines dva itself composed. The commands ran with os.Stdout wired straight
// to the terminal, so their output arrived *before* the labels describing it and belonged to
// no step a reader could name (TASK-168).
//
// Prefixing was chosen over buffering the children. Buffering produces tidier per-step blocks,
// but a parallel batch exists precisely because its steps are slow, and a buffered batch is
// silent until it joins — a step that hangs is then indistinguishable from a step that is
// working, with nothing on screen to say which of the four it is. Streaming keeps the property
// the sequential path already has (output appears as it is produced) and adds the attribution
// the sequential path gets for free from running one thing at a time.
//
// Concurrency: every writer in a batch shares one mutex, because they share one terminal. A
// line is written under that lock, so no two steps can interleave mid-line.
//
// Known limit: a child that redraws in place with bare \r (a progress bar) has no line breaks
// to split on, so its frames accumulate and land as one long line at Flush. That is a
// regression only in theory — this path is unreachable below two concurrent steps
// (provision.go routes a one-step batch to the sequential executor), and two children
// redrawing on the same terminal already destroyed each other's output before this existed.
type stepPrefixWriter struct {
	mu      *sync.Mutex
	out     io.Writer
	prefix  string
	pending []byte
}

// newStepPrefixWriters builds one writer per step over a shared lock, with the labels padded
// to a common width so the output forms a readable column.
func newStepPrefixWriters(out io.Writer, labels []string) []*stepPrefixWriter {
	width := 0
	for _, l := range labels {
		if n := utf8.RuneCountInString(l); n > width {
			width = n
		}
	}

	mu := &sync.Mutex{}
	writers := make([]*stepPrefixWriter, len(labels))
	for i, l := range labels {
		pad := width - utf8.RuneCountInString(l)
		writers[i] = &stepPrefixWriter{
			mu:  mu,
			out: out,
			// Padding is counted in runes, not bytes: a step named in Hangul or with an
			// emoji would otherwise pad by its UTF-8 length and break the column it is
			// here to create.
			prefix: l + spaces(pad) + " │ ",
		}
	}
	return writers
}

func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	return string(bytes.Repeat([]byte{' '}, n))
}

func (w *stepPrefixWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.pending = append(w.pending, p...)
	for {
		i := bytes.IndexByte(w.pending, '\n')
		if i < 0 {
			break
		}
		w.emit(w.pending[:i])
		w.pending = w.pending[i+1:]
	}
	// Always the full length: a short count makes io.Copy report ErrShortWrite, and the
	// bytes held back here are held deliberately, not dropped.
	return len(p), nil
}

// Flush emits whatever the last line lacked a newline to complete. Called once per step after
// its commands have finished, or the final line of a command that does not end with \n is lost.
func (w *stepPrefixWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.pending) > 0 {
		w.emit(w.pending)
		w.pending = nil
	}
}

// emit writes one prefixed line. The caller holds the lock.
func (w *stepPrefixWriter) emit(line []byte) {
	// CRLF from a Windows child: strip the \r so it does not land between the line and the
	// newline and blank the line on a terminal that honours it.
	line = bytes.TrimSuffix(line, []byte{'\r'})

	// A blank line stays blank rather than becoming a prefix with trailing whitespace —
	// notes are rendered with blank lines either side, and those separate the block, they
	// do not belong to it.
	if len(line) == 0 {
		_, _ = fmt.Fprintln(w.out)
		return
	}
	_, _ = fmt.Fprintf(w.out, "%s%s\n", w.prefix, line)
}
