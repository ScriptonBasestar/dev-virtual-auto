package config

import (
	"strings"
	"testing"
)

func TestValidateReservedCommands_NoConflict(t *testing.T) {
	interaction := map[string]*InteractionCommand{
		"shell": {Description: "Open shell"},
		"test":  {Description: "Run tests"},
		"lint":  {Description: "Run linter"},
	}

	conflicts := ValidateReservedCommands(interaction)
	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d: %v", len(conflicts), conflicts)
	}
}

func TestValidateReservedCommands_WithConflicts(t *testing.T) {
	interaction := map[string]*InteractionCommand{
		"shell": {Description: "Open shell"},
		"up":    {Description: "shadows reserved 'up'"},
		"build": {Description: "shadows reserved 'build'"},
	}

	conflicts := ValidateReservedCommands(interaction)
	if len(conflicts) != 2 {
		t.Fatalf("expected 2 conflicts, got %d", len(conflicts))
	}

	names := map[string]bool{}
	for _, c := range conflicts {
		names[c.Name] = true
	}
	if !names["up"] {
		t.Error("expected conflict for 'up'")
	}
	if !names["build"] {
		t.Error("expected conflict for 'build'")
	}
}

func TestValidateReservedCommands_EmptyInteraction(t *testing.T) {
	conflicts := ValidateReservedCommands(map[string]*InteractionCommand{})
	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts for empty interaction, got %d", len(conflicts))
	}
}

func TestValidateReservedCommands_AllReserved(t *testing.T) {
	reserved := ReservedCommands()
	interaction := make(map[string]*InteractionCommand, len(reserved))
	for name := range reserved {
		interaction[name] = &InteractionCommand{Description: "conflict"}
	}

	conflicts := ValidateReservedCommands(interaction)
	if len(conflicts) != len(reserved) {
		t.Errorf("expected %d conflicts, got %d", len(reserved), len(conflicts))
	}
}

func TestFormatConflictWarnings_Empty(t *testing.T) {
	msg := FormatConflictWarnings(nil)
	if msg != "" {
		t.Errorf("expected empty string, got %q", msg)
	}
}

func TestFormatConflictWarnings_Single(t *testing.T) {
	conflicts := []ReservedCommandConflict{
		{Name: "up", Source: "interaction"},
	}
	msg := FormatConflictWarnings(conflicts)
	if msg == "" {
		t.Error("expected non-empty warning message")
	}
	if !contains(msg, "'up'") {
		t.Errorf("expected message to contain 'up', got: %s", msg)
	}
}

func TestFormatConflictWarnings_Multiple(t *testing.T) {
	conflicts := []ReservedCommandConflict{
		{Name: "up", Source: "interaction"},
		{Name: "build", Source: "interaction"},
	}
	msg := FormatConflictWarnings(conflicts)
	if !contains(msg, "'up'") || !contains(msg, "'build'") {
		t.Errorf("expected message to contain both names, got: %s", msg)
	}
}

func TestReservedCommandsMapIsPopulated(t *testing.T) {
	reserved := ReservedCommands()
	if len(reserved) == 0 {
		t.Fatal("ReservedCommands should not be empty")
	}
	// Spot-check a few known commands
	expected := []string{"help", "up", "down", "run", "init", "validate"}
	for _, cmd := range expected {
		if !reserved[cmd] {
			t.Errorf("ReservedCommands missing expected command: %s", cmd)
		}
	}
}

func TestIsReservedCommand(t *testing.T) {
	if !IsReservedCommand("up") {
		t.Error("'up' should be reserved")
	}
	if IsReservedCommand("migrate") {
		t.Error("'migrate' should not be reserved without a built-in command")
	}
	if IsReservedCommand("cmd") {
		t.Error("'cmd' should not be reserved without a built-in command")
	}
	if IsReservedCommand("shell") {
		t.Error("'shell' should not be reserved")
	}
}

func TestValidateReservedCommands_AllowsNonBuiltInCommandNames(t *testing.T) {
	interaction := map[string]*InteractionCommand{
		"cmd":     {Command: "echo cmd"},
		"migrate": {Command: "make db-migrate"},
	}

	conflicts := ValidateReservedCommands(interaction)
	if len(conflicts) != 0 {
		t.Fatalf("expected former placeholder names to be allowed, got %v", conflicts)
	}
}

func TestValidateReservedCommands_NamespacePrefixConflict(t *testing.T) {
	interaction := map[string]*InteractionCommand{
		"app:build":   {Description: "build app"},
		"infra:setup": {Description: "setup infra"},
		"cargo:build": {Description: "no conflict"},
	}

	conflicts := ValidateReservedCommands(interaction)
	if len(conflicts) != 2 {
		t.Fatalf("expected 2 conflicts (app:build, infra:setup), got %d: %v", len(conflicts), conflicts)
	}

	names := map[string]bool{}
	for _, c := range conflicts {
		names[c.Name] = true
	}
	if !names["app:build"] {
		t.Error("expected conflict for 'app:build'")
	}
	if !names["infra:setup"] {
		t.Error("expected conflict for 'infra:setup'")
	}
	if names["cargo:build"] {
		t.Error("'cargo:build' should not conflict")
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
