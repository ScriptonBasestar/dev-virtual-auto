package cli

import (
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/runner"
)

// The subject of this file is one question asked of two surfaces: when an interaction is declared
// under a built-in's name, which invocation reaches it? `dva ls` and `dva manifest` describe the
// same key to the same reader, so they are tested together — a fix that marks one and not the
// other leaves the reader with two documents that disagree.

// shadowedInteractions declares a reserved name with a plain command (nothing extends the
// built-in, so the bare form is lost to it) alongside a free name that must stay untouched.
func shadowedInteractions() *config.Config {
	return &config.Config{Interaction: map[string]*config.InteractionCommand{
		"build":    {Description: "reserved name, plain command", Command: "make build"},
		"my-build": {Description: "free name, same command", Command: "make build"},
	}}
}

// hookInteractions declares the same reserved name in hook form. Measured against bin/dva,
// `dva build` dispatches to the replace: hook instead of the default compose build, so the bare
// form does reach what the author declared — this is the case that must NOT be marked, and the
// reason the mark keys off reachability rather than off IsReservedCommand alone.
//
// Step alone is a label, not a command: runner.executeSteps takes the commands from run: and
// skips an item with none. The fixture below therefore proves dispatch, not execution; execution
// was proved separately with a step:/run: pair.
func hookInteractions() *config.Config {
	return &config.Config{Interaction: map[string]*config.InteractionCommand{
		"build": {
			Description: "reserved name, replace hook",
			Replace:     []config.ProvisionItem{{Step: "echo REPLACED-BUILD"}},
		},
	}}
}

// resolve builds the same commands map and key order the ls and manifest code paths build, so a
// test cannot pass on a key shape production never produces.
func resolve(t *testing.T, c *config.Config) (map[string]*runner.ResolvedCommand, []string) {
	t.Helper()
	commands := runner.NewInteractionTree(c.Interaction).List()
	return commands, sortedKeys(commands)
}

func TestManifestMarksReservedInteraction(t *testing.T) {
	m := buildManifest(shadowedInteractions())

	entry, ok := m.DynamicCommands["build"]
	if !ok {
		t.Fatal("manifest dropped the 'build' interaction; a shadowed command must still be listed — the user declared it and dva did receive it")
	}
	if entry.ShadowedByBuiltin != "build" {
		t.Errorf("shadowed_by_builtin = %q, want %q; without the field a consumer has to parse the description to learn the short form runs something else", entry.ShadowedByBuiltin, "build")
	}
}

func TestManifestUsageExampleReachesTheInteraction(t *testing.T) {
	m := buildManifest(shadowedInteractions())

	// usage_example carries an implicit promise that running it invokes the entry it sits inside.
	// `dva build` is the one form that provably does not.
	if got, want := m.DynamicCommands["build"].UsageExample, "dva run build"; got != want {
		t.Errorf("usage_example = %q, want %q", got, want)
	}
	if _, ok := m.StaticCommands["build"]; !ok {
		t.Error("static_commands has no 'build' entry, so shadowed_by_builtin points at nothing; the field names the built-in that runs instead and must resolve inside the same document")
	}
}

func TestManifestLeavesNonReservedInteractionAlone(t *testing.T) {
	m := buildManifest(shadowedInteractions())

	entry := m.DynamicCommands["my-build"]
	if got, want := entry.UsageExample, "dva my-build"; got != want {
		t.Errorf("usage_example = %q, want %q; a free name keeps the short form", got, want)
	}
	if entry.ShadowedByBuiltin != "" {
		t.Errorf("shadowed_by_builtin = %q, want empty; marking a reachable command is a false claim in the opposite direction", entry.ShadowedByBuiltin)
	}
}

func TestManifestDoesNotMarkHookFormReservedName(t *testing.T) {
	m := buildManifest(hookInteractions())

	entry := m.DynamicCommands["build"]
	if entry.ShadowedByBuiltin != "" {
		t.Errorf("shadowed_by_builtin = %q, want empty: the built-in dispatches to the replace: hook, so 'dva build' does reach what the author declared", entry.ShadowedByBuiltin)
	}
	if got, want := entry.UsageExample, "dva build"; got != want {
		t.Errorf("usage_example = %q, want %q: sending a hook author to 'dva run build' routes around the built-in that runs their hook", got, want)
	}
}

// A subcommand of a reserved parent is the case the whole-key check missed: `IsReservedCommand`
// is false for "build fast", so it kept `usage_example: "dva build fast"` — measured, that runs
// the built-in compose build with an argument and never reaches the subcommand. Both parent
// shapes are covered because the hook exemption rescues the parent but not the child.
func TestSubcommandOfReservedParentIsMarked(t *testing.T) {
	parents := []struct {
		name   string
		parent *config.InteractionCommand
	}{
		{"plain parent", &config.InteractionCommand{Command: "echo PARENT"}},
		// `dva build fast` here dispatches to the parent's replace: hook and ignores "fast", so the subcommand is
		// still only reachable through `dva run` even though `dva build` reaches the parent.
		{"hook-form parent", &config.InteractionCommand{Replace: []config.ProvisionItem{{Step: "echo REPLACED"}}}},
	}

	for _, p := range parents {
		t.Run(p.name, func(t *testing.T) {
			p.parent.Description = "reserved parent"
			p.parent.Subcommands = map[string]*config.InteractionCommand{
				"fast": {Description: "subcommand", Command: "echo SUB"},
			}
			c := &config.Config{Interaction: map[string]*config.InteractionCommand{
				"build": p.parent,
				"my-build": {Description: "free parent", Command: "echo MY", Subcommands: map[string]*config.InteractionCommand{
					"fast": {Description: "subcommand", Command: "echo SUB-MY"},
				}},
			}}

			m := buildManifest(c)
			sub, ok := m.DynamicCommands["build fast"]
			if !ok {
				t.Fatalf("manifest has no 'build fast' entry: %v", m.DynamicCommands)
			}
			if got, want := sub.UsageExample, "dva run build fast"; got != want {
				t.Errorf("usage_example = %q, want %q", got, want)
			}
			if got, want := sub.ShadowedByBuiltin, "build"; got != want {
				t.Errorf("shadowed_by_builtin = %q, want %q — the field names the built-in that runs, which is the parent's name", got, want)
			}

			// A free parent's subcommand keeps the short form; measured, `dva my-build fast` runs it.
			free := m.DynamicCommands["my-build fast"]
			if got, want := free.UsageExample, "dva my-build fast"; got != want {
				t.Errorf("free parent's subcommand: usage_example = %q, want %q", got, want)
			}
			if free.ShadowedByBuiltin != "" {
				t.Errorf("free parent's subcommand marked as shadowed: %q", free.ShadowedByBuiltin)
			}
		})
	}
}

func TestLsJSONMarksReservedInteraction(t *testing.T) {
	c := shadowedInteractions()
	commands, keys := resolve(t, c)
	entries := buildCommandEntries(c, commands, keys)

	shadowed, ok := entries["build"].(map[string]any)
	if !ok {
		t.Fatalf("no 'build' entry in ls output: %v", entries)
	}
	if got := shadowed["shadowed_by_builtin"]; got != "build" {
		t.Errorf("shadowed_by_builtin = %v, want \"build\"", got)
	}

	// Absence is the signal for the reachable case: a consumer checks for the key, so emitting
	// it empty on every entry would make the check always true.
	free := entries["my-build"].(map[string]any)
	if got, ok := free["shadowed_by_builtin"]; ok {
		t.Errorf("free name carries shadowed_by_builtin = %v; the field's presence is what a consumer keys off", got)
	}
}

func TestLsTextMarksReservedInteraction(t *testing.T) {
	// printTable has three output shapes and the mark has to survive all of them — a user who
	// runs `dva ls -d`, or who declared no description, is reading the same wrong offer.
	shapes := []struct {
		name        string
		detailed    bool
		description string
	}{
		{"plain with description", false, "reserved name, plain command"},
		{"detailed", true, "reserved name, plain command"},
		{"plain without description", false, ""},
	}

	for _, shape := range shapes {
		t.Run(shape.name, func(t *testing.T) {
			c := &config.Config{Interaction: map[string]*config.InteractionCommand{
				"build":    {Description: shape.description, Command: "make build"},
				"my-build": {Description: shape.description, Command: "make build"},
			}}
			commands, keys := resolve(t, c)

			old := lsDetailed
			lsDetailed = shape.detailed
			defer func() { lsDetailed = old }()

			out := captureOutput(t, func() {
				if err := printTable(c, commands, keys); err != nil {
					t.Errorf("printTable: %v", err)
				}
			})

			lines := map[string]string{}
			for line := range strings.SplitSeq(out, "\n") {
				if strings.HasPrefix(line, "build") {
					lines["build"] = line
				} else if strings.HasPrefix(line, "my-build") {
					lines["my-build"] = line
				}
			}

			if !strings.Contains(lines["build"], "dva run build") {
				t.Errorf("shadowed row does not carry the form that reaches it; the listing reads as an offer of 'dva build', which runs the built-in.\ngot: %q", lines["build"])
			}
			if strings.Contains(lines["my-build"], "dva run") {
				t.Errorf("free name marked as shadowed: %q", lines["my-build"])
			}
		})
	}
}

func TestLsTextLeavesHookFormUnmarked(t *testing.T) {
	c := hookInteractions()
	commands, keys := resolve(t, c)

	out := captureOutput(t, func() {
		if err := printTable(c, commands, keys); err != nil {
			t.Errorf("printTable: %v", err)
		}
	})

	if strings.Contains(out, "takes this name") {
		t.Errorf("hook-form reserved name marked as shadowed; the built-in dispatches to its replace: hook.\ngot: %q", out)
	}
}
