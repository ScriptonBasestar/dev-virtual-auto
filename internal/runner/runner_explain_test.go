package runner

import (
	"os"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
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

// TestExplainListsStepsForStepDrivenInteraction covers TASK-146. A steps-only interaction has no
// single Command:, and Explain used to print a blank `Command:` line and name no step — so the
// one tool for checking what is about to happen hid the declared work. The plan must now state
// the interaction is step-driven and list each step with what it will run, mirroring runStepLoop's
// labels (a note renders as `  → label: note`, the same line the executing path prints).
//
// Fails on the reverted code: it printed `Command: \n` (blank) and no Steps section. Explain never
// reaches ExecReplace, so the in-process hazard TASK-144 guards does not apply here.
func TestExplainListsStepsForStepDrivenInteraction(t *testing.T) {
	cmd := ResolvedCommand{
		Steps: []config.ProvisionItem{
			{Step: "build assets", Run: "pnpm build"},
			{Step: "migrate db", Note: "safe to re-run", Run: "bin/migrate"},
		},
	}

	out := captureStdout(t, func() {
		if err := Explain(&cmd, false); err != nil {
			t.Fatalf("Explain: %v", err)
		}
	})

	// The plan must not print a blank Command: line for a steps-only interaction.
	if strings.Contains(out, "Command: \n") {
		t.Errorf("plan printed a blank Command: line instead of naming the steps:\n%s", out)
	}
	if !strings.Contains(out, "Command: (step-driven") {
		t.Errorf("plan does not state the interaction is step-driven:\n%s", out)
	}
	if !strings.Contains(out, "Steps:") {
		t.Errorf("plan has no Steps section:\n%s", out)
	}
	// Each step named, in order, with what it will run; the note renders as the exec path.
	for _, want := range []string{"build assets", "run: pnpm build", "migrate db: safe to re-run", "run: bin/migrate"} {
		if !strings.Contains(out, want) {
			t.Errorf("plan missing %q; got:\n%s", want, out)
		}
	}
}

// TestExplainStepsMirrorDispatch pins the cases TestExplainListsStepsForStepDrivenInteraction does
// not reach, all of which mirror runStepLoop's contract: a compose key short-circuits the step
// (so run:/echo:/cmd: on the same step must NOT appear), an inert step shows its marker, and an
// unnamed step falls back to "step N". Without the compose short-circuit assertion this task
// would ship the very misrendering the exec path avoids.
func TestExplainStepsMirrorDispatch(t *testing.T) {
	cmd := ResolvedCommand{
		Steps: []config.ProvisionItem{
			// compose_up AND run: — the exec path runs only compose_up; the plan must not list run:.
			{Step: "start db", ComposeUp: []string{"postgres"}, Run: "echo seed"},
			// a label-only step is inert.
			{Step: "manual deploy"},
			// no label → "step 3" fallback.
			{Run: "echo unnamed"},
		},
	}

	out := captureStdout(t, func() {
		if err := Explain(&cmd, false); err != nil {
			t.Fatalf("Explain: %v", err)
		}
	})

	// Compose short-circuit: compose_up shows, the run: on the SAME step does not. (A run: line
	// for the third step is expected; assert the short-circuited "echo seed" specifically is absent.)
	if !strings.Contains(out, "compose up: postgres") {
		t.Errorf("compose_up step did not show its payload:\n%s", out)
	}
	if strings.Contains(out, "echo seed") {
		t.Errorf("compose_up short-circuit violated: run: echo seed is shown but the exec path would skip it:\n%s", out)
	}
	// Inert step: named, with the shared inert marker.
	if !strings.Contains(out, "manual deploy") || !strings.Contains(out, config.InertStepMessage) {
		t.Errorf("inert step not rendered with its marker:\n%s", out)
	}
	// Label fallback: an unnamed step is "step 3", and its run command still shows.
	if !strings.Contains(out, "step 3") || !strings.Contains(out, "run: echo unnamed") {
		t.Errorf("unnamed step label fallback or its run command missing:\n%s", out)
	}
}
