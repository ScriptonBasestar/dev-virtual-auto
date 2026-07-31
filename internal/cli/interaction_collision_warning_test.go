// Package cli — regression tests for TASK-104's reporting half.
//
// Sorting the expansion made the dropped command stable; it did not make it visible. `dva validate`
// exited 0 and printed "dva.yml is valid" on a config with two declarations spelling one command
// name. These tests pin the message that replaced the silence — specifically that it names both
// declarations by the path the author wrote, not by the flattened name they never wrote.
package cli

import (
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func collidingConfig() *config.Config {
	return &config.Config{
		Interaction: map[string]*config.InteractionCommand{
			"a": {
				Subcommands: map[string]*config.InteractionCommand{
					"b c": {Command: "echo RAN-LITERAL-SUB"},
					"b": {
						Subcommands: map[string]*config.InteractionCommand{
							"c": {Command: "echo RAN-NESTED-SUB"},
						},
					},
				},
			},
		},
	}
}

func TestCollisionWarningNamesBothDeclarations(t *testing.T) {
	warnings := detectInteractionCollisionWarnings(collidingConfig())
	if len(warnings) != 1 {
		t.Fatalf("got %d warnings, want 1: %v", len(warnings), warnings)
	}
	got := warnings[0]

	// Both declarations, addressed the way they appear in dva.yml. Naming only the flattened
	// key would tell the author a command is unreachable without telling them which of the two
	// lines to edit.
	for _, want := range []string{
		`interaction.a.subcommands.b.subcommands.c`,
		`interaction.a.subcommands."b c"`,
		`"a b c"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("warning does not mention %s\n  got: %s", want, got)
		}
	}
}

// TestCollisionWarningQuotesOnlyTheSegmentWithASpace pins the detail that makes the message
// self-explanatory: the quoted segment is the one whose embedded space caused the collision, so a
// reader can see the cause without knowing how the tree flattens paths.
func TestCollisionWarningQuotesOnlyTheSegmentWithASpace(t *testing.T) {
	for _, tc := range []struct {
		path []string
		want string
	}{
		{[]string{"rails"}, "interaction.rails"},
		{[]string{"rails", "console"}, "interaction.rails.subcommands.console"},
		{[]string{"rails console"}, `interaction."rails console"`},
		{[]string{"a", "b c"}, `interaction.a.subcommands."b c"`},
		{[]string{"a", "b", "c"}, "interaction.a.subcommands.b.subcommands.c"},
	} {
		if got := describeInteractionPath(tc.path); got != tc.want {
			t.Errorf("describeInteractionPath(%v) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

// TestNoCollisionWarningWithoutACollision is the negative control: without it, every assertion
// above is also satisfied by a function that warns about every config it is handed.
func TestNoCollisionWarningWithoutACollision(t *testing.T) {
	c := &config.Config{
		Interaction: map[string]*config.InteractionCommand{
			"rails": {
				Command: "bundle exec rails",
				Subcommands: map[string]*config.InteractionCommand{
					"console": {Command: "rails console"},
					"db": {
						Subcommands: map[string]*config.InteractionCommand{
							"migrate": {Command: "db:migrate"},
						},
					},
				},
			},
			"build": {Command: "make build"},
		},
	}

	if warnings := detectInteractionCollisionWarnings(c); len(warnings) != 0 {
		t.Errorf("got %d warnings on a collision-free config: %v", len(warnings), warnings)
	}
	t.Logf("commands declared=5 warnings=0")
}
