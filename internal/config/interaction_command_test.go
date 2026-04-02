package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestInteractionCommandUnmarshal_StringCommand(t *testing.T) {
	input := `
command: "echo hello"
description: "Simple command"
`
	var cmd InteractionCommand
	if err := yaml.Unmarshal([]byte(input), &cmd); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if cmd.Command != "echo hello" {
		t.Errorf("Command = %q, want %q", cmd.Command, "echo hello")
	}
	if cmd.CommandLines != nil {
		t.Errorf("CommandLines should be nil for scalar command, got %v", cmd.CommandLines)
	}
	if cmd.Description != "Simple command" {
		t.Errorf("Description = %q, want %q", cmd.Description, "Simple command")
	}
}

func TestInteractionCommandUnmarshal_ListCommand(t *testing.T) {
	input := `
command:
  - "git pull"
  - "docker compose build"
  - "docker compose up -d"
description: "Deploy steps"
`
	var cmd InteractionCommand
	if err := yaml.Unmarshal([]byte(input), &cmd); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(cmd.CommandLines) != 3 {
		t.Fatalf("CommandLines len = %d, want 3", len(cmd.CommandLines))
	}
	if cmd.CommandLines[0] != "git pull" {
		t.Errorf("CommandLines[0] = %q, want %q", cmd.CommandLines[0], "git pull")
	}
	if cmd.CommandLines[2] != "docker compose up -d" {
		t.Errorf("CommandLines[2] = %q, want %q", cmd.CommandLines[2], "docker compose up -d")
	}
	// Command should be the first entry for backward compat
	if cmd.Command != "git pull" {
		t.Errorf("Command = %q, want %q (first entry)", cmd.Command, "git pull")
	}
	if cmd.Description != "Deploy steps" {
		t.Errorf("Description = %q, want %q", cmd.Description, "Deploy steps")
	}
}

func TestInteractionCommandUnmarshal_InlineScript(t *testing.T) {
	input := `
description: "Inline script"
script: |
  #!/bin/bash
  set -e
  echo "building..."
  make build
`
	var cmd InteractionCommand
	if err := yaml.Unmarshal([]byte(input), &cmd); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if cmd.Script == "" {
		t.Error("Script should not be empty")
	}
	if cmd.Command != "" {
		t.Errorf("Command should be empty when script is set, got %q", cmd.Command)
	}
	wantPrefix := "#!/bin/bash"
	if len(cmd.Script) < len(wantPrefix) || cmd.Script[:len(wantPrefix)] != wantPrefix {
		t.Errorf("Script does not start with %q, got: %q", wantPrefix, cmd.Script[:20])
	}
}

func TestInteractionCommandUnmarshal_ScriptFile(t *testing.T) {
	input := `
description: "Script file"
script_file: ".sb/dva/scripts/deploy.sh"
runner: local
`
	var cmd InteractionCommand
	if err := yaml.Unmarshal([]byte(input), &cmd); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if cmd.ScriptFile != ".sb/dva/scripts/deploy.sh" {
		t.Errorf("ScriptFile = %q, want %q", cmd.ScriptFile, ".sb/dva/scripts/deploy.sh")
	}
	if cmd.Runner != "local" {
		t.Errorf("Runner = %q, want %q", cmd.Runner, "local")
	}
}

func TestInteractionCommandUnmarshal_Steps(t *testing.T) {
	input := `
description: "Named steps"
steps:
  - step: "Install deps"
    run: "npm install"
  - step: "Build"
    run: "npm run build"
  - step: "Ready"
    note: "Done!"
runner: local
`
	var cmd InteractionCommand
	if err := yaml.Unmarshal([]byte(input), &cmd); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(cmd.Steps) != 3 {
		t.Fatalf("Steps len = %d, want 3", len(cmd.Steps))
	}
	if cmd.Steps[0].Step != "Install deps" {
		t.Errorf("Steps[0].Step = %q, want %q", cmd.Steps[0].Step, "Install deps")
	}
	if cmd.Steps[2].Note != "Done!" {
		t.Errorf("Steps[2].Note = %q, want %q", cmd.Steps[2].Note, "Done!")
	}
}

func TestInteractionCommandUnmarshal_HasHelpers(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		wantFn   func(*InteractionCommand) bool
		wantBool bool
	}{
		{
			name:     "HasMultiCommand true",
			yaml:     "command:\n  - echo a\n  - echo b",
			wantFn:   (*InteractionCommand).HasMultiCommand,
			wantBool: true,
		},
		{
			name:     "HasMultiCommand false",
			yaml:     "command: echo a",
			wantFn:   (*InteractionCommand).HasMultiCommand,
			wantBool: false,
		},
		{
			name:     "HasScript true",
			yaml:     "script: |\n  echo hi",
			wantFn:   (*InteractionCommand).HasScript,
			wantBool: true,
		},
		{
			name:     "HasScriptFile true",
			yaml:     "script_file: deploy.sh",
			wantFn:   (*InteractionCommand).HasScriptFile,
			wantBool: true,
		},
		{
			name:     "HasSteps true",
			yaml:     "steps:\n  - step: a\n    run: echo",
			wantFn:   (*InteractionCommand).HasSteps,
			wantBool: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cmd InteractionCommand
			if err := yaml.Unmarshal([]byte(tt.yaml), &cmd); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got := tt.wantFn(&cmd); got != tt.wantBool {
				t.Errorf("%s() = %v, want %v", tt.name, got, tt.wantBool)
			}
		})
	}
}

func TestInteractionCommandUnmarshal_EffectiveCommand(t *testing.T) {
	t.Run("single command", func(t *testing.T) {
		var cmd InteractionCommand
		_ = yaml.Unmarshal([]byte("command: echo hello"), &cmd)
		if cmd.EffectiveCommand() != "echo hello" {
			t.Errorf("EffectiveCommand() = %q, want %q", cmd.EffectiveCommand(), "echo hello")
		}
	})
	t.Run("multi command joined", func(t *testing.T) {
		var cmd InteractionCommand
		_ = yaml.Unmarshal([]byte("command:\n  - git pull\n  - make build"), &cmd)
		want := "git pull && make build"
		if cmd.EffectiveCommand() != want {
			t.Errorf("EffectiveCommand() = %q, want %q", cmd.EffectiveCommand(), want)
		}
	})
}
