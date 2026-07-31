package output

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// captureStdout swaps a pipe in for os.Stdout, runs fn, and returns what fn wrote.
//
// The pipe is drained on a goroutine rather than after fn returns: a document larger than
// the pipe buffer would otherwise block the writer forever and hang the test instead of
// failing it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close write end: %v", err)
	}
	out := <-done
	if err := r.Close(); err != nil {
		t.Fatalf("close read end: %v", err)
	}
	return out
}

// brokenStdout points os.Stdout at a pipe whose read end is already closed, so the next
// write fails with EPIPE, and returns a restore func.
//
// This is a real errno on a real os.File, not a fake, and it is only survivable because Go
// decides whether EPIPE becomes a fatal SIGPIPE from the descriptor NUMBER — os.File sets
// a flag at construction when its fd is 1 or 2 — rather than from which variable the file
// happens to be assigned to. A pipe in a test binary gets a descriptor well above 2, so the
// write reports the error instead of killing the process. The same code path against the
// process's actual stdout dies at exit 141, which is why a broken pipe is NOT the trigger
// for the defect this package had; a full filesystem is.
func brokenStdout(t *testing.T) func() {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close read end: %v", err)
	}
	old := os.Stdout
	os.Stdout = w
	return func() {
		os.Stdout = old
		_ = w.Close()
	}
}

// shortWriter accepts limit bytes and then fails, which is the one case a real file cannot
// be made to produce on demand: a write that delivered something and still returned an
// error. printDocument keys stdoutHasDocument on the byte count precisely so this case sets
// the flag, so a writer that can produce it is the only way to tell the intended behaviour
// apart from the accidental one.
type shortWriter struct {
	limit int
}

var errShortWrite = errors.New("device out of space")

func (s shortWriter) Write(p []byte) (int, error) {
	if len(p) <= s.limit {
		return len(p), nil
	}
	return s.limit, errShortWrite
}

func TestPrintJSONReturnsWriteError(t *testing.T) {
	ResetStdoutDocument()
	defer ResetStdoutDocument()
	defer brokenStdout(t)()

	err := PrintJSON(map[string]string{"key": "value"})
	if err == nil {
		t.Fatal("PrintJSON returned nil for a write that never landed; " +
			"the caller has no way left to know stdout is empty")
	}
	if !strings.Contains(err.Error(), "pipe") {
		t.Errorf("the underlying write error must survive, not be replaced; got %v", err)
	}
}

func TestPrintYAMLReturnsWriteError(t *testing.T) {
	ResetStdoutDocument()
	defer ResetStdoutDocument()
	defer brokenStdout(t)()

	err := PrintYAML(map[string]string{"key": "value"})
	if err == nil {
		t.Fatal("PrintYAML returned nil for a write that never landed")
	}
}

// TestStdoutHasDocumentTracksBytesDelivered pins the contract the package comment states.
// The three rows are the three outcomes a write has, and they must not collapse into two:
// the middle one is the reason the flag is keyed on the byte count.
func TestStdoutHasDocumentTracksBytesDelivered(t *testing.T) {
	const doc = "0123456789"

	cases := []struct {
		name    string
		writer  io.Writer
		wantErr bool
		wantSet bool
		why     string
	}{
		{
			name:    "a write that fully lands sets the flag",
			writer:  io.Discard,
			wantErr: false,
			wantSet: true,
			why:     "stdout holds a whole document; a failure envelope after it would be a second one",
		},
		{
			name:    "a write that lands nothing leaves the flag clear",
			writer:  shortWriter{limit: 0},
			wantErr: true,
			wantSet: false,
			why:     "stdout is still empty, so the envelope is the only thing that can describe the failure",
		},
		{
			name:    "a write that lands part of the document sets the flag",
			writer:  shortWriter{limit: 4},
			wantErr: true,
			wantSet: true,
			why:     "stdout holds a truncated document; appending an envelope would make it two half-objects",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ResetStdoutDocument()
			defer ResetStdoutDocument()

			if StdoutHasDocument() {
				t.Fatal("ResetStdoutDocument did not clear the flag")
			}

			err := printDocument(tc.writer, doc)
			if tc.wantErr && err == nil {
				t.Error("the write failed and printDocument reported success")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("the write succeeded and printDocument reported %v", err)
			}
			if got := StdoutHasDocument(); got != tc.wantSet {
				t.Errorf("StdoutHasDocument() = %v, want %v — %s", got, tc.wantSet, tc.why)
			}
		})
	}
}

func TestPrintJSONRoundTrips(t *testing.T) {
	ResetStdoutDocument()
	defer ResetStdoutDocument()

	in := map[string]any{"name": "dva", "count": 3, "tags": []any{"a", "b"}}

	var out string
	var err error
	out = captureStdout(t, func() { err = PrintJSON(in) })
	if err != nil {
		t.Fatalf("PrintJSON: %v", err)
	}

	var got map[string]any
	if decodeErr := json.Unmarshal([]byte(out), &got); decodeErr != nil {
		t.Fatalf("PrintJSON did not emit parseable JSON: %v\ngot: %q", decodeErr, out)
	}
	if got["name"] != "dva" || got["count"] != float64(3) {
		t.Errorf("round trip lost data: %#v", got)
	}

	// Exactly one trailing newline. PrintJSON used to reach stdout through fmt.Println,
	// which supplied it; the write now carries it explicitly, and a consumer reading
	// line-delimited documents would notice either its loss or its duplication.
	if !strings.HasSuffix(out, "}\n") || strings.HasSuffix(out, "}\n\n") {
		t.Errorf("want exactly one trailing newline after the closing brace; got %q",
			out[max(0, len(out)-8):])
	}

	if !StdoutHasDocument() {
		t.Error("a document reached stdout and the flag says otherwise")
	}
}

func TestPrintYAMLRoundTrips(t *testing.T) {
	ResetStdoutDocument()
	defer ResetStdoutDocument()

	in := map[string]any{"name": "dva", "count": 3}

	var out string
	var err error
	out = captureStdout(t, func() { err = PrintYAML(in) })
	if err != nil {
		t.Fatalf("PrintYAML: %v", err)
	}

	var got map[string]any
	if decodeErr := yaml.Unmarshal([]byte(out), &got); decodeErr != nil {
		t.Fatalf("PrintYAML did not emit parseable YAML: %v\ngot: %q", decodeErr, out)
	}
	if got["name"] != "dva" || got["count"] != 3 {
		t.Errorf("round trip lost data: %#v", got)
	}

	// yaml.Marshal terminates its own output, so PrintYAML must not add a second newline —
	// this is the asymmetry with PrintJSON that the two call sites encode.
	if strings.HasSuffix(out, "\n\n") {
		t.Errorf("PrintYAML added a newline yaml.Marshal had already supplied; got %q", out)
	}

	if !StdoutHasDocument() {
		t.Error("a document reached stdout and the flag says otherwise")
	}
}

// TestMarshalFailureLeavesStdoutClean feeds the same unmarshalable value to both printers.
//
// A channel has no JSON or YAML representation. encoding/json returns UnsupportedTypeError;
// yaml.v3 used to panic — its own recover re-panics anything that is not its private yamlError
// type — so before TASK-120 the YAML row here crashed the test binary instead of failing it,
// and PrintYAML's error branch read as 75 percent coverage because the statement was
// unreachable. The recover in PrintYAML turns that panic into a returned error, and both rows
// now assert the same contract: a marshal failure is reported, and stdout stays empty.
func TestMarshalFailureLeavesStdoutClean(t *testing.T) {
	unmarshalable := make(chan int)

	t.Run("PrintJSON", func(t *testing.T) {
		ResetStdoutDocument()
		defer ResetStdoutDocument()

		if err := PrintJSON(unmarshalable); err == nil {
			t.Error("PrintJSON accepted a value it cannot marshal")
		}
		if StdoutHasDocument() {
			t.Error("nothing was printed, so stdout must still count as empty")
		}
	})

	t.Run("PrintYAML", func(t *testing.T) {
		ResetStdoutDocument()
		defer ResetStdoutDocument()

		if err := PrintYAML(unmarshalable); err == nil {
			t.Error("PrintYAML accepted a value it cannot marshal")
		}
		if StdoutHasDocument() {
			t.Error("nothing was printed, so stdout must still count as empty")
		}
	})
}

// yamlErrorFailer makes yaml.Marshal return a plain error rather than panic, so the marshal-
// error branch of PrintYAML — the `if err != nil { return err }` the recover sits beside — is
// exercised separately from the unsupported-kind panic the channel above drives. Without this
// row that branch reads as uncovered and looks like the dead code TASK-120 removed.
type yamlErrorFailer struct{}

func (yamlErrorFailer) MarshalYAML() (any, error) {
	return nil, errors.New("intentional yaml marshal error")
}

func TestPrintYAMLReturnsMarshalError(t *testing.T) {
	ResetStdoutDocument()
	defer ResetStdoutDocument()

	err := PrintYAML(yamlErrorFailer{})
	if err == nil {
		t.Fatal("PrintYAML returned nil for a value whose MarshalYAML failed")
	}
	if !strings.Contains(err.Error(), "intentional yaml marshal error") {
		t.Errorf("the marshal error must survive; got %v", err)
	}
	if StdoutHasDocument() {
		t.Error("nothing was printed, so stdout must still count as empty")
	}
}
