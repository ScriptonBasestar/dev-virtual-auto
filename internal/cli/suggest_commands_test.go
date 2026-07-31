package cli

import (
	"slices"
	"testing"
)

// After TASK-108 the only suggestion block dva prints is cobra's. These tests pin what that block
// contains, since deleting dva's own suggestCommands moved the behaviour into a dependency and a
// dependency's defaults can change under us without a compile error.
//
// They call SuggestionsFor directly rather than running the binary because Execute() ends in
// os.Exit(1) on an unknown command. Two details make the direct call faithful to the CLI path:
//
//   - InitDefaultHelpCmd registers `help` and `completion`, which cobra otherwise adds lazily
//     inside Execute(). Without it the command set under test is smaller than the real one.
//   - SuggestionsFor compares against c.SuggestionsMinimumDistance with no default applied;
//     it is findSuggestions (the runtime caller) that substitutes 2 when the field is <= 0.
//     Setting it here reproduces what the user actually gets.
func suggestionsFor(t *testing.T, input string) []string {
	t.Helper()
	rootCmd.InitDefaultHelpCmd()
	rootCmd.SuggestionsMinimumDistance = 2
	return rootCmd.SuggestionsFor(input)
}

// TestCobraSuggestsWhatDvaUsedToMiss is the reason TASK-108 chose cobra's block over dva's.
// dva's suggestCommands scored `levenshtein("sta", "status") == 3` against a cutoff of 2 and so
// never offered `status`; cobra also matches on prefix and does.
func TestCobraSuggestsWhatDvaUsedToMiss(t *testing.T) {
	got := suggestionsFor(t, "sta")
	if !slices.Contains(got, "status") {
		t.Errorf("suggestions for %q = %q; want it to contain %q — the name dva's own block could not reach", "sta", got, "status")
	}
	for _, want := range []string{"stack", "stop"} {
		if !slices.Contains(got, want) {
			t.Errorf("suggestions for %q = %q; want it to still contain %q, which dva's block did offer", "sta", got, want)
		}
	}
}

// TestSuggestionsAreStable guards the property TASK-107 fixed in dva's now-deleted implementation.
// cobra ranges an ordered slice rather than a map, so this should hold; the test exists because
// nothing else in this repo would notice if that changed.
func TestSuggestionsAreStable(t *testing.T) {
	const runs = 200

	first := suggestionsFor(t, "sta")
	if len(first) < 2 {
		// One suggestion can never be out of order, so a fixture returning fewer than two would
		// satisfy the loop below no matter how cobra iterated.
		t.Fatalf("fixture returns %d suggestions; needs at least 2 to detect an ordering defect: %q", len(first), first)
	}

	for i := range runs {
		if got := suggestionsFor(t, "sta"); !slices.Equal(got, first) {
			t.Fatalf("run %d returned %q; the initial call returned %q", i, got, first)
		}
	}
	t.Logf("suggestions=%q stable across %d runs", first, runs)
}

// TestCobraNeverSuggestsHelp records the one thing TASK-108 gave up, so it reads as a known cost
// rather than as a regression someone discovers later.
//
// cobra's IsAvailableCommand returns false when a command is its parent's helpCommand, and
// SuggestionsFor only considers available commands — so `dva hlep` gets no suggestion at all,
// where dva's deleted block answered `dva help`. If a future cobra changes this, the test fails
// and the comment above the call site in root.go needs updating.
func TestCobraNeverSuggestsHelp(t *testing.T) {
	for _, input := range []string{"hlep", "hepl", "helo"} {
		if got := suggestionsFor(t, input); slices.Contains(got, "help") {
			t.Errorf("suggestions for %q = %q; cobra now offers `help` — TASK-108's recorded cost no longer applies, update root.go's comment", input, got)
		}
	}
	// Control: the same near-miss shape against a non-help command must still suggest, otherwise
	// the assertions above would pass simply because nothing is ever suggested.
	if got := suggestionsFor(t, "versoin"); !slices.Contains(got, "version") {
		t.Fatalf("suggestions for %q = %q; want %q — without this the help assertions prove nothing", "versoin", got, "version")
	}
}

// TestNoSuggestionsForAnUnrelatedInput is the negative control for the whole file.
func TestNoSuggestionsForAnUnrelatedInput(t *testing.T) {
	if got := suggestionsFor(t, "qqqqqqqqqq"); len(got) != 0 {
		t.Errorf("suggestions for an input near nothing = %q, want none", got)
	}
}
