package cli

import (
	"strings"
	"testing"
)

const interactionEnvFileRemovedGuidance = "removed from interaction: declare shared inputs in the top-level 'env_file:', or inline command-local values under this command's 'environment:'"

// TestValidateRejectsInteractionEnvFile is Stage B: schema rejection, not a warning.
// Default validate must fail so a leftover declaration cannot look valid.
func TestValidateRejectsInteractionEnvFile(t *testing.T) {
	configPath := writeValidateConfigForTest(t, `version: "0.1.44"
interaction:
  rails:
    command: bundle exec rails
    env_file: .env.rails
`)

	defaultRun := runValidateCommandForTest(t, configPath, "validate")
	if defaultRun.err == "" {
		t.Fatal("default validate succeeded, want schema rejection of interaction env_file")
	}
	if !strings.Contains(defaultRun.err, "env_file") {
		t.Errorf("validate error does not name env_file: %s", defaultRun.err)
	}
	combined := defaultRun.err + defaultRun.stdout + defaultRun.stderr
	if !strings.Contains(combined, interactionEnvFileRemovedGuidance) {
		t.Errorf("validate output missing path-scoped removal guidance:\n%s", combined)
	}
}

// TestValidateJSONRejectsNestedInteractionEnvFile pins nested subcommand rejection
// on the JSON validate path as a hard error, not a semantic warning.
func TestValidateJSONRejectsNestedInteractionEnvFile(t *testing.T) {
	_, _, err := runValidate(t, `version: "0.1.44"
interaction:
  rails:
    command: bundle exec rails
    subcommands:
      db:
        command: db:migrate
        env_file:
          - path: .env.db
            required: true
`, true)
	if err == nil {
		t.Fatal("json validate succeeded, want schema rejection")
	}
	if !strings.Contains(err.Error(), "env_file") {
		t.Errorf("json validate error does not name env_file: %v", err)
	}
}
