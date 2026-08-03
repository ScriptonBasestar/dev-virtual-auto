package config

import (
	"strings"
	"testing"
)

// TestWarnIgnoredParallelSteps covers the validate-time half of TASK-140.
//
// The provision entry in the fixture is the load-bearing one. `parallel:` is honoured
// there, so a check that keyed off the field alone — rather than off where the field
// appears — would flag it and tell the author to remove a key that works.
func TestWarnIgnoredParallelSteps(t *testing.T) {
	c := &Config{
		Interaction: map[string]*InteractionCommand{
			"par": {
				Steps: []ProvisionItem{
					{Step: "a", Run: "sleep 1", Parallel: true},
					{Step: "b", Run: "sleep 1"}, // fine: asks for nothing it will not get
				},
			},
			"db": {
				Subcommands: map[string]*InteractionCommand{
					"migrate": {
						// Hooks run through the same scheduler-less loop, two levels down.
						Before: []ProvisionItem{{Step: "backup", Run: "true", Parallel: true}},
					},
				},
			},
		},
	}
	c.Provision.Profiles = map[string][]ProvisionItem{
		"default": {{Step: "seed", Run: "true", Parallel: true}}, // honoured — must not warn
	}

	warnings := c.warnIgnoredParallelSteps()

	want := []string{
		`interaction.db.subcommands.migrate.before[0] "backup"`,
		`interaction.par.steps[0] "a"`,
	}
	if len(warnings) != len(want) {
		t.Fatalf("got %d warnings, want %d:\n%s", len(warnings), len(want), strings.Join(warnings, "\n"))
	}
	// In order, which also checks the sort: Interaction is a map, so an unsorted result
	// would reorder between runs and make `validate` output undiffable.
	for i, w := range want {
		if !strings.HasPrefix(warnings[i], w) {
			t.Errorf("warning %d = %q, want prefix %q", i, warnings[i], w)
		}
		if !strings.Contains(warnings[i], IgnoredParallelMessage) {
			t.Errorf("warning %d does not carry the shared wording: %q", i, warnings[i])
		}
	}
}

// TestWarnIgnoredParallelStepsIsSilentWhereTheKeyWorks is the negative control. A config
// using `parallel:` only where it is implemented must validate exactly as it did before
// this check existed — the key is not deprecated, it is scoped.
func TestWarnIgnoredParallelStepsIsSilentWhereTheKeyWorks(t *testing.T) {
	c := &Config{
		Interaction: map[string]*InteractionCommand{
			"build": {Steps: []ProvisionItem{{Step: "compile", Run: "make build"}}},
		},
	}
	c.Provision.Profiles = map[string][]ProvisionItem{
		"default": {
			{Step: "a", Run: "true", Parallel: true},
			{Step: "b", Run: "true", Parallel: true},
		},
	}

	if w := c.warnIgnoredParallelSteps(); len(w) != 0 {
		t.Errorf("config using parallel only where it works produced %d warnings:\n%s", len(w), strings.Join(w, "\n"))
	}
}

// TestValidateWarningsReportsIgnoredParallel pins the registration, not the check.
//
// The two tests above call warnIgnoredParallelSteps directly, so both stay green when the
// line wiring it into ValidateWarnings is deleted — `dva config validate` goes silent and
// nothing fails. Verified by deleting that line: internal/config passed, and only the
// runner test caught the regression. This closes that half.
func TestValidateWarningsReportsIgnoredParallel(t *testing.T) {
	c := &Config{
		Interaction: map[string]*InteractionCommand{
			"par": {Steps: []ProvisionItem{{Step: "a", Run: "true", Parallel: true}}},
		},
	}

	var found bool
	for _, w := range c.ValidateWarnings() {
		if strings.Contains(w, IgnoredParallelMessage) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ValidateWarnings does not surface the ignored-parallel check:\n%s",
			strings.Join(c.ValidateWarnings(), "\n"))
	}
}

// TestIgnoredParallelMessageNamesWhereTheKeyWorks pins the one thing the wording has to
// do. An author who reads "ignored" and nothing else concludes the key is dead and
// deletes it from their provision profiles too, where it is doing real work.
func TestIgnoredParallelMessageNamesWhereTheKeyWorks(t *testing.T) {
	if !strings.Contains(IgnoredParallelMessage, "provision") {
		t.Errorf("IgnoredParallelMessage = %q, which does not name the path that honours the key", IgnoredParallelMessage)
	}
	if !strings.Contains(IgnoredParallelMessage, "sequentially") {
		t.Errorf("IgnoredParallelMessage = %q, which does not say what happens instead", IgnoredParallelMessage)
	}
}
