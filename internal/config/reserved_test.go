package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestValidateReservedCommands_NoConflict(t *testing.T) {
	interaction := map[string]*InteractionCommand{
		"shell": {Description: "Open shell"},
		"test":  {Description: "Run tests"},
		"lint":  {Description: "Run linter"},
	}

	conflicts := ValidateReservedCommands(interaction)
	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d: %v", len(conflicts), conflicts)
	}
}

func TestValidateReservedCommands_WithConflicts(t *testing.T) {
	interaction := map[string]*InteractionCommand{
		"shell": {Description: "Open shell"},
		"up":    {Description: "shadows reserved 'up'"},
		"build": {Description: "shadows reserved 'build'"},
	}

	conflicts := ValidateReservedCommands(interaction)
	if len(conflicts) != 2 {
		t.Fatalf("expected 2 conflicts, got %d", len(conflicts))
	}

	names := map[string]bool{}
	for _, c := range conflicts {
		names[c.Name] = true
	}
	if !names["up"] {
		t.Error("expected conflict for 'up'")
	}
	if !names["build"] {
		t.Error("expected conflict for 'build'")
	}
}

func TestValidateReservedCommands_EmptyInteraction(t *testing.T) {
	conflicts := ValidateReservedCommands(map[string]*InteractionCommand{})
	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts for empty interaction, got %d", len(conflicts))
	}
}

func TestValidateReservedCommands_AllReserved(t *testing.T) {
	reserved := ReservedCommands()
	interaction := make(map[string]*InteractionCommand, len(reserved))
	for name := range reserved {
		interaction[name] = &InteractionCommand{Description: "conflict"}
	}

	conflicts := ValidateReservedCommands(interaction)
	if len(conflicts) != len(reserved) {
		t.Errorf("expected %d conflicts, got %d", len(reserved), len(conflicts))
	}
}

func TestFormatConflictWarnings_Empty(t *testing.T) {
	msg := FormatConflictWarnings(nil)
	if msg != "" {
		t.Errorf("expected empty string, got %q", msg)
	}
}

func TestFormatConflictWarnings_Single(t *testing.T) {
	conflicts := []ReservedCommandConflict{
		{Name: "up", Source: "interaction"},
	}
	msg := FormatConflictWarnings(conflicts)
	if msg == "" {
		t.Error("expected non-empty warning message")
	}
	if !contains(msg, "'up'") {
		t.Errorf("expected message to contain 'up', got: %s", msg)
	}
}

func TestFormatConflictWarnings_Multiple(t *testing.T) {
	conflicts := []ReservedCommandConflict{
		{Name: "up", Source: "interaction"},
		{Name: "build", Source: "interaction"},
	}
	msg := FormatConflictWarnings(conflicts)
	if !contains(msg, "'up'") || !contains(msg, "'build'") {
		t.Errorf("expected message to contain both names, got: %s", msg)
	}
}

// The three conflict kinds do not share a way out, and the difference is not cosmetic: measured
// against bin/dva, a plain reserved name stays reachable as `dva run <name>` while a namespaced
// key is reachable by nothing. Advice that names an invocation which refuses is worse than none,
// so each branch is pinned to what was executed.
func TestConflictAdviceNamesOnlyInvocationsThatWork(t *testing.T) {
	cases := []struct {
		name       string
		key        string
		mustHave   []string
		mustNotHav []string
	}{
		{
			// `dva run status` → the interaction ran; bare `dva status` → the built-in ran.
			name:       "non-hookable reserved name",
			key:        "status",
			mustHave:   []string{"'status'", "dva run status", "my-status"},
			mustNotHav: []string{"ignored", "before/replace/after"},
		},
		{
			// Hookable: the hook route is the one that gets the short form working, so it leads.
			name:       "hookable reserved name",
			key:        "build",
			mustHave:   []string{"'build'", "before/replace/after", "dva run build"},
			mustNotHav: []string{"ignored"},
		},
		{
			// Measured: `dva app:build` → "subproject `app` not found", and so is
			// `dva run app:build`. Offering either would send the reader to a command that
			// exits 1. `app-build` was executed and printed the interaction's output.
			name:       "namespace prefix is reserved",
			key:        "app:build",
			mustHave:   []string{"'app'", "app-build"},
			mustNotHav: []string{"dva run app:build", "reachable only as"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			advice := ConflictAdvice(tc.key)
			for _, want := range tc.mustHave {
				if !strings.Contains(advice, want) {
					t.Errorf("advice for %q omits %q; got: %s", tc.key, want, advice)
				}
			}
			for _, unwanted := range tc.mustNotHav {
				if strings.Contains(advice, unwanted) {
					t.Errorf("advice for %q offers %q, which does not hold for this kind of conflict; got: %s", tc.key, unwanted, advice)
				}
			}
		})
	}
}

// usageDocSection returns the body of a USAGE.md section, from its heading to the next heading of
// the same or higher level.
func usageDocSection(t *testing.T, heading string) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(file), "..", "..", "USAGE.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read USAGE.md: %v", err)
	}

	lines := strings.Split(string(content), "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == heading {
			start = i + 1
			break
		}
	}
	// Not a soft skip. A renamed heading has to fail loudly, because the alternative is a guard
	// that scans an empty string and passes — which is the shape of every doc check that was
	// present and still let the claim through.
	if start < 0 {
		t.Fatalf("USAGE.md has no section titled %q; if it was renamed, update this guard", heading)
	}

	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") || strings.HasPrefix(lines[i], "### ") {
			return strings.Join(lines[start:i], "\n")
		}
	}
	return strings.Join(lines[start:], "\n")
}

// TestUsageDocDoesNotSayReservedConflictsAreIgnored extends the `mustNotHav: "ignored"` guard above
// from the Go advice strings to the document. The Go strings were covered; USAGE.md was not, and so
// it told the reader a conflicting name was "조용히 무시" — silently ignored — thirty lines above the
// paragraph saying the conflict is a hard error that fails validate with exit 1. Both cannot be
// true, and it is the earlier one a reader acts on.
//
// reserved.go's own comment already says why this matters: telling the reader the declaration was
// ignored sends them looking for a command that never ran. TASK-099.
func TestUsageDocDoesNotSayReservedConflictsAreIgnored(t *testing.T) {
	section := usageDocSection(t, "### interaction (예약어와 훅)")
	if strings.TrimSpace(section) == "" {
		t.Fatal("the section is empty, so every assertion below would pass vacuously")
	}

	for _, claim := range []string{"조용히 무시", "silently ignored"} {
		if strings.Contains(section, claim) {
			t.Errorf("USAGE.md still says %q; a reserved-name conflict fails `dva validate` with "+
				"exit 1 and warns on every config load — see ValidateReservedCommands", claim)
		}
	}

	// The negative alone is satisfied by deleting the sentence outright. This half requires the
	// section to still answer the question the reader arrived with.
	for _, fact := range []string{"exit 1", "에러"} {
		if !strings.Contains(section, fact) {
			t.Errorf("USAGE.md's interaction section no longer states %q; removing the wrong "+
				"claim is only half the fix", fact)
		}
	}

	t.Logf("section scanned: %d lines", len(strings.Split(section, "\n")))
}

// FormatConflictWarnings used to detail conflicts[0] only, which is a map-iteration artifact: the
// same config produced a message about a different command from run to run, and the two conflicts
// below need different advice, so one clause could not have covered both.
func TestFormatConflictWarningsIsStableAndCoversEveryConflict(t *testing.T) {
	conflicts := []ReservedCommandConflict{
		{Name: "status", Source: "interaction"},
		{Name: "app:build", Source: "interaction"},
	}

	first := FormatConflictWarnings(conflicts)
	// Reversed input, same conflicts: a message built in map order would differ here.
	reversed := FormatConflictWarnings([]ReservedCommandConflict{conflicts[1], conflicts[0]})
	if first != reversed {
		t.Errorf("message depends on input order:\n  %s\n  %s", first, reversed)
	}

	if !strings.Contains(first, "dva run status") {
		t.Errorf("message drops the invocation that reaches 'status': %s", first)
	}
	if !strings.Contains(first, "app-build") {
		t.Errorf("message drops the rename that fixes 'app:build': %s", first)
	}
	if strings.Contains(first, "dva run app:build") {
		t.Errorf("message offers 'dva run app:build', which exits 1: %s", first)
	}
}

func TestReservedCommandsMapIsPopulated(t *testing.T) {
	reserved := ReservedCommands()
	if len(reserved) == 0 {
		t.Fatal("ReservedCommands should not be empty")
	}
	// Spot-check a few known commands
	expected := []string{"help", "up", "down", "run", "init", "validate"}
	for _, cmd := range expected {
		if !reserved[cmd] {
			t.Errorf("ReservedCommands missing expected command: %s", cmd)
		}
	}
}

func TestConfigValidate_AllowsFormerPlaceholderInteractions(t *testing.T) {
	cfg := loadConfigForSchemaTest(t, t.TempDir(), `version: "0.1.44"
interaction:
  cmd:
    runner: local
    command: echo cmd
  migrate:
    runner: local
    command: make db-migrate
`)

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected former placeholder names to pass config validation: %v", err)
	}
}

func TestValidateReservedCommands_NamespacePrefixConflict(t *testing.T) {
	interaction := map[string]*InteractionCommand{
		"app:build":   {Description: "build app"},
		"infra:setup": {Description: "setup infra"},
		"cargo:build": {Description: "no conflict"},
	}

	conflicts := ValidateReservedCommands(interaction)
	if len(conflicts) != 2 {
		t.Fatalf("expected 2 conflicts (app:build, infra:setup), got %d: %v", len(conflicts), conflicts)
	}

	names := map[string]bool{}
	for _, c := range conflicts {
		names[c.Name] = true
	}
	if !names["app:build"] {
		t.Error("expected conflict for 'app:build'")
	}
	if !names["infra:setup"] {
		t.Error("expected conflict for 'infra:setup'")
	}
	if names["cargo:build"] {
		t.Error("'cargo:build' should not conflict")
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
