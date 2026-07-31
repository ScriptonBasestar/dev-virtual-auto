// Package cli — regression coverage for TASK-096.
//
// `dva manifest` documents itself as the agent-facing entry point, and its static_commands table
// was a hand-copied 13 of the 27 real commands. The gap is not visible from either side: nothing
// in cli/ read the table back, and nothing in config/ knew the table existed. These tests are the
// missing link, and they are anchored on rootCmd — the actual registry — rather than on
// reservedCommands, so a command added to root fails here even if reserved.go is also forgotten.
package cli

import (
	"sort"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/spf13/cobra"
)

// rootCommandNames returns every command name cobra will accept at the top level.
//
// Commands() alone returns 25: `help` and `completion` are registered lazily by Execute(), not by
// an AddCommand call, so a test that never calls Execute() does not see them. The two Init calls
// below are what Execute() itself does first, and cobra makes both idempotent — calling them here
// gives the same 27 a user gets, without running a command.
func rootCommandNames(t *testing.T) []string {
	t.Helper()

	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultCompletionCmd()

	names := make([]string, 0, len(rootCmd.Commands()))
	for _, c := range rootCmd.Commands() {
		names = append(names, c.Name())
	}
	sort.Strings(names)
	return names
}

func staticCommandNames(t *testing.T) []string {
	t.Helper()

	m := buildManifest(&config.Config{})
	names := make([]string, 0, len(m.StaticCommands))
	for k := range m.StaticCommands {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// missing returns the elements of want that have absent from got, so a failure can name the
// commands rather than only report two counts that differ.
func missing(want, got []string) []string {
	have := make(map[string]bool, len(got))
	for _, g := range got {
		have[g] = true
	}
	var out []string
	for _, w := range want {
		if !have[w] {
			out = append(out, w)
		}
	}
	return out
}

func TestStaticCommandsCoverEveryRootCommand(t *testing.T) {
	root := rootCommandNames(t)
	static := staticCommandNames(t)

	t.Logf("rootCmd=%d static_commands=%d", len(root), len(static))

	if undocumented := missing(root, static); len(undocumented) > 0 {
		t.Errorf("%d command(s) registered on rootCmd with no static_commands entry: %s\n"+
			"add them to buildManifest — an agent reading the manifest concludes they do not exist",
			len(undocumented), strings.Join(undocumented, ", "))
	}

	if phantom := missing(static, root); len(phantom) > 0 {
		t.Errorf("%d static_commands entr(ies) name no real command: %s",
			len(phantom), strings.Join(phantom, ", "))
	}
}

// TestStaticCommandsAgreeWithReservedCommands checks the third source of truth. reservedCommands
// decides which interaction keys get shadowed by a builtin, so a command present in root and in
// the manifest but absent from reserved.go would let a user declare an interaction that silently
// never runs — the TASK-076 failure, reached from a different direction.
func TestStaticCommandsAgreeWithReservedCommands(t *testing.T) {
	reserved := config.ReservedCommands()
	names := make([]string, 0, len(reserved))
	for k := range reserved {
		names = append(names, k)
	}
	sort.Strings(names)

	static := staticCommandNames(t)
	t.Logf("reservedCommands=%d static_commands=%d", len(names), len(static))

	if diff := missing(names, static); len(diff) > 0 {
		t.Errorf("%d reserved command(s) undocumented: %s", len(diff), strings.Join(diff, ", "))
	}
	if diff := missing(static, names); len(diff) > 0 {
		t.Errorf("%d documented command(s) not reserved: %s", len(diff), strings.Join(diff, ", "))
	}
}

// TestEveryStaticCommandCarriesAType guards the half-filled entry. A new command added to the
// table with a description but no type still satisfies the count checks above, and Type is the
// field a consumer switches on.
func TestEveryStaticCommandCarriesAType(t *testing.T) {
	m := buildManifest(&config.Config{})

	known := map[string]bool{
		"dynamic_router": true, "query": true, "passthrough": true, "compose_shortcut": true,
		"lifecycle": true, "config": true, "meta": true, "info": true,
	}

	checked := 0
	for name, cmd := range m.StaticCommands {
		checked++
		if strings.TrimSpace(cmd.Description) == "" {
			t.Errorf("%q has an empty description", name)
		}
		if !known[cmd.Type] {
			t.Errorf("%q has type %q, which is not one of the eight in use — "+
				"either use an existing type or add the new one to this test deliberately",
				name, cmd.Type)
		}
	}
	t.Logf("checked=%d", checked)
}

// TestStaticCommandDescriptionsMatchTheirShort covers only the entries TASK-096 added, by
// construction: it asserts nothing about a command whose description already differed from its
// Short. Twelve of the original thirteen do differ — `version` is the only one that matches — and
// two of those (`up`, `down`) are stale rather than merely reworded. That is a real finding but
// not this task's; it is TASK-105. The test still has teeth for the 14, which is where a future
// copy-paste error would land.
func TestStaticCommandDescriptionsMatchTheirShort(t *testing.T) {
	added := []string{
		"stack", "app", "ssh", "infra", "logs", "restart", "console",
		"status", "show", "doctor", "config", "init", "help", "completion",
	}

	rootCommandNames(t) // ensure help and completion exist before we look them up
	byName := map[string]*cobra.Command{}
	for _, c := range rootCmd.Commands() {
		byName[c.Name()] = c
	}

	m := buildManifest(&config.Config{})
	for _, name := range added {
		cmd, ok := byName[name]
		if !ok {
			t.Errorf("%q is in the added set but not registered on rootCmd", name)
			continue
		}
		entry, ok := m.StaticCommands[name]
		if !ok {
			t.Errorf("%q is in the added set but has no static_commands entry", name)
			continue
		}
		if entry.Description != cmd.Short {
			t.Errorf("%q description drifted from its Short:\n manifest: %q\n cobra:    %q",
				name, entry.Description, cmd.Short)
		}
	}
	t.Logf("checked=%d", len(added))
}
