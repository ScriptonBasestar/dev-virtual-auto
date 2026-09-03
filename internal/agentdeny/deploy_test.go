package agentdeny

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/skillinstall"
)

func testOptions(t *testing.T) (Options, string) {
	t.Helper()
	root := t.TempDir()
	return Options{
		Scope:       skillinstall.ScopeProject,
		HomeDir:     filepath.Join(root, "home"),
		ProjectRoot: filepath.Join(root, "project"),
		StateRoot:   filepath.Join(root, "state"),
		Version:     "test",
	}, root
}

func settingsPath(root string) string {
	return filepath.Join(root, "project", ".claude", "settings.json")
}

func TestInstallCreatesFreshSettingsFile(t *testing.T) {
	options, root := testOptions(t)
	result, err := Install(options)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(result.Destinations) != 1 || result.Destinations[0].Status != "installed" {
		t.Fatalf("unexpected result: %+v", result)
	}
	want := desiredPatterns()
	if !stringsEqual(result.Destinations[0].Patterns, want) {
		t.Fatalf("got patterns %v, want %v", result.Destinations[0].Patterns, want)
	}

	contents, err := os.ReadFile(settingsPath(root))
	if err != nil {
		t.Fatalf("read settings file: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(contents, &doc); err != nil {
		t.Fatalf("settings file is not valid JSON: %v", err)
	}
}

// TestInstallDoesNotClobberSiblingContent is completion criterion 5: installing must
// never destroy a key or array entry it did not add, at the value level, even though the
// touched containers are reformatted.
func TestInstallDoesNotClobberSiblingContent(t *testing.T) {
	options, root := testOptions(t)
	dest := settingsPath(root)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	seed := `{
  "unrelatedTopLevelKey": {"nested": [1, 2, 3]},
  "permissions": {
    "allow": ["Bash(git status)"],
    "ask": ["Bash(rm*)"],
    "deny": ["Bash(curl*)"]
  }
}`
	if err := os.WriteFile(dest, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Install(options); err != nil {
		t.Fatalf("Install: %v", err)
	}

	contents, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(contents, &doc); err != nil {
		t.Fatalf("settings file is not valid JSON after install: %v", err)
	}

	unrelated, ok := doc["unrelatedTopLevelKey"].(map[string]any)
	if !ok {
		t.Fatalf("unrelatedTopLevelKey was clobbered: %v", doc["unrelatedTopLevelKey"])
	}
	nested, ok := unrelated["nested"].([]any)
	if !ok || len(nested) != 3 {
		t.Fatalf("unrelatedTopLevelKey.nested was clobbered: %v", unrelated["nested"])
	}

	perms := doc["permissions"].(map[string]any)
	allow := perms["allow"].([]any)
	if len(allow) != 1 || allow[0] != "Bash(git status)" {
		t.Fatalf("permissions.allow was clobbered: %v", allow)
	}
	ask := perms["ask"].([]any)
	if len(ask) != 1 || ask[0] != "Bash(rm*)" {
		t.Fatalf("permissions.ask was clobbered: %v", ask)
	}
	deny := perms["deny"].([]any)
	found := map[string]bool{}
	for _, entry := range deny {
		found[entry.(string)] = true
	}
	if !found["Bash(curl*)"] {
		t.Fatalf("pre-existing user deny entry was dropped: %v", deny)
	}
	for _, pattern := range desiredPatterns() {
		if !found[pattern] {
			t.Errorf("desired pattern %q was not added: %v", pattern, deny)
		}
	}
}

func TestInstallIsIdempotent(t *testing.T) {
	options, _ := testOptions(t)
	if _, err := Install(options); err != nil {
		t.Fatalf("first Install: %v", err)
	}
	result, err := Install(options)
	if err != nil {
		t.Fatalf("second Install: %v", err)
	}
	if result.Destinations[0].Status != "up-to-date" {
		t.Fatalf("expected up-to-date on second install, got %q", result.Destinations[0].Status)
	}
}

func TestInstallDryRunWritesNothing(t *testing.T) {
	options, root := testOptions(t)
	options.DryRun = true
	result, err := Install(options)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if result.Destinations[0].Status != "would-install" {
		t.Fatalf("expected would-install, got %q", result.Destinations[0].Status)
	}
	if _, err := os.Stat(settingsPath(root)); !os.IsNotExist(err) {
		t.Fatalf("dry-run install must not create the settings file, stat err: %v", err)
	}
	if _, hadReceipt, _ := readReceipt(receiptPath(options.StateRoot, settingsPath(root))); hadReceipt {
		t.Fatal("dry-run install must not write a receipt")
	}
}

func TestStatusReportsAbsentWithoutReceipt(t *testing.T) {
	options, _ := testOptions(t)
	result, err := Status(options)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if result.Destinations[0].Status != "absent" {
		t.Fatalf("expected absent, got %q", result.Destinations[0].Status)
	}
}

func TestStatusDetectsDrift(t *testing.T) {
	options, root := testOptions(t)
	if _, err := Install(options); err != nil {
		t.Fatalf("Install: %v", err)
	}

	dest := settingsPath(root)
	contents, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	deny, err := claudeCodeDenyArray(contents)
	if err != nil {
		t.Fatal(err)
	}
	remaining, removedCount := removeExact(deny, deny[:1])
	if removedCount != 1 {
		t.Fatalf("test setup: expected to remove exactly one entry, removed %d", removedCount)
	}
	top, err := loadJSONObject(contents)
	if err != nil {
		t.Fatal(err)
	}
	_, perms, err := readDenyArray(top)
	if err != nil {
		t.Fatal(err)
	}
	rewritten, err := writeDenyArray(top, perms, remaining)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, rewritten, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Status(options)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if result.Destinations[0].Status != "drifted" {
		t.Fatalf("expected drifted, got %q (%s)", result.Destinations[0].Status, result.Destinations[0].Detail)
	}
}

// TestUninstallRemovesOnlyDVAOwnedEntries is completion criteria 4 and 5: uninstall must
// remove exactly the patterns DVA added and leave a user-added deny entry alone.
func TestUninstallRemovesOnlyDVAOwnedEntries(t *testing.T) {
	options, root := testOptions(t)
	dest := settingsPath(root)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte(`{"permissions": {"deny": ["Bash(curl*)"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(options); err != nil {
		t.Fatalf("Install: %v", err)
	}

	result, err := Uninstall(options)
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if result.Destinations[0].Status != "uninstalled" {
		t.Fatalf("expected uninstalled, got %q (%s)", result.Destinations[0].Status, result.Destinations[0].Detail)
	}
	remaining := result.Destinations[0].Patterns
	if len(remaining) != 1 || remaining[0] != "Bash(curl*)" {
		t.Fatalf("uninstall must leave the user-added entry alone, got %v", remaining)
	}

	if _, hadReceipt, _ := readReceipt(receiptPath(options.StateRoot, dest)); hadReceipt {
		t.Fatal("uninstall must remove the receipt")
	}

	statusAfter, err := Status(options)
	if err != nil {
		t.Fatalf("Status after uninstall: %v", err)
	}
	if statusAfter.Destinations[0].Status != "absent" {
		t.Fatalf("expected absent after uninstall, got %q", statusAfter.Destinations[0].Status)
	}
}

// TestUninstallRefusesOnLocalModification is completion criterion 4's local-modification
// detection: if a user has already removed a DVA-owned deny pattern, uninstall must
// refuse rather than silently proceeding with a partial, guessed removal.
func TestUninstallRefusesOnLocalModification(t *testing.T) {
	options, root := testOptions(t)
	if _, err := Install(options); err != nil {
		t.Fatalf("Install: %v", err)
	}

	dest := settingsPath(root)
	contents, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	deny, err := claudeCodeDenyArray(contents)
	if err != nil {
		t.Fatal(err)
	}
	remaining, removedCount := removeExact(deny, deny[:1])
	if removedCount != 1 {
		t.Fatalf("test setup: expected to remove exactly one entry, removed %d", removedCount)
	}
	top, err := loadJSONObject(contents)
	if err != nil {
		t.Fatal(err)
	}
	_, perms, err := readDenyArray(top)
	if err != nil {
		t.Fatal(err)
	}
	rewritten, err := writeDenyArray(top, perms, remaining)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, rewritten, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Uninstall(options)
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if result.Destinations[0].Status != "recovery-required" {
		t.Fatalf("expected recovery-required on drifted uninstall, got %q", result.Destinations[0].Status)
	}

	// Refusal must be a no-op: the receipt and the (already-drifted) file are both left
	// exactly as Uninstall found them, so nothing is lost by the refusal itself.
	if _, hadReceipt, _ := readReceipt(receiptPath(options.StateRoot, dest)); !hadReceipt {
		t.Fatal("refused uninstall must not remove the receipt")
	}
}

func TestUninstallWithoutReceiptIsNotInstalled(t *testing.T) {
	options, _ := testOptions(t)
	result, err := Uninstall(options)
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if result.Destinations[0].Status != "not-installed" {
		t.Fatalf("expected not-installed, got %q", result.Destinations[0].Status)
	}
}

func TestUninstallDryRunWritesNothing(t *testing.T) {
	options, root := testOptions(t)
	if _, err := Install(options); err != nil {
		t.Fatalf("Install: %v", err)
	}
	before, err := os.ReadFile(settingsPath(root))
	if err != nil {
		t.Fatal(err)
	}

	options.DryRun = true
	result, err := Uninstall(options)
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if result.Destinations[0].Status != "would-uninstall" {
		t.Fatalf("expected would-uninstall, got %q", result.Destinations[0].Status)
	}
	after, err := os.ReadFile(settingsPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("dry-run uninstall must not modify the settings file")
	}
	dest := settingsPath(root)
	if _, hadReceipt, _ := readReceipt(receiptPath(options.StateRoot, dest)); !hadReceipt {
		t.Fatal("dry-run uninstall must not remove the receipt")
	}
}

func TestInstallRefusesInvalidJSON(t *testing.T) {
	options, root := testOptions(t)
	dest := settingsPath(root)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte(`{ not valid json`), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Install(options)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if result.Destinations[0].Status != "recovery-required" {
		t.Fatalf("expected recovery-required for invalid JSON, got %q", result.Destinations[0].Status)
	}
	unchanged, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(unchanged) != `{ not valid json` {
		t.Fatal("install must not touch a settings file it cannot parse")
	}
}

// TestInstallWritesWrappedAndSpacedPatternsToDisk is the Blocker-1 regression proof: it
// reads the literal bytes Install wrote to the real settings file — not the in-memory
// Result — and asserts DVA's own generated deny entries carry the "Bash(...)" wrapper and
// the space before the trailing "*". An earlier version of Patterns() emitted a bare,
// unwrapped argv string that satisfied every other test in this file (they all compare
// against desiredPatterns(), which just echoes Patterns()'s own output back at itself)
// while enforcing nothing against Claude Code, because a bare deny entry names a tool, not
// a command. This test instead pins the exact on-disk literal so a future regression to
// the unwrapped form fails here even if Patterns() and its caller agree with each other.
func TestInstallWritesWrappedAndSpacedPatternsToDisk(t *testing.T) {
	options, root := testOptions(t)
	if _, err := Install(options); err != nil {
		t.Fatalf("Install: %v", err)
	}

	contents, err := os.ReadFile(settingsPath(root))
	if err != nil {
		t.Fatalf("read settings file: %v", err)
	}
	deny, err := claudeCodeDenyArray(contents)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, entry := range deny {
		found[entry] = true
	}
	wantOnDisk := []string{
		"Bash(dva config env seal *)",
		"Bash(dva config env show *)",
	}
	for _, want := range wantOnDisk {
		if !found[want] {
			t.Errorf("settings file deny array %v is missing the wrapped-and-spaced literal %q", deny, want)
		}
	}
	for _, entry := range deny {
		if !strings.HasPrefix(entry, "Bash(") || !strings.HasSuffix(entry, " *)") {
			t.Errorf("deny entry %q written by Install is not in Bash(<argv> *) form", entry)
		}
	}
}

// TestInstallDoesNotClaimOwnershipOfPreExistingUserPattern is the receipt-ownership-delta
// regression proof: if the user already had one of DVA's exact desired deny patterns in
// their settings file before ever running install — hand-written, or added by some other
// tool — install must not record that pattern as DVA-owned in its receipt. Otherwise a
// later uninstall deletes a rule DVA never added, the same clobber class completion
// criterion 5 exists to prevent for every other kind of pre-existing entry.
func TestInstallDoesNotClaimOwnershipOfPreExistingUserPattern(t *testing.T) {
	options, root := testOptions(t)
	dest := settingsPath(root)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	preExisting := "Bash(dva config env show *)"
	seed := `{"permissions": {"deny": ["` + preExisting + `"]}}`
	if err := os.WriteFile(dest, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Install(options); err != nil {
		t.Fatalf("Install: %v", err)
	}

	result, err := Uninstall(options)
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if result.Destinations[0].Status != "uninstalled" {
		t.Fatalf("expected uninstalled, got %q (%s)", result.Destinations[0].Status, result.Destinations[0].Detail)
	}
	remaining := result.Destinations[0].Patterns
	found := false
	for _, pattern := range remaining {
		if pattern == preExisting {
			found = true
		}
	}
	if !found {
		t.Fatalf("uninstall deleted a pattern the user had before ever running install: remaining=%v", remaining)
	}
}

func TestUserScopeTargetsHomeDirectory(t *testing.T) {
	options, root := testOptions(t)
	options.Scope = skillinstall.ScopeUser
	result, err := Install(options)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	want := filepath.Join(root, "home", ".claude", "settings.json")
	if result.Destinations[0].Destination != want {
		t.Fatalf("got destination %q, want %q", result.Destinations[0].Destination, want)
	}
}
