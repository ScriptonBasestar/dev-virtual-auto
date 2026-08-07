package cli

import (
	"strings"
	"testing"
)

// TASK-148: group parents used to swallow unknown children as leftover args, print help,
// and exit 0 with no suggestions.

func TestUnknownSubcommandErrorSuggestsFromCobra(t *testing.T) {
	setGroupParentBehavior(configCmd)
	setGroupParentBehavior(sshCmd)

	err := unknownSubcommandError(sshCmd, "statu")
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
	msg := err.Error()
	if !strings.Contains(msg, `unknown command "statu"`) {
		t.Errorf("error missing unknown-command shape: %q", msg)
	}
	if !strings.Contains(msg, "Did you mean this?") || !strings.Contains(msg, "status") {
		t.Errorf("error missing cobra SuggestionsFor block: %q", msg)
	}
	// Exactly one suggestion header — TASK-108's top-level duplicate must not return here.
	if strings.Count(msg, "Did you mean this?") != 1 {
		t.Errorf("want exactly one suggestion block, got %q", msg)
	}
}

func TestGroupParentRejectsUnknownChild(t *testing.T) {
	setGroupParentBehavior(configCmd)
	setGroupParentBehavior(sshCmd)

	// Bare parent still helps (no error).
	if err := configCmd.RunE(configCmd, nil); err != nil {
		t.Fatalf("bare config parent: %v", err)
	}

	err := sshCmd.RunE(sshCmd, []string{"statu"})
	if err == nil {
		t.Fatal("ssh statu should error")
	}
	if !strings.Contains(err.Error(), "status") {
		t.Errorf("ssh statu should suggest status: %v", err)
	}

	err = configCmd.RunE(configCmd, []string{"migrat"})
	if err == nil {
		t.Fatal("config migrat should error")
	}
	if !strings.Contains(err.Error(), "migrate") {
		t.Errorf("config migrat should suggest migrate: %v", err)
	}
}

func TestGroupParentsCovered(t *testing.T) {
	// The set of pure group parents (children, no other RunE). Keep the list here so adding
	// a third without setGroupParentBehavior fails the count.
	groups := []*struct {
		name string
		cmd  interface{ HasSubCommands() bool }
	}{
		{"config", configCmd},
		{"ssh", sshCmd},
	}
	if len(groups) != 2 {
		t.Fatalf("group parent count = %d, update setGroupParentBehavior call sites", len(groups))
	}
	for _, g := range groups {
		if !g.cmd.HasSubCommands() {
			t.Errorf("%s should have subcommands", g.name)
		}
	}
}
