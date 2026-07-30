package cli

import (
	"strings"
	"testing"
)

// indexOfLine returns the offset of the help line introducing command name, or
// -1. Help lines are "  <name><padding> <hint>", so the trailing space keeps
// "app" from matching "applications" in prose elsewhere in the template.
func indexOfLine(t *testing.T, help, name string) int {
	t.Helper()
	return strings.Index(help, "\n  "+name+" ")
}

// The Lifecycle help block is where a user chooses between `dva up`, `dva stack`
// and `dva app`. Guardrail 33 (agent-mesh-flows/shared/library/shared-guardrails.md)
// makes `dva app` and `--mode` migration-only, and USAGE.md records that
// `dva stack up` is no longer the recommended model — so neither may be listed
// under "Recommended Flow", which is what the block claimed before.
func TestLifecycleHelpSeparatesRecommendedFromDirectAccess(t *testing.T) {
	help := rootCmd.UsageString()

	recommended := strings.Index(help, "Recommended Flow")
	direct := strings.Index(help, "Direct Access")
	other := strings.Index(help, "Other Commands")

	for _, h := range []struct {
		name string
		at   int
	}{{"Recommended Flow", recommended}, {"Direct Access", direct}, {"Other Commands", other}} {
		if h.at < 0 {
			t.Fatalf("lifecycle help is missing the %q block:\n%s", h.name, help)
		}
	}
	if recommended >= direct || direct >= other {
		t.Fatalf("blocks out of order: Recommended=%d Direct=%d Other=%d\n%s",
			recommended, direct, other, help)
	}

	// up/down are the current model's entry points and belong to the first block.
	for _, name := range []string{"up", "down"} {
		at := indexOfLine(t, help, name)
		if at < 0 {
			t.Fatalf("%q is not listed in the lifecycle help:\n%s", name, help)
		}
		if at < recommended || at > direct {
			t.Errorf("%q at %d is not under Recommended Flow (%d..%d)", name, at, recommended, direct)
		}
	}

	// stack/app must sit under Direct Access, not Recommended Flow.
	for _, name := range []string{"stack", "app"} {
		at := indexOfLine(t, help, name)
		if at < 0 {
			t.Fatalf("%q is not listed in the lifecycle help:\n%s", name, help)
		}
		if at < direct || at > other {
			t.Errorf("%q at %d is not under Direct Access (%d..%d); a user reading this "+
				"picks it as the recommended entry point", name, at, direct, other)
		}
	}
}

// `dva app` reads the legacy applications section. The marker is the only thing
// on the help screen that says so at the moment the user picks a command.
func TestAppIsMarkedLegacyInHelp(t *testing.T) {
	help := rootCmd.UsageString()

	at := indexOfLine(t, help, "app")
	if at < 0 {
		t.Fatalf("app is not listed in the lifecycle help:\n%s", help)
	}
	line := help[at+1:]
	if end := strings.IndexByte(line, '\n'); end >= 0 {
		line = line[:end]
	}
	if !strings.Contains(line, "[legacy]") {
		t.Errorf("app help line = %q, want a [legacy] marker", line)
	}
}

// The Recommended Flow block is the first place a user meets the lifecycle, and
// plans are what `dva up` actually takes. Before this, both entries described
// themselves with the legacy nouns only and the word never appeared.
func TestRecommendedFlowNamesPlans(t *testing.T) {
	help := rootCmd.UsageString()

	recommended := strings.Index(help, "Recommended Flow")
	direct := strings.Index(help, "Direct Access")
	if recommended < 0 || direct < 0 || direct < recommended {
		t.Fatalf("lifecycle help blocks not found in order:\n%s", help)
	}

	block := help[recommended:direct]
	if !strings.Contains(block, "plan") {
		t.Errorf("Recommended Flow block never mentions a plan:\n%s", block)
	}
}
