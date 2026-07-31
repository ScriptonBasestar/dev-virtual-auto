package runner

import (
	"sort"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// railsFixture mirrors examples/full-stack.yml:156-181 — the project's own shipped example,
// which is what made TASK-095 worth a P2: `rails db migrate` and `rails db seed` are declared
// there, validated by internal/config/examples_test.go, and were unreachable.
func railsFixture() map[string]*config.InteractionCommand {
	return map[string]*config.InteractionCommand{
		"rails": {
			Description: "Run Rails commands",
			Service:     "web",
			Command:     "bundle exec rails",
			DefaultArgs: "server -p 3000 -b 0.0.0.0",
			Environment: map[string]string{"RAILS_LOG_TO_STDOUT": "true"},
			Subcommands: map[string]*config.InteractionCommand{
				"console": {
					Description: "Start Rails console",
					Command:     "console",
				},
				"db": {
					Description: "Database related commands",
					Subcommands: map[string]*config.InteractionCommand{
						"migrate": {
							Description: "Run database migrations",
							Command:     "db:migrate",
						},
						"seed": {
							Description: "Seed the database",
							Command:     "db:seed",
						},
					},
				},
			},
		},
	}
}

// TestInteractionTreeExpandsThirdLevel covers TASK-095. mergeInteraction never assigned
// Subcommands, so the value expandInto recursed on always had a nil map and the walk stopped at
// depth 2. Nothing below was added to the tree: no error, no warning, `dva validate` exit 0 —
// the declaration was accepted and then discarded.
func TestInteractionTreeExpandsThirdLevel(t *testing.T) {
	list := NewInteractionTree(railsFixture()).List()

	names := make([]string, 0, len(list))
	for name := range list {
		names = append(names, name)
	}
	sort.Strings(names)
	t.Logf("expanded %d keys: %v", len(names), names)

	// `rails db` carries only a description — it is a pure container node whose entire purpose
	// is the two children below it. Listing it while dropping them is the defect.
	for _, want := range []string{"rails", "rails console", "rails db", "rails db migrate", "rails db seed"} {
		if _, ok := list[want]; !ok {
			t.Errorf("missing %q — the third level did not expand", want)
		}
	}
	if len(list) != 5 {
		t.Errorf("key count = %d, want 5 (3 before the fix)", len(list))
	}
}

func TestInteractionThirdLevelIsRunnable(t *testing.T) {
	tree := NewInteractionTree(railsFixture())

	// Listing a command it cannot then resolve would be the same silent loss in a new place,
	// so reaching it through Find — the path `dva run` uses — is the assertion that matters.
	cases := map[string]string{
		"migrate": "db:migrate",
		"seed":    "db:seed",
	}
	for sub, want := range cases {
		cmd := tree.Find("rails", "db", sub)
		if cmd == nil {
			t.Fatalf("Find(rails, db, %s) = nil", sub)
		}
		if cmd.Command != want {
			t.Errorf("command = %q, want %q", cmd.Command, want)
		}
		// Two levels of inheritance, not one: service comes from the grandparent, which is the
		// part a merge that only ever ran once could not have produced.
		if cmd.Service != "web" {
			t.Errorf("service = %q, want web inherited from the grandparent", cmd.Service)
		}
		if cmd.Environment["RAILS_LOG_TO_STDOUT"] != "true" {
			t.Errorf("environment = %v, want the grandparent's entry carried through", cmd.Environment)
		}
		if len(cmd.Argv) != 0 {
			t.Errorf("argv = %v, want empty — all three words were consumed as the name", cmd.Argv)
		}
	}

	// Trailing words past a leaf must still become argv rather than failing the lookup.
	if cmd := tree.Find("rails", "db", "migrate", "VERSION=0"); cmd == nil {
		t.Error("Find with a trailing arg = nil")
	} else if len(cmd.Argv) != 1 || cmd.Argv[0] != "VERSION=0" {
		t.Errorf("argv = %v, want [VERSION=0]", cmd.Argv)
	}
}

// TestInteractionTreeExpandsArbitraryDepth answers the open question in TASK-095: whether depth
// is bounded deliberately. It is not — so the fix is unbounded expansion rather than a
// validate-time rejection naming the limit.
func TestInteractionTreeExpandsArbitraryDepth(t *testing.T) {
	entries := map[string]*config.InteractionCommand{
		"l1": {Command: "echo L1", Subcommands: map[string]*config.InteractionCommand{
			"l2": {Command: "echo L2", Subcommands: map[string]*config.InteractionCommand{
				"l3": {Command: "echo L3", Subcommands: map[string]*config.InteractionCommand{
					"l4": {Command: "echo L4"},
				}},
			}},
		}},
	}
	tree := NewInteractionTree(entries)

	if got := len(tree.List()); got != 4 {
		t.Errorf("key count = %d, want 4", got)
	}
	cmd := tree.Find("l1", "l2", "l3", "l4")
	if cmd == nil {
		t.Fatal("Find at depth 4 = nil")
	}
	if cmd.Command != "echo L4" {
		t.Errorf("command = %q, want echo L4", cmd.Command)
	}
}

// TestInteractionMergeTakesTheChildsSubcommands pins the hazard the fix had to avoid. expandInto
// recurses on the merged value, so assigning the *parent's* map would re-expand the parent's own
// children under every child name and never terminate — `make test` would hang instead of fail.
// The nil this replaced was load-bearing for exactly that reason; the fix is only correct because
// it takes the child's map specifically.
func TestInteractionMergeTakesTheChildsSubcommands(t *testing.T) {
	parent := railsFixture()["rails"]
	merged := mergeInteraction(parent, parent.Subcommands["db"])

	if _, leaked := merged.Subcommands["db"]; leaked {
		t.Fatal("the merged value carries the parent's own subcommand map — expandInto would not terminate")
	}
	if _, leaked := merged.Subcommands["console"]; leaked {
		t.Fatal("the merged value carries a sibling from the parent's map — expandInto would not terminate")
	}
	for _, want := range []string{"migrate", "seed"} {
		if _, ok := merged.Subcommands[want]; !ok {
			t.Errorf("missing %q — the child's own map was not carried", want)
		}
	}
	if got := len(merged.Subcommands); got != 2 {
		t.Errorf("subcommand count = %d, want 2", got)
	}

	// A leaf merges to nil, which is what ends the recursion on that branch.
	if leaf := mergeInteraction(parent, parent.Subcommands["console"]); leaf.Subcommands != nil {
		t.Errorf("a leaf must merge to a nil map, got %v", leaf.Subcommands)
	}
}

// TestInteractionDefaultArgsInheritIntoSubcommands characterizes behaviour this fix did NOT
// change and does not endorse: DefaultArgs inherits even when the child replaces Command
// outright, so `dva run rails console` executes `console server -p 3000 -b 0.0.0.0`. Measured at
// depth 2 against bin/dva before TASK-095, so it is pre-existing — the fix merely lets depth 3
// reach the same code. Tracked as TASK-101; when that lands, this assertion is meant to fail and
// be rewritten deliberately rather than to be discovered by surprise.
func TestInteractionDefaultArgsInheritIntoSubcommands(t *testing.T) {
	tree := NewInteractionTree(railsFixture())
	for _, name := range [][]string{{"rails", "console"}, {"rails", "db", "migrate"}} {
		cmd := tree.Find(name[0], name[1:]...)
		if cmd == nil {
			t.Fatalf("Find%v = nil", name)
		}
		if cmd.DefaultArgs != "server -p 3000 -b 0.0.0.0" {
			t.Errorf("%v DefaultArgs = %q; if TASK-101 changed this, update the test and its comment", name, cmd.DefaultArgs)
		}
	}
}
