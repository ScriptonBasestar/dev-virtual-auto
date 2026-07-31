// Package runner — regression tests for TASK-097.
//
// The tree keys commands by their segments joined with a space. That join is one-way as soon as a
// segment contains a space of its own, which schema.json permits, so ResolvedCommand carries Path
// alongside Name and consumers read it instead of re-splitting. These tests pin the property the
// consumers depend on: Path is the exact segment list, whatever the segments contain.
package runner

import (
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func spacedTree() *InteractionTree {
	return NewInteractionTree(map[string]*config.InteractionCommand{
		"my task": {
			Command: "echo top",
			Subcommands: map[string]*config.InteractionCommand{
				"sub": {Command: "echo sub"},
			},
		},
		"rails": {
			Command: "echo rails",
			Subcommands: map[string]*config.InteractionCommand{
				"db": {
					Command: "echo db",
					Subcommands: map[string]*config.InteractionCommand{
						"migrate":   {Command: "echo migrate"},
						"seed":      {Command: "echo seed"},
						"roll back": {Command: "echo rollback"},
					},
				},
			},
		},
	})
}

// TestListCarriesTheSegmentPath is the assertion consumers rely on. The `my task` rows are the
// ones a split of Name gets wrong: cutting at the first space reports the parent as `my`.
func TestListCarriesTheSegmentPath(t *testing.T) {
	commands := spacedTree().List()

	for _, tc := range []struct {
		key  string
		want []string
	}{
		{"my task", []string{"my task"}},
		{"my task sub", []string{"my task", "sub"}},
		{"rails", []string{"rails"}},
		{"rails db", []string{"rails", "db"}},
		{"rails db migrate", []string{"rails", "db", "migrate"}},
		{"rails db seed", []string{"rails", "db", "seed"}},
		{"rails db roll back", []string{"rails", "db", "roll back"}},
	} {
		cmd, ok := commands[tc.key]
		if !ok {
			t.Errorf("List() has no %q", tc.key)
			continue
		}
		if len(cmd.Path) != len(tc.want) {
			t.Errorf("%q: Path = %q, want %q", tc.key, cmd.Path, tc.want)
			continue
		}
		for i := range tc.want {
			if cmd.Path[i] != tc.want[i] {
				t.Errorf("%q: Path = %q, want %q", tc.key, cmd.Path, tc.want)
				break
			}
		}
		// Name stays the joined form — nothing about the existing key space changed, only what
		// is available beside it.
		if cmd.Name != tc.key {
			t.Errorf("%q: Name = %q, want the map key", tc.key, cmd.Name)
		}
	}
}

// TestSiblingPathsDoNotAlias catches the append trap directly. Three children hang off `rails db`;
// if the recursion handed them a slice sharing one backing array, each would overwrite the
// previous one's last segment and only the child visited last would be right. Map iteration order
// makes that a different survivor per run, so the failure would be intermittent.
func TestSiblingPathsDoNotAlias(t *testing.T) {
	commands := spacedTree().List()

	for _, key := range []string{"rails db migrate", "rails db seed", "rails db roll back"} {
		cmd := commands[key]
		if cmd == nil {
			t.Fatalf("List() has no %q", key)
		}
		if got := strings.Join(cmd.Path, " "); got != key {
			t.Errorf("%q: Path joins back to %q — a sibling overwrote a shared array", key, got)
		}
	}
}

// TestFindAlsoCarriesThePath keeps the two entry points consistent. Find expands through the same
// walk, so a command resolved for execution knows its own shape too; a future consumer reading
// Path off a Find result must not get nil.
func TestFindAlsoCarriesThePath(t *testing.T) {
	cmd := spacedTree().Find("rails", "db", "migrate")
	if cmd == nil {
		t.Fatal("Find did not resolve rails db migrate")
	}
	if got, want := strings.Join(cmd.Path, "|"), "rails|db|migrate"; got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}
