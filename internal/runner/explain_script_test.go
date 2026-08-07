package runner

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// TASK-174 / TASK-176: Explain must describe the form that runs, not an inherited command
// and not a blank Command: line.

func TestExplainScriptChildDoesNotNameParentCommand(t *testing.T) {
	tree := NewInteractionTree(map[string]*config.InteractionCommand{
		"rails": {
			Command:     "bundle exec rails",
			DefaultArgs: "-e development",
			Subcommands: map[string]*config.InteractionCommand{
				"scripted": {
					Script: "echo \"scripted child ran\"\n",
				},
			},
		},
	})
	cmd := tree.Find("rails", "scripted")
	if cmd == nil {
		t.Fatal("Find returned nil")
	}
	// Inheritance still leaves Command set on the struct (compose argv / other readers);
	// Explain must not report it as what runs.
	if cmd.Command == "" {
		t.Fatal("precondition: child still carries inherited Command on the struct")
	}

	out := captureStdout(t, func() {
		if err := Explain(cmd, false); err != nil {
			t.Fatal(err)
		}
	})
	if strings.Contains(out, "Command: bundle exec rails") {
		t.Errorf("plan names parent command that will not run:\n%s", out)
	}
	if !strings.Contains(out, "Command: (script-driven") {
		t.Errorf("plan missing script-driven Command line:\n%s", out)
	}
	if !strings.Contains(out, "scripted child ran") {
		t.Errorf("plan never names the script body:\n%s", out)
	}

	jsonOut := captureStdout(t, func() {
		if err := Explain(cmd, true); err != nil {
			t.Fatal(err)
		}
	})
	var plan map[string]any
	if err := json.Unmarshal([]byte(jsonOut), &plan); err != nil {
		t.Fatalf("json: %v\n%s", err, jsonOut)
	}
	if plan["command"] != "" {
		t.Errorf("json command = %v, want empty when script wins", plan["command"])
	}
	if plan["script"] == nil || !strings.Contains(plan["script"].(string), "scripted child ran") {
		t.Errorf("json script = %v, want the inline body", plan["script"])
	}
}

func TestExplainScriptFileNamesDeclaredPath(t *testing.T) {
	cmd := &ResolvedCommand{
		Name:       "seed",
		ScriptFile: "scripts/seed.sh",
		Pod:        "web",
	}
	out := captureStdout(t, func() {
		if err := Explain(cmd, false); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "Command: (script_file-driven") {
		t.Errorf("missing script_file-driven line:\n%s", out)
	}
	if !strings.Contains(out, "Script File: scripts/seed.sh") {
		t.Errorf("missing declared path:\n%s", out)
	}

	jsonOut := captureStdout(t, func() {
		if err := Explain(cmd, true); err != nil {
			t.Fatal(err)
		}
	})
	var plan map[string]any
	if err := json.Unmarshal([]byte(jsonOut), &plan); err != nil {
		t.Fatal(err)
	}
	if plan["script_file"] != "scripts/seed.sh" {
		t.Errorf("script_file = %v", plan["script_file"])
	}
	if plan["command"] != "" {
		t.Errorf("command should be empty for script_file form, got %v", plan["command"])
	}
}

func TestExplainChildStepsOnlyIsStepDriven(t *testing.T) {
	tree := NewInteractionTree(map[string]*config.InteractionCommand{
		"rails": {
			Command: "bundle exec rails",
			Subcommands: map[string]*config.InteractionCommand{
				"migrate": {
					Steps: []config.ProvisionItem{{Step: "up", Run: "rails db:migrate"}},
				},
			},
		},
	})
	cmd := tree.Find("rails", "migrate")
	out := captureStdout(t, func() {
		if err := Explain(cmd, false); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "Command: (step-driven") {
		t.Errorf("child steps should reach TASK-146 wording:\n%s", out)
	}
	if strings.Contains(out, "Command: bundle exec rails") {
		t.Errorf("must not name parent command:\n%s", out)
	}
}

func TestExplainDescriptionOnlyChildStillInheritsCommand(t *testing.T) {
	tree := NewInteractionTree(map[string]*config.InteractionCommand{
		"rails": {
			Command: "bundle exec rails",
			Subcommands: map[string]*config.InteractionCommand{
				"console": {Description: "open console"},
			},
		},
	})
	cmd := tree.Find("rails", "console")
	if cmd.Command != "bundle exec rails" {
		t.Fatalf("description-only child lost parent command: %q", cmd.Command)
	}
	out := captureStdout(t, func() {
		if err := Explain(cmd, false); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "Command: bundle exec rails") {
		t.Errorf("description-only child should still show inherited command:\n%s", out)
	}
}
