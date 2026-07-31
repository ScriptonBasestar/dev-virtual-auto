package runner

import (
	"os"
	"strings"
	"testing"
)

// TestExplainReportsTheArgumentsThatWillBePassed pins the plan to the same source the runners
// read — commandArgs — rather than to cmd.Argv. Before TASK-101, `dva run rails console --explain`
// printed no Arguments line at all while the exec carried `server -p 3000 -b 0.0.0.0`, so the one
// tool a user has for checking a command by hand was the tool that hid the defect.
//
// Explain writes through fmt.Println rather than an injected io.Writer, so it is read here with
// captureStdout (inert_step_test.go), the same os.Stdout interception the step tests use.
func TestExplainReportsTheArgumentsThatWillBePassed(t *testing.T) {
	cases := []struct {
		name string
		cmd  ResolvedCommand
		// want is the exact Arguments line; empty means there must be no such line at all.
		want string
	}{
		{
			name: "inherited default_args are shown, and attributed",
			cmd:  ResolvedCommand{Command: "console", DefaultArgs: "server -p 3000"},
			want: "Arguments: server -p 3000  (from default_args)",
		},
		{
			name: "an explicit invocation is shown unannotated",
			cmd:  ResolvedCommand{Command: "echo", Argv: []string{"EXTRA"}},
			want: "Arguments: EXTRA",
		},
		{
			// The precedence commandArgs implements, made visible: argv wins outright, it does
			// not append. Without this row the annotation could be wrong in the common case.
			name: "argv wins over default_args, and the annotation goes away with it",
			cmd:  ResolvedCommand{Command: "echo", Argv: []string{"EXTRA"}, DefaultArgs: "server -p 3000"},
			want: "Arguments: EXTRA",
		},
		{
			// The negative control: nothing to pass must print nothing, or every plan would
			// grow an empty Arguments line and the assertions above would pass vacuously.
			name: "nothing to pass prints no Arguments line",
			cmd:  ResolvedCommand{Command: "echo"},
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := tc.cmd
			out := captureStdout(t, func() { Explain(&cmd, false) })

			if !strings.Contains(out, "Command: "+cmd.Command) {
				t.Fatalf("plan does not look like a plan — captured %q", out)
			}

			var got string
			for line := range strings.SplitSeq(out, "\n") {
				if strings.HasPrefix(line, "Arguments:") {
					got = line
				}
			}
			if got != tc.want {
				t.Errorf("Arguments line = %q, want %q\nfull plan:\n%s", got, tc.want, out)
			}
		})
	}
}

// brokenStdout returns a restore func after pointing os.Stdout at a pipe whose read end is
// already closed, so the next write fails with EPIPE. It is the same trick internal/output's
// tests use (TASK-114), lifted here because Explain writes through output.PrintJSON which reads
// os.Stdout live rather than through a captured pointer.
//
// EPIPE on the process's real stdout (fd 1) would escalate to a fatal SIGPIPE at exit 141 — Go
// decides that from the descriptor NUMBER, not from the os.Stdout variable. A test process that
// swapped fd 1 for a closed pipe would die instead of returning an error, so the swap stays at
// the variable: the pipe is a fresh descriptor the runtime does not treat as fatal. That is why
// this is a valid in-process test and why a real `dva run x --explain --json | head -1` is not.
func brokenStdout(t *testing.T) func() {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close read end: %v", err)
	}
	os.Stdout = w
	return func() {
		os.Stdout = orig
		_ = w.Close()
	}
}

// TestExplainJSONBranchPropagatesWriteError covers TASK-121. output.PrintJSON can fail at the
// write (TASK-114), and Explain used to drop it because it had no return value — so
// `dva run x --explain --json` on a stdout that cannot be written printed nothing and exited 0.
// The text branch is out of scope here by design; see Explain's doc comment.
func TestExplainJSONBranchPropagatesWriteError(t *testing.T) {
	cmd := ResolvedCommand{Command: "console", Service: "web", Argv: []string{"x"}}

	restore := brokenStdout(t)
	defer restore()

	if err := Explain(&cmd, true); err == nil {
		t.Fatal("Explain(--json) = nil, want the write error from output.PrintJSON")
	}
}

// TestExplainTextBranchReturnsNil pins the asymmetry the doc comment promises: the text branch
// ignores its fmt errors and returns nil. Without this, a future pass that "fixes" the branch by
// returning the last fmt error would change a dozen call sites' contracts on a whim.
func TestExplainTextBranchReturnsNil(t *testing.T) {
	cmd := ResolvedCommand{Command: "console", Argv: []string{"x"}}

	// captureStdout so the text branch still writes somewhere real rather than to a closed pipe,
	// which would be testing the wrong thing — the text branch is not on this task's hook.
	out := captureStdout(t, func() {
		if err := Explain(&cmd, false); err != nil {
			t.Fatalf("Explain(--text) = %v, want nil by design", err)
		}
	})
	if !strings.Contains(out, "=== Command Execution Plan ===") {
		t.Fatalf("text plan did not print; captured %q", out)
	}
}
