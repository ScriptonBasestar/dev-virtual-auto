package cli

import (
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// TestValidateAnnouncesInteractionEnvFileDeprecation is the end-to-end half of TASK-266
// Stage A: the announcement has to travel the channel that already exists — the
// `[warn] semantic:` prefix — leave the default exit code alone, and be promoted by
// --strict through the pre-existing semantic rule rather than a rule of its own.
//
// The default-run assertion is the one that matters most. Stage A changes no runtime
// behavior, so a non-zero exit here would break configs that work today over a field that
// has never done anything.
func TestValidateAnnouncesInteractionEnvFileDeprecation(t *testing.T) {
	configPath := writeValidateConfigForTest(t, `version: "0.1.44"
interaction:
  rails:
    command: bundle exec rails
    env_file: .env.rails
`)

	defaultRun := runValidateCommandForTest(t, configPath, "validate")
	if defaultRun.err != "" {
		t.Fatalf("default run returned an error: %s", defaultRun.err)
	}
	want := "[warn] semantic: interaction.rails: " + config.InteractionEnvFileMessage
	if !strings.Contains(defaultRun.stdout, want) {
		t.Errorf("default run stdout missing the deprecation under [warn] semantic:\n%s", defaultRun.stdout)
	}

	strictRun := runValidateCommandForTest(t, configPath, "validate", "--strict")
	if strictRun.err == "" {
		t.Fatal("--strict run succeeded, want non-zero exit on a semantic warning")
	}
}

// TestValidateInteractionEnvFileUsesSemanticJSONCategory pins the JSON side: the existing
// "semantic" category, not a new one. TASK-266 forbids a new category, flag or route, and a
// JSON consumer keying off category names is exactly who a new one would break.
func TestValidateInteractionEnvFileUsesSemanticJSONCategory(t *testing.T) {
	out, _, err := runValidate(t, `version: "0.1.44"
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
	if err != nil {
		t.Fatalf("json run returned an error: %v", err)
	}

	doc := decodeOneDocument(t, out)
	want := "interaction.rails.subcommands.db: " + config.InteractionEnvFileMessage
	for _, w := range doc.Warnings {
		if w.Category == "semantic" && w.Message == want {
			return
		}
	}
	t.Errorf("no semantic-category warning carries the nested deprecation; got %+v", doc.Warnings)
}

// TestNonValidateRoutesStaySilentAboutInteractionEnvFile holds the other half of the
// criterion: only `dva config validate` says anything. `show` loads the same config through
// the same pipeline, so if the announcement ever leaked into config loading rather than
// validation, this is where it would surface.
func TestNonValidateRoutesStaySilentAboutInteractionEnvFile(t *testing.T) {
	configPath := writeValidateConfigForTest(t, `version: "0.1.44"
interaction:
  rails:
    command: bundle exec rails
    env_file: .env.rails
`)

	run := runValidateCommandForTest(t, configPath, "show")
	if run.err != "" {
		t.Fatalf("show returned an error: %s", run.err)
	}
	for name, stream := range map[string]string{"stdout": run.stdout, "stderr": run.stderr} {
		if strings.Contains(stream, config.InteractionEnvFileMessage) {
			t.Errorf("dva show %s carries the deprecation:\n%s", name, stream)
		}
	}
}
