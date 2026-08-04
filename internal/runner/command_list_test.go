package runner

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// TASK-178: `command:` as a YAML list ran only its first line on every runner but local, and
// --explain reported that one line as the whole plan. These tests assert the count, not just
// the shape: the defect was never a malformed argv, it was three argvs that were never built.

func TestClassifyFormPicksTheFormThatWins(t *testing.T) {
	tests := []struct {
		name string
		cmd  *ResolvedCommand
		want execForm
	}{
		{"steps beat everything", &ResolvedCommand{
			Steps:        []config.ProvisionItem{{Step: "one", Run: "echo one"}},
			ScriptFile:   "s.sh",
			Script:       "echo hi",
			CommandLines: []string{"a", "b"},
			Command:      "a",
		}, formSteps},
		{"script_file beats script", &ResolvedCommand{ScriptFile: "s.sh", Script: "echo hi"}, formScriptFile},
		{"script beats a list", &ResolvedCommand{Script: "echo hi", CommandLines: []string{"a", "b"}, Command: "a"}, formScript},
		// The case the whole task is about. Command is non-empty here because
		// polymorphicCommand puts line one in the scalar, which is exactly why testing
		// `Command != ""` first — as all three runners and Explain used to — picked the
		// wrong form.
		{"a list beats the scalar it populated", &ResolvedCommand{CommandLines: []string{"a", "b"}, Command: "a"}, formCommandList},
		{"a scalar command", &ResolvedCommand{Command: "echo hi"}, formCommand},
		// Terminal by design: an interaction declaring nothing is an empty command:, not a
		// sixth form. Asserted so that adding one is a deliberate act with a failing test.
		{"nothing declared is still command:", &ResolvedCommand{}, formCommand},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyForm(tt.cmd); got != tt.want {
				t.Errorf("classifyForm() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestEveryRunnerRefusesAnUnhandledForm is the regression test for the defect *class* rather
// than for one instance of it. TASK-094, TASK-175 and TASK-178 were all the same event — a form
// no branch named, falling through to whatever the last condition produced, silently and with
// exit 0. Go cannot make that a compile error, so this asserts the next best thing: a form no
// case covers stops with an error naming dva, and runs nothing.
func TestEveryRunnerRefusesAnUnhandledForm(t *testing.T) {
	const unknown = execForm(99)
	cmd := &ResolvedCommand{Command: "echo one", Service: "api", Pod: "web"}

	runners := map[string]func() error{
		"local":   func() error { return (&LocalRunner{Cmd: cmd}).runForm(nil, unknown) },
		"compose": func() error { return (&DockerComposeRunner{Cmd: cmd}).runForm(nil, unknown) },
		"kubectl": func() error { return (&KubectlRunner{Cmd: cmd}).runForm(nil, unknown) },
	}

	for name, run := range runners {
		t.Run(name, func(t *testing.T) {
			err := run()
			if err == nil {
				t.Fatal("no error for an unhandled form — the form fell through to some other branch, which is the whole bug")
			}
			if !strings.Contains(err.Error(), name) || !strings.Contains(err.Error(), "dva bug") {
				t.Errorf("error = %q, want it to name the runner and say the fault is dva's", err)
			}
		})
	}
}

func TestKubectlRunsEveryLineOfAList(t *testing.T) {
	r := &KubectlRunner{Cmd: &ResolvedCommand{
		Pod:          "web",
		CommandLines: []string{"echo one", "  ", "echo two"},
		Command:      "echo one",
		Shell:        true,
	}}

	got := r.eachArgs(r.Cmd.CommandLines)
	want := [][]string{
		{"exec", "web", "--", "sh", "-c", "echo one"},
		{"exec", "web", "--", "sh", "-c", "echo two"},
	}
	if len(got) != len(want) {
		t.Fatalf("built %d invocations, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if !equalArgs(got[i], want[i]) {
			t.Errorf("invocation %d = %v, want %v", i, got[i], want[i])
		}
	}
}

// The one-shot path asks for a terminal and the list path must not: kubectl fails outright when
// --tty is asked for and cannot be had, and a command that is one of several is never the one
// holding the terminal. Asserted separately from the argv above because it is a decision, not a
// detail — and because it is the only thing distinguishing these two argvs.
func TestKubectlListDoesNotAskForATerminal(t *testing.T) {
	cmd := &ResolvedCommand{Pod: "web", CommandLines: []string{"echo one", "echo two"}, Command: "echo one", Shell: true}
	r := &KubectlRunner{Cmd: cmd}

	for i, args := range r.eachArgs(cmd.CommandLines) {
		for _, a := range args {
			if a == "--tty" || a == "--stdin" {
				t.Errorf("invocation %d asks for a terminal: %v", i, args)
			}
		}
	}

	// Control: the single-command form still does, so this is a property of the list path and
	// not a flag that got dropped everywhere.
	one := &KubectlRunner{Cmd: &ResolvedCommand{Pod: "web", Command: "echo one"}}
	argv, err := one.execArgs()
	if err != nil {
		t.Fatalf("execArgs: %v", err)
	}
	if !strings.Contains(strings.Join(argv, " "), "--tty") {
		t.Errorf("single command lost --tty: %v", argv)
	}
}

func TestComposeRunsEveryLineOfAList(t *testing.T) {
	r := &DockerComposeRunner{Cmd: &ResolvedCommand{
		Service:      "api",
		CommandLines: []string{"echo one", "echo two"},
		Command:      "echo one",
		Shell:        true,
		Compose:      ComposeOpts{Method: "run"},
	}}

	got := r.eachArgs(nil, r.Cmd.CommandLines)
	if len(got) != 2 {
		t.Fatalf("built %d invocations, want 2: %v", len(got), got)
	}
	for i, want := range []string{"echo one", "echo two"} {
		joined := strings.Join(got[i], " ")
		if !strings.Contains(joined, want) {
			t.Errorf("invocation %d = %q, want it to run %q", i, joined, want)
		}
		// exec, never run, whatever Method says — `docker compose run` builds a fresh
		// container per invocation, so line two would execute somewhere line one never ran.
		if got[i][0] != "exec" {
			t.Errorf("invocation %d starts with %q, want exec", i, got[i][0])
		}
	}
}

// A list takes no arguments on any runner: LocalRunner hands CommandLines to ExecSequential and
// never reads commandArgs, so appending default_args on the other two would make one dva.yml
// mean two things.
func TestAListTakesNoDefaultArgsOnEitherRunner(t *testing.T) {
	lines := []string{"echo one", "echo two"}
	k := &KubectlRunner{Cmd: &ResolvedCommand{Pod: "web", CommandLines: lines, Command: "echo one", DefaultArgs: "--verbose", Shell: true}}
	c := &DockerComposeRunner{Cmd: &ResolvedCommand{Service: "api", CommandLines: lines, Command: "echo one", DefaultArgs: "--verbose", Shell: true}}

	for name, argvs := range map[string][][]string{"kubectl": k.eachArgs(lines), "compose": c.eachArgs(nil, lines)} {
		for i, args := range argvs {
			if strings.Contains(strings.Join(args, " "), "--verbose") {
				t.Errorf("%s invocation %d carries default_args: %v", name, i, args)
			}
		}
	}
}

func TestExplainNamesEveryLineOfAList(t *testing.T) {
	cmd := &ResolvedCommand{
		Name:         "seed",
		Pod:          "web",
		CommandLines: []string{"echo one", "echo two"},
		Command:      "echo one",
	}

	out := captureStdout(t, func() {
		if err := Explain(cmd, false); err != nil {
			t.Fatalf("Explain: %v", err)
		}
	})

	// The defect: `Command: echo one`, a complete-looking plan for one line of two.
	if strings.Contains(out, "Command: echo one") {
		t.Errorf("plan still reports one line as the command:\n%s", out)
	}
	if !strings.Contains(out, "Command: (2 commands") {
		t.Errorf("plan does not say how many commands there are:\n%s", out)
	}
	for _, line := range cmd.CommandLines {
		if !strings.Contains(out, "→ "+line) {
			t.Errorf("plan never names %q:\n%s", line, out)
		}
	}
}

func TestExplainJSONNamesEveryLineOfAList(t *testing.T) {
	cmd := &ResolvedCommand{Pod: "web", CommandLines: []string{"echo one", "echo two"}, Command: "echo one"}

	out := captureStdout(t, func() {
		if err := Explain(cmd, true); err != nil {
			t.Fatalf("Explain: %v", err)
		}
	})

	var plan map[string]any
	if err := json.Unmarshal([]byte(out), &plan); err != nil {
		t.Fatalf("unmarshal plan: %v\n%s", err, out)
	}
	if plan["command"] != "" {
		t.Errorf("command = %v, want empty — a list has no single command, and reporting line one is the defect", plan["command"])
	}
	lines, ok := plan["command_lines"].([]any)
	if !ok || len(lines) != 2 || lines[0] != "echo one" || lines[1] != "echo two" {
		t.Fatalf("command_lines = %v, want both lines in order", plan["command_lines"])
	}
}

// The key is absent for everything else, so `--explain --json` for every interaction that does
// not use a list is byte-identical to what it was. Corpus diffs depend on this.
func TestExplainJSONOmitsCommandLinesWithoutAList(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Explain(&ResolvedCommand{Command: "echo one"}, true); err != nil {
			t.Fatalf("Explain: %v", err)
		}
	})
	if strings.Contains(out, "command_lines") {
		t.Errorf("plan carries command_lines for a scalar command:\n%s", out)
	}
}
