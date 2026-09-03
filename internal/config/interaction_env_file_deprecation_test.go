package config

import (
	"slices"
	"strings"
	"testing"
)

// TestWarnInertInteractionEnvFile pins the shape TASK-265 §4 froze: one warning per
// declaring node, at the node's own dotted path, carrying the exact announced text.
//
// The fixture declares at three depths deliberately. Depth 1 is what a check written
// before the walker existed would have found on its own; the two nested declarations are
// what a non-recursive check would silently pass, which is the failure mode
// warnInertProvisionSteps already records for the same tree.
func TestWarnInertInteractionEnvFile(t *testing.T) {
	c := &Config{
		Interaction: map[string]*InteractionCommand{
			"rails": {
				Command: "bundle exec rails",
				EnvFile: ".env.rails",
				Subcommands: map[string]*InteractionCommand{
					"db": {
						EnvFile: []any{".env.db"},
						Subcommands: map[string]*InteractionCommand{
							"migrate": {
								Command: "db:migrate",
								EnvFile: map[string]any{"path": ".env.migrate", "required": true},
							},
							"seed": {Command: "db:seed"},
						},
					},
					"console": {Command: "console"},
				},
			},
			"clean": {Command: "echo clean"},
		},
	}

	got := c.warnInertInteractionEnvFile()
	want := []string{
		"interaction.rails.subcommands.db.subcommands.migrate: " + InteractionEnvFileMessage,
		"interaction.rails.subcommands.db: " + InteractionEnvFileMessage,
		"interaction.rails: " + InteractionEnvFileMessage,
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d warnings, got %d:\n%s", len(want), len(got), strings.Join(got, "\n"))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("warning %d:\n got  %s\n want %s", i, got[i], want[i])
		}
	}
}

// TestWarnInertInteractionEnvFileIgnoresNonDeclaringConfigs guards the two shapes that must
// stay quiet: a config with no interaction section at all, and one whose only `env_file:` is
// the top-level declaration the field is being redirected to. Warning on the latter would
// tell the author to move a declaration that is already where it belongs.
func TestWarnInertInteractionEnvFileIgnoresNonDeclaringConfigs(t *testing.T) {
	for name, c := range map[string]*Config{
		"no interaction": {EnvFile: []any{".env"}},
		"root env_file only": {
			EnvFile: []any{".env"},
			Interaction: map[string]*InteractionCommand{
				"rails": {
					Command:     "bundle exec rails",
					Environment: map[string]string{"RAILS_ENV": "test"},
					Subcommands: map[string]*InteractionCommand{"db": {Command: "db:migrate"}},
				},
			},
		},
	} {
		if got := c.warnInertInteractionEnvFile(); len(got) != 0 {
			t.Errorf("%s: expected no warnings, got:\n%s", name, strings.Join(got, "\n"))
		}
	}
}

// TestWarnInertInteractionEnvFileReachesValidateWarnings proves the check is registered.
// A warning function nobody calls passes its own unit test and reports nothing to a user,
// which is exactly the failure this deprecation exists to avoid repeating.
func TestWarnInertInteractionEnvFileReachesValidateWarnings(t *testing.T) {
	c := &Config{
		Interaction: map[string]*InteractionCommand{
			"rails": {Command: "bundle exec rails", EnvFile: ".env.rails"},
		},
	}

	want := "interaction.rails: " + InteractionEnvFileMessage
	got := c.ValidateWarnings()
	if !slices.Contains(got, want) {
		t.Errorf("ValidateWarnings() does not carry the interaction env_file warning:\n%s",
			strings.Join(got, "\n"))
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
