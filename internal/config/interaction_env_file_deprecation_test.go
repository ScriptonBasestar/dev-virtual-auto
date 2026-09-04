package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const interactionEnvFileRemovedGuidance = "removed from interaction: declare shared inputs in the top-level 'env_file:', or inline command-local values under this command's 'environment:'"

func validateYAML(t *testing.T, body string) error {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return (&Config{filePath: path}).Validate()
}

// TestSchemaRejectsInteractionEnvFileWithPathScopedGuidance is Stage B: the field is
// gone from the schema, and the additional-property error names the frozen replacement.
func TestSchemaRejectsInteractionEnvFileWithPathScopedGuidance(t *testing.T) {
	err := validateYAML(t, `version: "0.1.44"
interaction:
  rails:
    command: bundle exec rails
    env_file: .env.rails
    subcommands:
      db:
        command: db:migrate
        env_file:
          - path: .env.db
            required: true
`)
	if err == nil {
		t.Fatal("Validate() succeeded, want schema rejection of interaction env_file")
	}
	got := err.Error()
	if !strings.Contains(got, "env_file") {
		t.Errorf("error does not name env_file:\n%s", got)
	}
	if !strings.Contains(got, interactionEnvFileRemovedGuidance) {
		t.Errorf("error missing path-scoped removal guidance:\n%s", got)
	}
	if strings.Contains(got, "inert and will be rejected") {
		t.Errorf("Stage A warning leaked into Stage B schema rejection:\n%s", got)
	}
}

// TestSchemaKeepsRootEnvFileWithoutInteractionGuidance is why the map is path-scoped:
// a valid top-level env_file must not grow the interaction-removal sentence.
func TestSchemaKeepsRootEnvFileWithoutInteractionGuidance(t *testing.T) {
	err := validateYAML(t, `version: "0.1.44"
env_file:
  - .env
interaction:
  rails:
    command: bundle exec rails
`)
	if err != nil {
		t.Fatalf("valid root env_file must validate: %v", err)
	}
}

// TestMigrateReportsInteractionEnvFileWithoutRewriting covers the migrate half of Stage A.
// The declaration has to appear under Blocked — not Changes — and the returned document has
// to still contain it: `migrate` writes its output back, so a step that dropped the key here
// would silently edit a user's file on a command that promised to report.
func TestMigrateReportsInteractionEnvFileWithoutRewriting(t *testing.T) {
	src := `version: "0.1.44"
env_file:
  - .env
interaction:
  rails:
    command: bundle exec rails
    env_file: .env.rails
    subcommands:
      db:
        command: db:migrate
        env_file:
          - path: .env.db
            required: true
      console:
        command: console
`

	out, report, err := Migrate([]byte(src))
	if err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	want := []string{
		"interaction.rails.env_file: " + InteractionEnvFileBlockedMessage,
		"interaction.rails.subcommands.db.env_file: " + InteractionEnvFileBlockedMessage,
	}
	for _, w := range want {
		if !slices.Contains(report.Blocked, w) {
			t.Errorf("Blocked missing %q:\n%s", w, strings.Join(report.Blocked, "\n"))
		}
	}
	for _, ch := range report.Changes {
		if strings.Contains(ch, "env_file") {
			t.Errorf("interaction env_file must be blocked, not converted: %q", ch)
		}
	}
	if !strings.Contains(string(out), "env_file: .env.rails") {
		t.Errorf("migrate must not rewrite the declaration:\n%s", out)
	}
	if !strings.Contains(string(out), ".env.db") {
		t.Errorf("migrate must not rewrite the nested declaration:\n%s", out)
	}
}

// TestMigrateLeavesRootEnvFileAlone is the counterpart the path-scoping exists for: the
// top-level `env_file:` is valid and is one of the two replacements the guidance names, so a
// walk that matched on the key name alone would tell the author to remove their fix.
func TestMigrateLeavesRootEnvFileAlone(t *testing.T) {
	src := `version: "0.1.44"
env_file:
  - .env
interaction:
  rails:
    command: bundle exec rails
`

	_, report, err := Migrate([]byte(src))
	if err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	for _, b := range report.Blocked {
		if strings.Contains(b, "env_file") {
			t.Errorf("root env_file must not be reported: %q", b)
		}
	}
}

// TestMigrateNamesModuleScopeGap pins TASK-285 §1: a module declared under `modules:` is a
// separate file `Migrate` never opens, so an `interaction.*.env_file` written inside it is
// invisible to ReportInteractionEnvFile. The gap cannot be closed without reading the module
// file (out of scope here, TASK-285's direction), so this pins that the config is instead
// named in "Left for you" — the reader is told the coverage stops at this document rather
// than being left to assume "no blocked entries" means "no deprecations".
func TestMigrateNamesModuleScopeGap(t *testing.T) {
	src := `version: "0.1.44"
modules:
  - extra
interaction:
  clean:
    command: echo clean
`

	_, report, err := Migrate([]byte(src))
	if err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	found := false
	for _, b := range report.Blocked {
		if strings.Contains(b, "modules:") && strings.Contains(b, "extra") {
			found = true
			if !strings.Contains(b, "dva config validate") {
				t.Errorf("module-scope entry must point at 'dva config validate' for the merged view: %q", b)
			}
		}
	}
	if !found {
		t.Errorf("Blocked does not name the modules declared, so a module-declared env_file "+
			"would be silently absent from \"Left for you\":\n%s", strings.Join(report.Blocked, "\n"))
	}
}

// TestMigrateSkipsModuleScopeGapWithoutModules guards the counterpart: a config declaring no
// `modules:` must not grow a phantom entry pointing at nothing.
func TestMigrateSkipsModuleScopeGapWithoutModules(t *testing.T) {
	src := `version: "0.1.44"
interaction:
  clean:
    command: echo clean
`

	_, report, err := Migrate([]byte(src))
	if err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	for _, b := range report.Blocked {
		if strings.Contains(b, "modules:") {
			t.Errorf("no modules declared, so no module-scope entry is expected: %q", b)
		}
	}
}
