package runner

import (
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// TASK-173: empty resolved nodes used to become `sh -c ""` and exit 0.

func TestHasExecutionTarget(t *testing.T) {
	tests := []struct {
		name string
		cmd  *ResolvedCommand
		want bool
	}{
		{"nil", nil, false},
		{"empty", &ResolvedCommand{Name: "lone"}, false},
		{"description only", &ResolvedCommand{Name: "lone", Description: "x"}, false},
		{"command", &ResolvedCommand{Command: "echo hi"}, true},
		{"command list", &ResolvedCommand{CommandLines: []string{"a", "b"}, Command: "a"}, true},
		{"script", &ResolvedCommand{Script: "echo hi"}, true},
		{"script_file", &ResolvedCommand{ScriptFile: "s.sh"}, true},
		{"steps", &ResolvedCommand{Steps: []config.ProvisionItem{{Step: "one", Run: "echo"}}}, true},
		{"service", &ResolvedCommand{Service: "api"}, true},
		{"pod", &ResolvedCommand{Pod: "web"}, true},
		{"runner name", &ResolvedCommand{RunnerName: "local"}, true},
		{"default_args alone", &ResolvedCommand{DefaultArgs: "echo reached"}, true},
		{"argv alone", &ResolvedCommand{Argv: []string{"echo", "hi"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cmd.HasExecutionTarget(); got != tt.want {
				t.Errorf("HasExecutionTarget() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExplainNothingToRunSucceeds(t *testing.T) {
	cmd := &ResolvedCommand{Name: "lone", Description: "x"}
	out := captureStdout(t, func() {
		if err := Explain(cmd, false); err != nil {
			t.Fatalf("Explain: %v", err)
		}
	})
	if !strings.Contains(out, "nothing to run") {
		t.Errorf("explain output missing nothing-to-run diagnosis:\n%s", out)
	}
	if err := Explain(cmd, true); err != nil {
		t.Fatalf("Explain json: %v", err)
	}
}

func TestErrNothingToRunNamesTheNode(t *testing.T) {
	err := ErrNothingToRun(&ResolvedCommand{Name: "lone"})
	if err == nil || !strings.Contains(err.Error(), "lone") || !strings.Contains(err.Error(), "nothing to run") {
		t.Fatalf("ErrNothingToRun = %v", err)
	}
}

// Inheritance: a child with no command of its own still runs when the parent supplies one.
func TestInheritedCommandHasExecutionTarget(t *testing.T) {
	tree := NewInteractionTree(map[string]*config.InteractionCommand{
		"parent": {
			Command: "echo from-parent",
			Subcommands: map[string]*config.InteractionCommand{
				"child": {Description: "inherits"},
			},
		},
	})
	resolved := tree.Find("parent", "child")
	if resolved == nil {
		t.Fatal("Find returned nil")
	}
	if !resolved.HasExecutionTarget() {
		t.Fatalf("inherited command should count as a target; got %+v", resolved)
	}
	if resolved.Command != "echo from-parent" {
		t.Errorf("Command = %q, want inherited parent command", resolved.Command)
	}
}
