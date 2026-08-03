package cli

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/runner"
)

// namespacedInteractions declares one key behind a reserved prefix and one behind a free
// prefix. Both are namespaced, so a mark that keys off the colon rather than off the
// prefix would flag them alike.
func namespacedInteractions() *config.Config {
	return &config.Config{Interaction: map[string]*config.InteractionCommand{
		"app:build":   {Command: "echo hello", Description: "behind a reserved prefix"},
		"mytool:fast": {Command: "echo free", Description: "behind a free prefix"},
		"my-build":    {Command: "make build", Description: "no namespace at all"},
	}}
}

// TestUnroutableKeyFailsBothInvocationForms pins the mark to the behaviour it describes.
// `unroutable` is a claim that no invocation reaches the key; if either form ever starts
// working, the claim becomes the lie the mark exists to prevent.
//
// Both forms converge on the same failure. `dva app:build` is not a built-in
// (isTopLevelCommand is false), so root.go's dynamic routing rewrites it to
// `dva run app:build` — the key IS in c.Interaction, so the rewrite fires — and run.go
// then splits on the colon and reads `app:` as a subproject reference. That is the same
// code path `dva run app:build` takes directly, which is why one assertion covers both.
//
// The run form goes through runCmd.RunE with the key unsplit, rather than calling
// runSubprojectCommand("app", "build") directly. The direct call was the whole weakness:
// it hardcoded the two halves and so passed no matter what run.go:33 does with the colon.
// TASK-167's "route it" option edits exactly that split — looking the literal key up in
// c.Interaction before splitting — and this test is cited there as the tripwire that
// catches it. Pre-split arguments would have made that citation false.
func TestUnroutableKeyFailsBothInvocationForms(t *testing.T) {
	c := namespacedInteractions()

	// Bare form: nothing built-in serves this name, so it cannot be handled without the
	// rewrite, and the rewrite lands on the run form asserted below. These two conditions
	// are root.go:189 and root.go:194 — the rewrite fires iff both hold.
	if isTopLevelCommand("app:build") {
		t.Fatal("`app:build` resolved as a built-in command; the unroutable mark assumes it does not")
	}
	if c.Interaction["app:build"] == nil {
		t.Fatal("fixture lost the key; the routing rewrite in root.go keys off this lookup")
	}

	// Run form (and the bare form's destination), driven through the real entry point so
	// the colon is split by the code under test. RunE loads the config from the working
	// directory, so the fixture has to exist on disk.
	dir := t.TempDir()
	// No version: key — it declares the *minimum dva version*, not a schema version, so
	// "1.0" makes the load fail with an upgrade demand. That failure is fatal inside
	// mustLoadConfig (os.Exit(1)), which takes the whole test binary with it.
	fixture := "interaction:\n  \"app:build\":\n    command: echo hello\n"
	if err := os.WriteFile(filepath.Join(dir, "dva.yml"), []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	err := runCmd.RunE(runCmd, []string{"app:build"})
	if err == nil {
		t.Fatal("`dva run app:build` succeeded; the key is marked unroutable, so this must fail")
	}
	if !strings.Contains(err.Error(), "subproject `app` not found") {
		t.Errorf("run form failed with %q, want the subproject-not-found error the mark's reason cites", err)
	}

	// The reason string the surfaces publish must name the failure that actually happens,
	// or a consumer reading it is told a different story than the one the binary tells.
	reason := config.ConflictAdvice("app:build")
	if !strings.Contains(reason, "subproject 'app' not found") {
		t.Errorf("ConflictAdvice = %q, which does not name the error the run form returns", reason)
	}
}

// TestManifestMarksUnroutableNamespacedKey covers the state and the usage_example promise
// together: the mark is only worth adding if the dead invocation stops being advertised
// beside it.
func TestManifestMarksUnroutableNamespacedKey(t *testing.T) {
	m := buildManifest(namespacedInteractions())

	entry, ok := m.DynamicCommands["app:build"]
	if !ok {
		t.Fatal("manifest dropped `app:build`; an unroutable key must still be listed — the author declared it and needs to see dva received it")
	}
	if entry.Unroutable != "app" {
		t.Errorf("unroutable = %q, want %q", entry.Unroutable, "app")
	}
	if entry.ShadowedByBuiltin != "" {
		t.Errorf("shadowed_by_builtin = %q, want empty: nothing shadows this key, it is unreachable — reusing that field would tell a consumer `dva run app:build` works", entry.ShadowedByBuiltin)
	}
	if entry.UsageExample != "" {
		t.Errorf("usage_example = %q, want empty; every candidate string exits non-zero", entry.UsageExample)
	}
	if !strings.Contains(entry.UnroutableReason, "reserved DVA command") {
		t.Errorf("unroutable_reason = %q, want the ConflictAdvice sentence", entry.UnroutableReason)
	}
}

// TestSubcommandsOfAnUnroutableKeyAreMarkedToo covers the entries the author never wrote.
//
// A declared `app:build` carrying subcommands: flattens into `app:build` AND
// `app:build fast`. The dead prefix kills both — run.go splits args[0] and never reaches
// the rest — but the child arrives at interactionUsage with a two-segment path, so a
// length-guarded check marked only the parent and published `dva app:build fast` as a
// working invocation beside it. That is worse than no mark: the manifest's audience is
// agents, which read the unmarked sibling as the way in.
func TestSubcommandsOfAnUnroutableKeyAreMarkedToo(t *testing.T) {
	c := &config.Config{Interaction: map[string]*config.InteractionCommand{
		"app:build": {
			Command:     "echo top",
			Subcommands: map[string]*config.InteractionCommand{"fast": {Command: "echo fast"}},
		},
	}}
	m := buildManifest(c)

	entry, ok := m.DynamicCommands["app:build fast"]
	if !ok {
		t.Fatalf("manifest has no entry for the derived subcommand; keys: %v", manifestKeys(m))
	}
	if entry.Unroutable != "app" {
		t.Errorf("subcommand unroutable = %q, want %q — the prefix is dead for the child too", entry.Unroutable, "app")
	}
	if entry.UsageExample != "" {
		t.Errorf("usage_example = %q for a subcommand nothing reaches; measured, both `dva app:build fast` and `dva run app:build fast` exit 1", entry.UsageExample)
	}

	// The human mark has to name a key the author can actually edit. `app:build fast` is
	// assembled by the tree walk and appears nowhere in the file, so running it through the
	// rename produced `app-build fast` — advice to write a key containing a space.
	commands := runner.NewInteractionTree(c.Interaction).List()
	out := captureStdout(t, func() {
		if err := printTable(c, commands, sortedKeys(commands)); err != nil {
			t.Fatalf("printTable: %v", err)
		}
	})
	if strings.Contains(out, "app-build fast") {
		t.Errorf("ls advises renaming to the flattened display name, which is not a key:\n%s", out)
	}
	if n := strings.Count(out, "rename to 'app-build'"); n != 2 {
		t.Errorf("want both rows pointing at the one declared key to rename, got %d:\n%s", n, out)
	}
}

// TestRenameSuggestionLeavesNoColon pins the advice to something that actually runs.
//
// The suggestion used to replace the first colon only, so `app:sub:cmd` was told to become
// `app-sub:cmd` — still split by run.go, still failing, and now with a free prefix, so
// validate reports the config clean and ls drops the mark. Taking the tool's advice moved
// the author from a caught error to a silent one.
func TestRenameSuggestionLeavesNoColon(t *testing.T) {
	for _, name := range []string{"app:build", "app:sub:cmd", "a:b:c:d"} {
		got := config.RenameSuggestion(name)
		if strings.Contains(got, ":") {
			t.Errorf("RenameSuggestion(%q) = %q, which run.go still splits — the advice names a command that cannot work", name, got)
		}
		if config.UnroutableNamespacePrefix(got) != "" {
			t.Errorf("RenameSuggestion(%q) = %q, still unroutable", name, got)
		}
	}

	// The advice string is the copy users actually read; it must carry the fixed form.
	if advice := config.ConflictAdvice("app:sub:cmd"); !strings.Contains(advice, "app-sub-cmd") {
		t.Errorf("ConflictAdvice = %q, which does not offer the colon-free rename", advice)
	}
}

// manifestKeys is for failure messages only — a missing entry is far easier to diagnose
// with the actual key set in hand than with "not found".
func manifestKeys(m *Manifest) []string {
	keys := make([]string, 0, len(m.DynamicCommands))
	for k := range m.DynamicCommands {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// TestFreePrefixNamespacedKeyIsNotMarked keeps the mark keyed off the reserved prefix
// rather than off the colon.
//
// `mytool:fast` is in fact unreachable too — run.go splits every key on ':' — but that is
// a routing defect with a different answer (TASK-167), and marking it here would report a
// reserved-command collision that did not happen.
func TestFreePrefixNamespacedKeyIsNotMarked(t *testing.T) {
	m := buildManifest(namespacedInteractions())

	for _, key := range []string{"mytool:fast", "my-build"} {
		entry, ok := m.DynamicCommands[key]
		if !ok {
			t.Fatalf("manifest has no entry for %q", key)
		}
		if entry.Unroutable != "" {
			t.Errorf("%q: unroutable = %q, want empty — no reserved command is involved", key, entry.Unroutable)
		}
		if entry.UnroutableReason != "" {
			t.Errorf("%q: unroutable_reason = %q, want empty", key, entry.UnroutableReason)
		}
	}
}

// TestLsJSONExposesTheSameUnroutableState is the parity check. The two surfaces share
// interactionUsage, so this fails only if a call site stops forwarding the third state —
// which is exactly how shadowed_by_builtin could have drifted before TASK-076 unified them.
func TestLsJSONExposesTheSameUnroutableState(t *testing.T) {
	c := namespacedInteractions()
	commands, keys := resolve(t, c)
	entries := buildCommandEntries(c, commands, keys)
	m := buildManifest(c)

	var checked int
	for _, k := range keys {
		entry, ok := entries[k].(map[string]any)
		if !ok {
			t.Fatalf("ls entry for %q is not an object", k)
		}
		lsUnroutable, _ := entry["unroutable"].(string)
		if lsUnroutable != m.DynamicCommands[k].Unroutable {
			t.Errorf("%q: ls says unroutable=%q, manifest says %q", k, lsUnroutable, m.DynamicCommands[k].Unroutable)
		}
		lsReason, _ := entry["unroutable_reason"].(string)
		if lsReason != m.DynamicCommands[k].UnroutableReason {
			t.Errorf("%q: ls and manifest disagree on unroutable_reason", k)
		}
		checked++
	}

	// Comparing two empty maps agrees vacuously.
	if checked != 3 {
		t.Fatalf("compared %d keys, want 3 — the fixture or the resolver changed and this proved nothing", checked)
	}
	if m.DynamicCommands["app:build"].Unroutable == "" {
		t.Fatal("the marked key carried no mark, so the parity above compared empty strings")
	}
}
