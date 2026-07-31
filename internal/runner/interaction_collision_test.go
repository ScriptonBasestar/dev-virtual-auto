// Package runner — regression tests for TASK-104.
//
// Two different declarations can spell one command name, because the tree joins path segments with
// a space and schema.json admits a space inside a segment. The expansion writes into one flat map,
// so one of the two is dropped. Before this fix the drop was decided by Go's map seed: measured on
// an unchanged file, `dva run a b c` produced one command 18 times out of 20 and the other twice.
//
// These tests pin two separate properties. Determinism — the same tree always yields the same
// winner — and visibility — the loser is reported rather than vanishing. They fail independently,
// which is the point: making the drop reproducible does not make it acceptable.
package runner

import (
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// intraEntryCollision has both colliding declarations under one top-level entry, so they meet
// inside the map Find builds — which is why this shape breaks execution and not merely listing.
func intraEntryCollision() *InteractionTree {
	return NewInteractionTree(map[string]*config.InteractionCommand{
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
	})
}

// crossEntryCollision has them under different top-level entries, so they meet only in List.
func crossEntryCollision() *InteractionTree {
	return NewInteractionTree(map[string]*config.InteractionCommand{
		"rails console": {Command: "echo RAN-LITERAL-TOPLEVEL"},
		"rails": {
			Command: "echo RAN-RAILS",
			Subcommands: map[string]*config.InteractionCommand{
				"console": {Command: "echo RAN-EXPANDED-SUB"},
			},
		},
	})
}

// TestListIsDeterministicUnderCollision calls List repeatedly in one process. Go randomizes map
// iteration per range statement rather than per process, so an unsorted walk varies between calls
// here — no -count flag needed, though the package is also run with one.
func TestListIsDeterministicUnderCollision(t *testing.T) {
	for _, tc := range []struct {
		name string
		tree func() *InteractionTree
		key  string
	}{
		{"intra-entry", intraEntryCollision, "a b c"},
		{"cross-entry", crossEntryCollision, "rails console"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seen := map[string]int{}
			const runs = 200
			for i := range runs {
				cmd := tc.tree().List()[tc.key]
				if cmd == nil {
					t.Fatalf("%q resolved to nothing on run %d", tc.key, i)
				}
				seen[cmd.Command]++
			}
			if len(seen) != 1 {
				t.Errorf("%q resolved to %d different commands across %d runs: %v",
					tc.key, len(seen), runs, seen)
			}
		})
	}
}

// TestFindIsDeterministicUnderCollision is the execution half. Find expands the single entry it
// looked up, so only the intra-entry shape reaches it — and that is the shape the original filing
// missed when it recorded that execution was unaffected.
func TestFindIsDeterministicUnderCollision(t *testing.T) {
	seen := map[string]int{}
	const runs = 200
	for i := range runs {
		cmd := intraEntryCollision().Find("a", "b", "c")
		if cmd == nil {
			t.Fatalf("`a b c` resolved to nothing on run %d", i)
		}
		seen[cmd.Command]++
	}
	if len(seen) != 1 {
		t.Errorf("`dva run a b c` executed %d different commands across %d runs: %v",
			len(seen), runs, seen)
	}
}

func TestCollisionsAreReported(t *testing.T) {
	for _, tc := range []struct {
		name          string
		tree          func() *InteractionTree
		key           string
		winner, loser []string
	}{
		{"intra-entry", intraEntryCollision, "a b c", []string{"a", "b", "c"}, []string{"a", "b c"}},
		{"cross-entry", crossEntryCollision, "rails console", []string{"rails", "console"}, []string{"rails console"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, collisions := tc.tree().ListWithCollisions()
			if len(collisions) != 1 {
				t.Fatalf("got %d collisions, want 1: %+v", len(collisions), collisions)
			}
			c := collisions[0]
			if c.Key != tc.key {
				t.Errorf("key = %q, want %q", c.Key, tc.key)
			}
			// %q, not %v. A path is exactly the thing whose segment boundaries a space-joined
			// rendering loses: %v prints both ["a","b","c"] and ["a","b c"] as [a b c], so a
			// failure here read "winner = [a b c], want [a b c]" — measured, during the
			// non-vacuity probe. Same defect this task is about, in the report of the report.
			if !equalPath(c.Winner, tc.winner) {
				t.Errorf("winner = %q, want %q", c.Winner, tc.winner)
			}
			if !equalPath(c.Loser, tc.loser) {
				t.Errorf("loser = %q, want %q", c.Loser, tc.loser)
			}
		})
	}
}

// TestTheLoserIsTheOneMissingFromTheMap ties the two halves together: the reported Loser must be
// the path that is actually absent. A report naming the wrong one of the two would satisfy the
// count check above and send the author to edit the command that still works.
func TestTheLoserIsTheOneMissingFromTheMap(t *testing.T) {
	commands, collisions := intraEntryCollision().ListWithCollisions()
	if len(collisions) != 1 {
		t.Fatalf("got %d collisions, want 1", len(collisions))
	}

	survivor := commands[collisions[0].Key]
	if survivor == nil {
		t.Fatal("the colliding key resolves to nothing")
	}
	if !equalPath(survivor.Path, collisions[0].Winner) {
		t.Errorf("the map holds path %q but the collision names %q as the winner",
			survivor.Path, collisions[0].Winner)
	}
	if equalPath(survivor.Path, collisions[0].Loser) {
		t.Error("the collision named the surviving path as the loser")
	}
}

// TestSubcommandsOfALoserStillExpand guards the recursion. The losing declaration keeps its own
// children, which sit at longer paths and need not collide with anything; skipping them would turn
// one dropped command into a dropped subtree.
func TestSubcommandsOfALoserStillExpand(t *testing.T) {
	tree := NewInteractionTree(map[string]*config.InteractionCommand{
		"a": {
			Subcommands: map[string]*config.InteractionCommand{
				"b c": {
					Command: "echo RAN-LITERAL-SUB",
					Subcommands: map[string]*config.InteractionCommand{
						"d": {Command: "echo RAN-UNDER-LOSER"},
					},
				},
				"b": {
					Subcommands: map[string]*config.InteractionCommand{
						"c": {Command: "echo RAN-NESTED-SUB"},
					},
				},
			},
		},
	})

	commands := tree.List()
	cmd, ok := commands["a b c d"]
	if !ok {
		t.Fatalf("`a b c d` is absent; keys = %v", keysOf(commands))
	}
	if cmd.Command != "echo RAN-UNDER-LOSER" {
		t.Errorf("command = %q, want %q", cmd.Command, "echo RAN-UNDER-LOSER")
	}
}

// TestNoCollisionsWhenNoneExist is the negative control. Without it the tests above pass for an
// implementation that reports a collision for every command.
func TestNoCollisionsWhenNoneExist(t *testing.T) {
	commands, collisions := spacedTree().ListWithCollisions()
	if len(collisions) != 0 {
		t.Errorf("got %d collisions on a collision-free tree: %+v", len(collisions), collisions)
	}
	if len(commands) == 0 {
		t.Fatal("the fixture expanded to nothing, so the check above proves nothing")
	}
	t.Logf("commands=%d collisions=0", len(commands))
}

func equalPath(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func keysOf(m map[string]*ResolvedCommand) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
