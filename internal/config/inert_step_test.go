package config

import (
	"strings"
	"testing"
)

// TestIsInert pins the definition of "carries no payload". The risk in both directions is
// real: a definition too narrow leaves the original bug (an item that announces work and
// performs none), and one too wide libels a working config — every field below is a payload
// that some example file actually uses.
func TestIsInert(t *testing.T) {
	cases := []struct {
		name  string
		item  ProvisionItem
		inert bool
	}{
		{"a label and nothing else", ProvisionItem{Step: "make build"}, true},
		{"entirely empty", ProvisionItem{}, true},
		{"parallel is a modifier, not a payload", ProvisionItem{Step: "build", Parallel: true}, true},

		{"run as a string", ProvisionItem{Step: "build", Run: "make build"}, false},
		{"run as a list", ProvisionItem{Step: "build", Run: []any{"make", "make test"}}, false},
		{"the bare-string form", ProvisionItem{Raw: "make build"}, false},
		{"note", ProvisionItem{Step: "Manual", Note: "rotate the token"}, false},
		{"compose_up", ProvisionItem{Step: "db", ComposeUp: []string{"postgres"}}, false},
		{"compose_exec", ProvisionItem{Step: "wait", ComposeExec: "pg_isready"}, false},
		{"compose_run", ProvisionItem{Step: "migrate", ComposeRun: "alembic upgrade head"}, false},
		{"legacy echo", ProvisionItem{Echo: "starting"}, false},
		{"legacy cmd", ProvisionItem{Cmd: "make build"}, false},

		// An empty `run:` list is the one shape that looks like a payload and is not: it
		// yields zero commands, so the item behaves exactly like a bare label.
		{"an empty run list yields no commands", ProvisionItem{Step: "build", Run: []any{}}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.item.IsInert(); got != tc.inert {
				t.Errorf("IsInert() = %v, want %v", got, tc.inert)
			}
		})
	}
}

// TestWarnInertProvisionSteps covers the validate-time half. The nesting case is the one
// worth having: hooks recurse through Subcommands, and a check that stopped at depth 1 would
// report the shallow mistake and stay silent on the identical deep one.
func TestWarnInertProvisionSteps(t *testing.T) {
	c := &Config{
		Interaction: map[string]*InteractionCommand{
			"build": {
				Replace: []ProvisionItem{
					{Step: "generate", Run: "make generate"}, // fine
					{Step: "compile"},                        // inert
				},
			},
			"db": {
				Subcommands: map[string]*InteractionCommand{
					"migrate": {
						Before: []ProvisionItem{{Step: "backup"}}, // inert, two levels down
					},
				},
			},
		},
	}
	c.Provision.Profiles = map[string][]ProvisionItem{
		"default": {
			{Raw: "echo hello"},   // fine
			{Step: "seed the db"}, // inert
		},
	}

	warnings := c.warnInertProvisionSteps()

	want := []string{
		`interaction.build.replace[1] "compile"`,
		// `.subcommands.` spelled out, because `interaction.db.migrate` is not a path into
		// the document — `migrate` lives under `db.subcommands`, and a user who searches
		// their file for the shorter form finds nothing. TASK-128.
		`interaction.db.subcommands.migrate.before[0] "backup"`,
		`provision.default[1] "seed the db"`,
	}
	if len(warnings) != len(want) {
		t.Fatalf("got %d warnings, want %d:\n%s", len(warnings), len(want), strings.Join(warnings, "\n"))
	}
	// Compared in order, which also checks the sort: both sources are maps, and an unsorted
	// result would reorder between runs and make `validate` output undiffable.
	for i, w := range want {
		if !strings.HasPrefix(warnings[i], w) {
			t.Errorf("warning %d = %q, want prefix %q", i, warnings[i], w)
		}
		if !strings.Contains(warnings[i], InertStepMessage) {
			t.Errorf("warning %d does not carry the shared wording: %q", i, warnings[i])
		}
	}
}

// TestWarnInertProvisionStepsIsSilentOnACleanConfig is the negative control: without it, a
// check that returned every item would still pass the test above's count by accident only if
// the fixture were all-inert, and this fixture is not.
func TestWarnInertProvisionStepsIsSilentOnACleanConfig(t *testing.T) {
	c := &Config{
		Interaction: map[string]*InteractionCommand{
			"build": {
				Steps:   []ProvisionItem{{Step: "compile", Run: "make build"}},
				After:   []ProvisionItem{{Step: "notify", Note: "tell the team"}},
				Replace: []ProvisionItem{{Raw: "make all"}},
			},
		},
	}
	c.Provision.Profiles = map[string][]ProvisionItem{
		"default": {{Step: "db", ComposeUp: []string{"postgres"}}},
	}

	if w := c.warnInertProvisionSteps(); len(w) != 0 {
		t.Errorf("clean config produced %d warnings:\n%s", len(w), strings.Join(w, "\n"))
	}
}
