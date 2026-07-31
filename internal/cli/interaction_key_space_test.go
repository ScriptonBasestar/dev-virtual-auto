// Package cli — regression tests for TASK-097.
//
// `interactionUsage` decided whether a key was a subcommand by cutting it at its first space.
// The tree joins parent and child with a space, but a *segment* may contain one too —
// schema.json's interaction key pattern includes \s and no Go-side check rejects it — so the
// character could not tell the two shapes apart. A declared `"my task"` was read as parent `my`,
// and the emitted `dva my task` was the one form that provably did not run: measured against
// bin/dva at 8ae8da5, exit 1 with `unknown command "my" for "dva"`, while `dva 'my task'` ran it.
//
// The tests below do not pin the rendered string. They hand it back to the dispatcher the way a
// terminal would, so an example that is well-formed but unreachable fails here.
package cli

import (
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/runner"
)

// spacedInteractions covers every shape that shares the flat key space: a declared name holding a
// literal space, the same with a child, a subcommand whose own name holds one, an ordinary
// parent/child pair as the control, and a reserved name so the shadowing path stays exercised.
func spacedInteractions() *config.Config {
	return &config.Config{Interaction: map[string]*config.InteractionCommand{
		"my task": {
			Description: "declared key containing a space",
			Command:     "echo RAN-MY-TASK",
			Subcommands: map[string]*config.InteractionCommand{
				"sub": {Command: "echo RAN-MY-TASK-SUB"},
			},
		},
		"rails": {
			Description: "ordinary parent",
			Command:     "echo RAN-RAILS",
			Subcommands: map[string]*config.InteractionCommand{
				"console":    {Command: "echo RAN-CONSOLE"},
				"db migrate": {Command: "echo RAN-DB-MIGRATE"},
			},
		},
		"build": {Description: "reserved name", Command: "make build"},
	}}
}

// shellTokens undoes exactly what shellJoin does: bare words, single-quoted runs, and a
// backslash-escaped quote. Deliberately not a general shell parser — its only job is to let a
// test type the emitted example the way a user would, so quoting errors surface as
// unreachability.
//
// The escape sequence is spelled out in prose rather than shown: gofmt runs the doc comment
// formatter over this, and a pair of straight single quotes is legacy Go doc syntax for a
// closing typographic quote, so writing the literal form here silently rewrites it.
func shellTokens(t *testing.T, s string) []string {
	t.Helper()
	var (
		tokens  []string
		cur     strings.Builder
		quoted  bool
		started bool
		escape  bool
	)
	for _, r := range s {
		switch {
		case escape:
			cur.WriteRune(r)
			started, escape = true, false
		case r == '\\' && !quoted:
			escape = true
			started = true
		case r == '\'':
			quoted = !quoted
			started = true
		case r == ' ' && !quoted:
			if started {
				tokens = append(tokens, cur.String())
				cur.Reset()
				started = false
			}
		default:
			cur.WriteRune(r)
			started = true
		}
	}
	if quoted || escape {
		t.Fatalf("unbalanced quoting in %q — shellJoin emitted something this splitter cannot read back", s)
	}
	if started {
		tokens = append(tokens, cur.String())
	}
	return tokens
}

// dispatch replays how dva turns a typed line into a lookup. root.go prepends `run` only when
// args[0] is a declared interaction key — otherwise cobra reports an unknown command — and run.go
// then calls tree.Find(args[0], args[1:]...). Replaying it, rather than comparing strings, is what
// makes this test fail on the defect instead of on a rename.
func dispatch(t *testing.T, c *config.Config, usage string) *runner.ResolvedCommand {
	t.Helper()
	tokens := shellTokens(t, usage)
	if len(tokens) == 0 || tokens[0] != "dva" {
		t.Fatalf("usage %q does not start with the binary name", usage)
	}
	tokens = tokens[1:]
	if len(tokens) == 0 {
		return nil
	}
	if tokens[0] == "run" {
		tokens = tokens[1:]
	} else if c.Interaction[tokens[0]] == nil {
		// The bare form only routes when the first token is itself a declared key. This is the
		// branch the old rendering fell into: `dva my task` offered `my`, which is not one.
		return nil
	}
	if len(tokens) == 0 {
		return nil
	}
	return runner.NewInteractionTree(c.Interaction).Find(tokens[0], tokens[1:]...)
}

// TestEveryAdvertisedUsageReachesItsOwnEntry is the whole task in one assertion: for every key the
// manifest describes, the example it prints must resolve back to that same key. Reaching *a*
// command is not enough — landing on the parent instead of the child would otherwise read as a
// pass, and that is a shape this key space can produce.
func TestEveryAdvertisedUsageReachesItsOwnEntry(t *testing.T) {
	c := spacedInteractions()
	m := buildManifest(c)

	if len(m.DynamicCommands) != 6 {
		t.Fatalf("fixture resolved to %d dynamic commands, want 6: %v", len(m.DynamicCommands), m.DynamicCommands)
	}

	for key, entry := range m.DynamicCommands {
		resolved := dispatch(t, c, entry.UsageExample)
		if resolved == nil {
			t.Errorf("%q advertises %q, which reaches nothing", key, entry.UsageExample)
			continue
		}
		if resolved.Name != key {
			t.Errorf("%q advertises %q, which reaches %q instead", key, entry.UsageExample, resolved.Name)
		}
	}
}

// TestUsageQuotesOnlyWhatNeedsIt pins the rendering itself. The reachability test above passes for
// any over-quoted form too — `dva 'rails' 'console'` resolves fine — and an example a reader is
// meant to copy should not carry quotes it does not need.
func TestUsageQuotesOnlyWhatNeedsIt(t *testing.T) {
	m := buildManifest(spacedInteractions())

	for _, tc := range []struct{ key, want string }{
		{"my task", "dva 'my task'"},
		{"my task sub", "dva 'my task' sub"},
		{"rails", "dva rails"},
		{"rails console", "dva rails console"},
		{"rails db migrate", "dva rails 'db migrate'"},
		// Unchanged by this task: a reserved name is still reached through `dva run`.
		{"build", "dva run build"},
	} {
		entry, ok := m.DynamicCommands[tc.key]
		if !ok {
			t.Errorf("manifest has no entry for %q", tc.key)
			continue
		}
		if entry.UsageExample != tc.want {
			t.Errorf("%q: usage_example = %q, want %q", tc.key, entry.UsageExample, tc.want)
		}
	}
}

// TestShadowingSurvivesTheKeyChange guards the predecessor. TASK-076's marking runs off the path's
// first segment, which is now read from Path rather than cut out of the name, so it is worth
// asserting that the reserved-name answers did not move.
func TestShadowingSurvivesTheKeyChange(t *testing.T) {
	c := &config.Config{Interaction: map[string]*config.InteractionCommand{
		"build": {
			Command:     "make build",
			Subcommands: map[string]*config.InteractionCommand{"fast": {Command: "make fast"}},
		},
		"my-build": {Command: "make build"},
	}}
	m := buildManifest(c)

	for _, tc := range []struct{ key, usage, shadow string }{
		{"build", "dva run build", "build"},
		{"build fast", "dva run build fast", "build"},
		{"my-build", "dva my-build", ""},
	} {
		entry := m.DynamicCommands[tc.key]
		if entry.UsageExample != tc.usage || entry.ShadowedByBuiltin != tc.shadow {
			t.Errorf("%q: usage=%q shadowed_by=%q, want usage=%q shadowed_by=%q",
				tc.key, entry.UsageExample, entry.ShadowedByBuiltin, tc.usage, tc.shadow)
		}
	}
}

// TestLsAndManifestStillAgree is the reason interactionUsage is one function. Both surfaces now
// take a *ResolvedCommand rather than a key, so a call site left passing the wrong one would put
// the two documents back into disagreement.
func TestLsAndManifestStillAgree(t *testing.T) {
	c := spacedInteractions()
	commands, keys := resolve(t, c)
	entries := buildCommandEntries(c, commands, keys)
	m := buildManifest(c)

	for _, k := range keys {
		entry := entries[k].(map[string]any)
		lsShadow, _ := entry["shadowed_by_builtin"].(string)
		if lsShadow != m.DynamicCommands[k].ShadowedByBuiltin {
			t.Errorf("%q: ls says shadowed_by=%q, manifest says %q", k, lsShadow, m.DynamicCommands[k].ShadowedByBuiltin)
		}
	}
}
