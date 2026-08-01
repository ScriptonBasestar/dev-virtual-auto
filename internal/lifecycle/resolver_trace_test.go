package lifecycle

import (
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// The resolution trace is user-visible output: 'dva up <plan> --dry-run' prints every step
// verbatim (printPlanResolution, internal/cli/plan_lifecycle.go). Before that it was built
// and never read by anything, which is how the line claiming env_file was "skipped (TODO)"
// survived — no test and no reader could contradict it. These tests are the contract that
// keeps a step from claiming something the resolver does not do.

// traceStep returns the single step whose prefix matches, failing when a layer is missing
// or duplicated. Matching on the layer prefix rather than the whole line keeps the tests
// pinned to which layers are reported and in what order, without freezing the prose.
func traceStep(t *testing.T, plan *ExecutionPlan, prefix string) string {
	t.Helper()
	var found []string
	for _, step := range plan.ResolutionTrace {
		if strings.HasPrefix(step, prefix) {
			found = append(found, step)
		}
	}
	if len(found) != 1 {
		t.Fatalf("want exactly 1 trace step with prefix %q, got %d in:\n  %s",
			prefix, len(found), strings.Join(plan.ResolutionTrace, "\n  "))
	}
	return found[0]
}

func traceIndex(t *testing.T, plan *ExecutionPlan, prefix string) int {
	t.Helper()
	for i, step := range plan.ResolutionTrace {
		if strings.HasPrefix(step, prefix) {
			return i
		}
	}
	t.Fatalf("no trace step with prefix %q in:\n  %s", prefix, strings.Join(plan.ResolutionTrace, "\n  "))
	return -1
}

func fullTraceConfig() *config.Config {
	return &config.Config{
		EnvFile:     []any{".env", ".env.local"},
		Environment: map[string]string{"TOP_LEVEL": "yes"},
		Vars:        map[string]string{"G1": "a", "G2": "b"},
		Stack: map[string]*config.LifecycleEntry{
			"api": {
				Name:          "api",
				Plugin:        "process",
				DefaultRunner: "native",
				Runners: map[string]any{
					"native": &config.NativeRunnerConfig{Run: "go run ./cmd/api"},
				},
				Process: &config.ProcessPluginConfig{Command: "go run ./cmd/api"},
			},
		},
		Environments: map[string]config.EnvironmentProfile{
			"dev": {Environment: map[string]string{"APP_ENV": "dev"}},
		},
		Sites: map[string]*config.SiteConfig{
			"local": {Vars: map[string]string{"DVA_SITE": "local"}},
		},
		Plans: map[string]*config.PlanConfig{
			"full": {
				Environment: "dev",
				Site:        "local",
				Vars:        map[string]string{"LOG_LEVEL": "debug"},
				Entries:     []config.PlanEntry{{Name: "api", Runner: "native", Order: 10}},
			},
		},
	}
}

// TestResolutionTraceReportsEveryPrecedenceLayer is the reason the trace exists: a user
// reading it should be able to account for every layer docs/31 §4-3 documents, including
// the two applied before ResolvePlan runs and the OS environment applied after it.
func TestResolutionTraceReportsEveryPrecedenceLayer(t *testing.T) {
	plan, err := ResolvePlan(fullTraceConfig(), "full", map[string]string{"EXTRA": "1"})
	if err != nil {
		t.Fatalf("ResolvePlan failed: %v", err)
	}

	for _, prefix := range []string{
		"vars: env_file",
		"vars: environment:",
		"vars: global vars",
		`vars: environments."dev"`,
		`vars: sites."local".vars`,
		`vars: plans."full".vars`,
		"vars: cli --var",
		"vars: OS environment",
	} {
		traceStep(t, plan, prefix)
	}
}

// TestResolutionTraceOrderMatchesMergeOrder guards the property that makes the trace worth
// printing: the steps are in the order the layers are applied, so reading top to bottom is
// reading lowest to highest precedence. A step recorded after its merge instead of before
// (or a merge moved without its step) would break this without breaking any other test.
func TestResolutionTraceOrderMatchesMergeOrder(t *testing.T) {
	plan, err := ResolvePlan(fullTraceConfig(), "full", map[string]string{"EXTRA": "1"})
	if err != nil {
		t.Fatalf("ResolvePlan failed: %v", err)
	}

	ordered := []string{
		"vars: env_file",
		"vars: environment:",
		"vars: global vars",
		`vars: environments."dev"`,
		`vars: sites."local".vars`,
		`vars: plans."full".vars`,
		"vars: cli --var",
		"vars: OS environment",
	}
	for i := 1; i < len(ordered); i++ {
		prev, cur := traceIndex(t, plan, ordered[i-1]), traceIndex(t, plan, ordered[i])
		if prev >= cur {
			t.Errorf("%q must be traced before %q, got positions %d and %d", ordered[i-1], ordered[i], prev, cur)
		}
	}
}

// TestResolutionTraceDoesNotClaimEnvFileIsSkipped is the regression this work exists for.
// The trace said "env_file merge skipped (TODO)" while env_file was in fact applied by
// loadEnv one stage earlier, and nothing caught it because nothing printed or asserted the
// trace. The claim must never come back.
func TestResolutionTraceDoesNotClaimEnvFileIsSkipped(t *testing.T) {
	plan, err := ResolvePlan(fullTraceConfig(), "full", nil)
	if err != nil {
		t.Fatalf("ResolvePlan failed: %v", err)
	}

	step := traceStep(t, plan, "vars: env_file")
	for _, forbidden := range []string{"skip", "TODO"} {
		if strings.Contains(strings.ToLower(step), strings.ToLower(forbidden)) {
			t.Errorf("env_file step must not claim %q: %s", forbidden, step)
		}
	}
	for _, want := range []string{".env", ".env.local"} {
		if !strings.Contains(step, want) {
			t.Errorf("env_file step should name the declared file %q: %s", want, step)
		}
	}
}

// TestResolutionTraceNamesDeclaredFilesNotAppliedOnes pins the wording chosen because
// config.LoadEnvFile skips a missing optional file without error. ResolvePlan does no file
// I/O, so it can only report what the config declares — saying the files were applied would
// be the same kind of unverifiable claim the "skipped (TODO)" line was.
func TestResolutionTraceNamesDeclaredFilesNotAppliedOnes(t *testing.T) {
	plan, err := ResolvePlan(fullTraceConfig(), "full", nil)
	if err != nil {
		t.Fatalf("ResolvePlan failed: %v", err)
	}

	step := traceStep(t, plan, "vars: env_file")
	if !strings.Contains(step, "declared") {
		t.Errorf("env_file step must present the list as declared, not as read: %s", step)
	}
}

// TestResolutionTraceReportsAbsentLayers covers the decision that an empty layer is still
// reported. A config that declares none of the optional layers is the common case, and the
// layer that contributed nothing is exactly the answer to "why is my variable unset".
func TestResolutionTraceReportsAbsentLayers(t *testing.T) {
	cfg := &config.Config{
		Stack: map[string]*config.LifecycleEntry{
			"api": {
				Name:          "api",
				Plugin:        "process",
				DefaultRunner: "native",
				Runners: map[string]any{
					"native": &config.NativeRunnerConfig{Run: "go run ./cmd/api"},
				},
				Process: &config.ProcessPluginConfig{Command: "go run ./cmd/api"},
			},
		},
		Plans: map[string]*config.PlanConfig{
			"bare": {Entries: []config.PlanEntry{{Name: "api", Runner: "native", Order: 10}}},
		},
	}

	plan, err := ResolvePlan(cfg, "bare", nil)
	if err != nil {
		t.Fatalf("ResolvePlan failed: %v", err)
	}

	cases := map[string]string{
		"vars: env_file":          "not declared",
		"vars: environment:":      "not declared",
		"vars: global vars":       "not declared",
		"vars: environments":      "none selected by this plan",
		"vars: sites":             "none selected by this plan",
		`vars: plans."bare".vars`: "not declared",
		"vars: cli --var":         "none passed",
	}
	for prefix, want := range cases {
		if step := traceStep(t, plan, prefix); !strings.Contains(step, want) {
			t.Errorf("step %q should report %q, got: %s", prefix, want, step)
		}
	}
}

// TestResolutionTraceDistinguishesFlagFromSection checks that the empty phrasing is chosen
// per layer. A --var flag is passed or not passed; a config section is declared or not.
// Reporting "not declared" for a command-line flag would be wrong in the same small way the
// env_file line was wrong.
func TestResolutionTraceDistinguishesFlagFromSection(t *testing.T) {
	plan, err := ResolvePlan(fullTraceConfig(), "full", nil)
	if err != nil {
		t.Fatalf("ResolvePlan failed: %v", err)
	}

	if step := traceStep(t, plan, "vars: cli --var"); strings.Contains(step, "declared") {
		t.Errorf("a flag is passed or not, never declared: %s", step)
	}
}

// TestResolutionTraceCountsMergedKeys checks the counts the trace reports, since a wrong
// count is a wrong answer to "did this layer contribute anything".
func TestResolutionTraceCountsMergedKeys(t *testing.T) {
	plan, err := ResolvePlan(fullTraceConfig(), "full", map[string]string{"EXTRA": "1"})
	if err != nil {
		t.Fatalf("ResolvePlan failed: %v", err)
	}

	cases := map[string]string{
		"vars: global vars":        "2 keys",
		`vars: environments."dev"`: "1 key",
		`vars: sites."local".vars`: "1 key",
		`vars: plans."full".vars`:  "1 key",
		"vars: cli --var":          "1 key",
	}
	for prefix, want := range cases {
		if step := traceStep(t, plan, prefix); !strings.Contains(step, want) {
			t.Errorf("step %q should report %q, got: %s", prefix, want, step)
		}
	}
}
