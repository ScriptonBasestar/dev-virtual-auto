package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestHasHooks_Empty(t *testing.T) {
	ic := &InteractionCommand{Description: "test"}
	if ic.HasHooks() {
		t.Error("expected HasHooks() to be false for empty command")
	}
}

func TestHasHooks_Before(t *testing.T) {
	ic := &InteractionCommand{
		Before: []ProvisionItem{{Step: "pre", Raw: "echo pre"}},
	}
	if !ic.HasHooks() {
		t.Error("expected HasHooks() to be true with before steps")
	}
}

func TestHasHooks_Replace(t *testing.T) {
	ic := &InteractionCommand{
		Replace: []ProvisionItem{{Step: "custom", Raw: "make build"}},
	}
	if !ic.HasHooks() {
		t.Error("expected HasHooks() to be true with replace steps")
	}
}

func TestHasHooks_After(t *testing.T) {
	ic := &InteractionCommand{
		After: []ProvisionItem{{Step: "post", Raw: "echo done"}},
	}
	if !ic.HasHooks() {
		t.Error("expected HasHooks() to be true with after steps")
	}
}

func TestHasHooks_AllPhases(t *testing.T) {
	ic := &InteractionCommand{
		Before:  []ProvisionItem{{Step: "pre", Raw: "echo pre"}},
		Replace: []ProvisionItem{{Step: "main", Raw: "make build"}},
		After:   []ProvisionItem{{Step: "post", Raw: "echo done"}},
	}
	if !ic.HasHooks() {
		t.Error("expected HasHooks() to be true with all phases")
	}
}

func TestIsHookableCommand(t *testing.T) {
	hookable := []string{"up", "down", "stop", "restart", "build", "clean", "logs"}
	for _, cmd := range hookable {
		if !IsHookableCommand(cmd) {
			t.Errorf("expected '%s' to be hookable", cmd)
		}
	}

	notHookable := []string{"run", "init", "validate", "show", "status", "config", "shell"}
	for _, cmd := range notHookable {
		if IsHookableCommand(cmd) {
			t.Errorf("expected '%s' to NOT be hookable", cmd)
		}
	}
}

func TestHookableCommandsReturnsCopy(t *testing.T) {
	h := HookableCommands()
	h["fake"] = true
	if IsHookableCommand("fake") {
		t.Error("HookableCommands should return a copy, not the original map")
	}
}

func TestValidateReservedCommands_HookableWithHooks_NoConflict(t *testing.T) {
	interaction := map[string]*InteractionCommand{
		"build": {
			Description: "Custom build with hooks",
			Before:      []ProvisionItem{{Step: "proto", Raw: "make proto"}},
		},
		"clean": {
			Description: "Custom clean",
			Replace:     []ProvisionItem{{Step: "deep clean", Raw: "make clean"}},
		},
	}

	conflicts := ValidateReservedCommands(interaction)
	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts for hookable commands with hooks, got %d: %v", len(conflicts), conflicts)
	}
}

func TestValidateReservedCommands_HookableWithoutHooks_Conflict(t *testing.T) {
	interaction := map[string]*InteractionCommand{
		"build": {
			Description: "shadows build",
			Command:     "cargo build",
		},
	}

	conflicts := ValidateReservedCommands(interaction)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	if conflicts[0].Name != "build" {
		t.Errorf("expected conflict for 'build', got '%s'", conflicts[0].Name)
	}
}

func TestValidateReservedCommands_NonHookableReserved_AlwaysConflict(t *testing.T) {
	interaction := map[string]*InteractionCommand{
		"init": {
			Description: "custom init",
			Before:      []ProvisionItem{{Step: "pre", Raw: "echo pre"}},
		},
	}

	conflicts := ValidateReservedCommands(interaction)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict for non-hookable reserved 'init', got %d", len(conflicts))
	}
}

func TestValidateReservedCommands_MixedHooksAndConflicts(t *testing.T) {
	interaction := map[string]*InteractionCommand{
		"up": {
			Description: "hook on up",
			After:       []ProvisionItem{{Step: "seed", Raw: "make seed"}},
		},
		"run": {
			Description: "shadows run",
			Command:     "my-runner",
		},
		"build": {
			Description: "hook on build",
			Before:      []ProvisionItem{{Step: "proto", Raw: "make proto"}},
		},
	}

	conflicts := ValidateReservedCommands(interaction)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict (only 'run'), got %d", len(conflicts))
	}
	if conflicts[0].Name != "run" {
		t.Errorf("expected conflict for 'run', got '%s'", conflicts[0].Name)
	}
}

func TestYAMLParsing_HookFields(t *testing.T) {
	yamlStr := `
build:
  description: "Extended build"
  before:
    - step: "Generate proto"
      run: "make proto"
    - step: "Fetch deps"
      run:
        - "cargo fetch"
        - "npm install"
  after:
    - step: "Tag image"
      run: "docker tag app:latest app:dev"
clean:
  description: "Custom clean"
  replace:
    - step: "Deep clean"
      run: "make deep-clean"
    - step: "Remove caches"
      run: "rm -rf .cache/"
`

	var interaction map[string]*InteractionCommand
	if err := yaml.Unmarshal([]byte(yamlStr), &interaction); err != nil {
		t.Fatalf("YAML parse error: %v", err)
	}

	// build: before + after
	build := interaction["build"]
	if build == nil {
		t.Fatal("expected 'build' command")
	}
	if !build.HasHooks() {
		t.Error("build should have hooks")
	}
	if len(build.Before) != 2 {
		t.Errorf("expected 2 before steps, got %d", len(build.Before))
	}
	if build.Before[0].Step != "Generate proto" {
		t.Errorf("expected step name 'Generate proto', got '%s'", build.Before[0].Step)
	}
	// Multi-command step
	cmds := build.Before[1].RunCommands()
	if len(cmds) != 2 {
		t.Errorf("expected 2 run commands, got %d", len(cmds))
	}
	if len(build.After) != 1 {
		t.Errorf("expected 1 after step, got %d", len(build.After))
	}
	if len(build.Replace) != 0 {
		t.Errorf("expected 0 replace steps, got %d", len(build.Replace))
	}

	// clean: replace
	clean := interaction["clean"]
	if clean == nil {
		t.Fatal("expected 'clean' command")
	}
	if !clean.HasHooks() {
		t.Error("clean should have hooks")
	}
	if len(clean.Replace) != 2 {
		t.Errorf("expected 2 replace steps, got %d", len(clean.Replace))
	}
	if len(clean.Before) != 0 {
		t.Errorf("expected 0 before steps, got %d", len(clean.Before))
	}
}

func TestYAMLParsing_HookWithComposeAware(t *testing.T) {
	yamlStr := `
up:
  description: "Up with pre-start"
  before:
    - step: "Start postgres first"
      compose_up: ["postgres"]
  after:
    - step: "Check health"
      compose_exec: "postgres pg_isready -U app"
`

	var interaction map[string]*InteractionCommand
	if err := yaml.Unmarshal([]byte(yamlStr), &interaction); err != nil {
		t.Fatalf("YAML parse error: %v", err)
	}

	up := interaction["up"]
	if up == nil {
		t.Fatal("expected 'up' command")
	}
	if len(up.Before) != 1 {
		t.Fatalf("expected 1 before step, got %d", len(up.Before))
	}
	if len(up.Before[0].ComposeUp) != 1 || up.Before[0].ComposeUp[0] != "postgres" {
		t.Errorf("expected compose_up: [postgres], got %v", up.Before[0].ComposeUp)
	}
	if len(up.After) != 1 {
		t.Fatalf("expected 1 after step, got %d", len(up.After))
	}
	if up.After[0].ComposeExec != "postgres pg_isready -U app" {
		t.Errorf("expected compose_exec, got '%s'", up.After[0].ComposeExec)
	}
}

func TestYAMLParsing_AllThreePhases(t *testing.T) {
	yamlStr := `
build:
  before:
    - step: "Pre"
      run: "echo pre"
  replace:
    - step: "Main"
      run: "cargo build"
  after:
    - step: "Post"
      run: "echo post"
`

	var interaction map[string]*InteractionCommand
	if err := yaml.Unmarshal([]byte(yamlStr), &interaction); err != nil {
		t.Fatalf("YAML parse error: %v", err)
	}

	build := interaction["build"]
	if len(build.Before) != 1 || len(build.Replace) != 1 || len(build.After) != 1 {
		t.Errorf("expected 1 step in each phase, got before=%d replace=%d after=%d",
			len(build.Before), len(build.Replace), len(build.After))
	}
}
