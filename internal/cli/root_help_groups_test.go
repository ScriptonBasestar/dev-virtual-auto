package cli

import (
	"strings"
	"testing"
)

// indexOfLine returns the offset of the help line introducing command name, or
// -1. Help lines are "  <name><padding> <hint>", so the two-space prefix and the
// trailing space together keep a short name from matching a longer one, or from
// matching the same word used in the prose of some other command's hint.
func indexOfLine(t *testing.T, help, name string) int {
	t.Helper()
	return strings.Index(help, "\n  "+name+" ")
}

// The Lifecycle block used to be a three-way choice — Recommended Flow / Direct Access /
// Other Commands — because `dva up` and `dva stack up` and `dva app up` were three ways to
// start the same thing, and the middle block existed to steer people off the backend-shaped
// ones without hiding them.
//
// There is no backend-shaped way in any more: every lifecycle verb takes the same plan name.
// So the middle block is gone and this asserts the two that remain. The ordering claim is the
// load-bearing part — a lifecycle verb that drifts above "Recommended Flow" is presented as an
// entry point, which is the mistake the original test was written to catch, and it is still
// available.
func TestLifecycleHelpSeparatesRecommendedFromOther(t *testing.T) {
	help := rootCmd.UsageString()

	recommended := strings.Index(help, "Recommended Flow")
	other := strings.Index(help, "Other Commands")

	for _, h := range []struct {
		name string
		at   int
	}{{"Recommended Flow", recommended}, {"Other Commands", other}} {
		if h.at < 0 {
			t.Fatalf("lifecycle help is missing the %q block:\n%s", h.name, help)
		}
	}
	if recommended >= other {
		t.Fatalf("blocks out of order: Recommended=%d Other=%d\n%s", recommended, other, help)
	}
	if strings.Contains(help, "Direct Access") {
		t.Errorf("lifecycle help still has a Direct Access block, but the commands it "+
			"held are gone:\n%s", help)
	}

	// up/down are the model's entry points and belong to the first block.
	for _, name := range []string{"up", "down"} {
		at := indexOfLine(t, help, name)
		if at < 0 {
			t.Fatalf("%q is not listed in the lifecycle help:\n%s", name, help)
		}
		if at < recommended || at > other {
			t.Errorf("%q at %d is not under Recommended Flow (%d..%d)", name, at, recommended, other)
		}
	}

	// The rest are lifecycle verbs a user reaches for once they know what they want, and
	// listing any of them as an entry point is the regression this guards.
	for _, name := range []string{"build", "logs", "restart", "stop"} {
		at := indexOfLine(t, help, name)
		if at < 0 {
			t.Fatalf("%q is not listed in the lifecycle help:\n%s", name, help)
		}
		if at < other {
			t.Errorf("%q at %d is above Other Commands (%d); a user reading this picks it "+
				"as the recommended entry point", name, at, other)
		}
	}
}

// The Recommended Flow block is the first place a user meets the lifecycle, and
// plans are what `dva up` actually takes. Before this, both entries described
// themselves with the legacy nouns only and the word never appeared.
func TestRecommendedFlowNamesPlans(t *testing.T) {
	help := rootCmd.UsageString()

	recommended := strings.Index(help, "Recommended Flow")
	other := strings.Index(help, "Other Commands")
	if recommended < 0 || other < 0 || other < recommended {
		t.Fatalf("lifecycle help blocks not found in order:\n%s", help)
	}

	block := help[recommended:other]
	if !strings.Contains(block, "plan") {
		t.Errorf("Recommended Flow block never mentions a plan:\n%s", block)
	}
}
