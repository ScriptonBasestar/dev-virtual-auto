package lifecycle

import (
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// nativeEnvConfig builds a one-entry plan whose runner is native, so the tests below
// differ only in which layer declares the key under test.
func nativeEnvConfig(nativeEnv, stackVars, planEntryVars map[string]string) *config.Config {
	return &config.Config{
		Stack: map[string]*config.LifecycleEntry{
			"api": {
				Name:          "api",
				DefaultRunner: "native",
				Vars:          stackVars,
				Runners: map[string]any{
					"native": &config.NativeRunnerConfig{Run: "go run ./cmd/api", Env: nativeEnv},
				},
			},
		},
		Plans: map[string]*config.PlanConfig{
			"local-dev": {
				Entries: []config.PlanEntry{{Name: "api", Runner: "native", Vars: planEntryVars}},
			},
		},
	}
}

func resolveNativeEntry(t *testing.T, cfg *config.Config) ResolvedEntry {
	t.Helper()
	plan, err := ResolvePlan(cfg, "local-dev", nil)
	if err != nil {
		t.Fatalf("ResolvePlan: %v", err)
	}
	if len(plan.Entries) != 1 {
		t.Fatalf("entries: got %d, want 1", len(plan.Entries))
	}
	return plan.Entries[0]
}

// runners.native.env was decoded and then read by nothing: the native runner is desugared
// to the process plugin, whose config has no Env field. Vars is the channel that actually
// reaches the command, so this asserts the key arrives there.
func TestResolvePlan_NativeRunnerEnvReachesVars(t *testing.T) {
	entry := resolveNativeEntry(t, nativeEnvConfig(map[string]string{"API_TOKEN": "from-runner"}, nil, nil))

	if got := entry.Vars["API_TOKEN"]; got != "from-runner" {
		t.Errorf("Vars[API_TOKEN] = %q, want %q — runners.native.env did not reach the entry", got, "from-runner")
	}
}

// Both are declarations on the same stack entry, but runner env applies only when that
// runner is the one chosen, so the narrower declaration wins.
func TestResolvePlan_NativeRunnerEnvBeatsStackVars(t *testing.T) {
	entry := resolveNativeEntry(t, nativeEnvConfig(
		map[string]string{"PORT": "9000"},
		map[string]string{"PORT": "8080"},
		nil,
	))

	if got := entry.Vars["PORT"]; got != "9000" {
		t.Errorf("Vars[PORT] = %q, want %q — runner-scoped env must beat entry-wide vars", got, "9000")
	}
}

// plans.entries[].vars is this run's choice for this entry, so it overrules the runner
// declaration. This is the layer directly above native env in the merge chain.
//
// Note the layer deliberately checked here is the plan *entry*, not plan-level `vars:`.
// Plan-level vars land in EnvVars, which the chain lays down before every stack-entry
// declaration — measured, a stack `vars:` key beats both plan vars and `--var` with no
// native env involved at all. That ordering predates this fix and is not restated here,
// so a later change to it fails its own test rather than this one.
func TestResolvePlan_PlanEntryVarsBeatNativeRunnerEnv(t *testing.T) {
	entry := resolveNativeEntry(t, nativeEnvConfig(
		map[string]string{"LOG_LEVEL": "info"},
		nil,
		map[string]string{"LOG_LEVEL": "debug"},
	))

	if got := entry.Vars["LOG_LEVEL"]; got != "debug" {
		t.Errorf("Vars[LOG_LEVEL] = %q, want %q — plan entry vars must overrule runner env", got, "debug")
	}
}
