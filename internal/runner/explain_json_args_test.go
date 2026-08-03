package runner

import (
	"encoding/json"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// argsFixture is a parent carrying default_args over the five child shapes that differ in how
// they redeclare what runs. railsFixture is deliberately not reused: it mirrors
// examples/full-stack.yml for TASK-095 and has no script/steps children, and widening it would
// make that mirror stop mirroring.
func argsFixture() map[string]*config.InteractionCommand {
	return map[string]*config.InteractionCommand{
		"rails": {
			Command:     "bundle exec rails",
			DefaultArgs: "-e development",
			Subcommands: map[string]*config.InteractionCommand{
				"container":  {Description: "description: only — inherits the command, so it inherits its arguments"},
				"redeclared": {Command: "bundle exec rake"},
				"own_args":   {DefaultArgs: "-e test"},
				"scripted":   {Script: "echo scripted"},
				"filed":      {ScriptFile: "./hello.sh"},
				"stepped":    {Steps: []config.ProvisionItem{{Step: "one", Run: "echo stepped"}}},
			},
		},
	}
}

// TestExplainJSONArgumentsAreTheEffectiveArguments pins the `arguments` key of the --json plan.
//
// The key changed meaning in TASK-101 — from cmd.Argv, the literal invocation, to commandArgs,
// what the runners will actually pass — and nothing asserted the value afterwards, so the
// semantics of an agent-consumed field rested on a comment. TASK-149.
//
// It doubles as the regression for the script/script_file/steps rows: before TASK-149 those
// children inherited the parent's default_args and published them here, describing arguments no
// execution consumes.
func TestExplainJSONArgumentsAreTheEffectiveArguments(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		// want is the expected arguments array; nil means the key must be null, which is what
		// "this command passes nothing" has to look like rather than an empty array.
		want []string
	}{
		{
			name: "a pure container child inherits the parent's arguments",
			argv: []string{"container"},
			want: []string{"-e", "development"},
		},
		{
			name: "redeclaring the command starts the arguments clean",
			argv: []string{"redeclared"},
			want: nil,
		},
		{
			name: "the child's own default_args win outright",
			argv: []string{"own_args"},
			want: []string{"-e", "test"},
		},
		{
			name: "a script: child redeclares what runs, so it inherits no arguments",
			argv: []string{"scripted"},
			want: nil,
		},
		{
			name: "a script_file: child likewise",
			argv: []string{"filed"},
			want: nil,
		},
		{
			name: "a steps: child likewise",
			argv: []string{"stepped"},
			want: nil,
		},
		{
			// The parent itself, as the control: if this row ever went nil the rows above would
			// pass by inheriting nothing rather than by the rule under test.
			name: "the parent still carries its own arguments",
			argv: nil,
			want: []string{"-e", "development"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := NewInteractionTree(argsFixture()).Find("rails", tc.argv...)
			if cmd == nil {
				t.Fatalf("Find(%q, %v) resolved nothing", "rails", tc.argv)
			}

			out := captureStdout(t, func() {
				if err := Explain(cmd, true); err != nil {
					t.Fatalf("Explain: %v", err)
				}
			})

			var plan struct {
				Arguments []string `json:"arguments"`
			}
			if err := json.Unmarshal([]byte(out), &plan); err != nil {
				t.Fatalf("plan is not JSON (%v) — captured %q", err, out)
			}

			if len(plan.Arguments) != len(tc.want) {
				t.Fatalf("arguments = %v, want %v", plan.Arguments, tc.want)
			}
			for i := range tc.want {
				if plan.Arguments[i] != tc.want[i] {
					t.Fatalf("arguments = %v, want %v", plan.Arguments, tc.want)
				}
			}
		})
	}
}

// TestExplainJSONArgumentsKeyIsAlwaysPresent guards the shape rather than the value. A consumer
// that reads plan["arguments"] must not have to distinguish "absent" from "nothing to pass";
// omitempty on this key would make the two indistinguishable and is the change this catches.
func TestExplainJSONArgumentsKeyIsAlwaysPresent(t *testing.T) {
	cmd := NewInteractionTree(argsFixture()).Find("rails", "redeclared")
	out := captureStdout(t, func() {
		if err := Explain(cmd, true); err != nil {
			t.Fatalf("Explain: %v", err)
		}
	})

	var plan map[string]any
	if err := json.Unmarshal([]byte(out), &plan); err != nil {
		t.Fatalf("plan is not JSON (%v) — captured %q", err, out)
	}
	if _, ok := plan["arguments"]; !ok {
		t.Fatalf("plan has no arguments key at all — captured %q", out)
	}
}
