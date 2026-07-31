package runner

import (
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
