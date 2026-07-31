package cli

import (
	"slices"
	"testing"
)

// TestSuggestCommandsIsStable calls suggestCommands repeatedly in one process. Go randomizes map
// iteration per range statement rather than per process, so an unsorted implementation varies
// between calls here — no -count flag needed.
//
// Measured against the binary before the fix: `dva sta` printed 16 different orderings of the same
// four names across 30 runs. TASK-107.
func TestSuggestCommandsIsStable(t *testing.T) {
	const runs = 200

	first := suggestCommands("sta")
	if len(first) < 2 {
		// One suggestion can never be out of order, so a fixture that returns fewer than two
		// would make every assertion below pass no matter how the function iterates.
		t.Fatalf("fixture returns %d suggestions; needs at least 2 to detect an ordering defect: %q", len(first), first)
	}

	for i := range runs {
		got := suggestCommands("sta")
		if !slices.Equal(got, first) {
			t.Fatalf("run %d returned %q; the initial call returned %q", i, got, first)
		}
	}
	t.Logf("suggestions=%q stable across %d runs", first, runs)
}

// TestSuggestCommandsKeepsTheWholeSet guards the half sorting could quietly break: the fix must
// reorder the results, not filter them.
func TestSuggestCommandsKeepsTheWholeSet(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  []string
	}{
		// Every name within edit distance 2 of the input, as the pre-fix implementation also
		// returned them — verified against `dva sta` output before the change.
		{"sta", []string{"ktl", "ssh", "stack", "stop"}},
		{"hlep", []string{"help"}},
		{"versoin", []string{"version"}},
	} {
		got := suggestCommands(tc.input)
		if !slices.Equal(got, tc.want) {
			t.Errorf("suggestCommands(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestSuggestCommandsReturnsNothingForAnUnrelatedInput is the negative control: without it, the
// assertions above are also satisfied by an implementation that returns every reserved command.
func TestSuggestCommandsReturnsNothingForAnUnrelatedInput(t *testing.T) {
	if got := suggestCommands("qqqqqqqqqq"); len(got) != 0 {
		t.Errorf("suggestCommands returned %q for an input near nothing", got)
	}
}
