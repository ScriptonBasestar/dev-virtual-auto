package lifecycle

import (
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestNewPlanOrchestratorMaterializesResolvedEntries(t *testing.T) {
	composeEntry := &config.LifecycleEntry{Name: "compose"}
	nativeEntry := &config.LifecycleEntry{Name: "api"}
	plan := &ExecutionPlan{Entries: []ResolvedEntry{
		{
			Name: "compose", StackEntry: composeEntry, Runner: "compose", Order: 10,
			RunnerConfig: &config.ComposePluginConfig{Files: []string{"compose.yaml"}},
			Services:     []string{"postgres", "redis"},
		},
		{
			Name: "api", StackEntry: nativeEntry, Runner: "native", Order: 20,
			RunnerConfig: &config.NativeRunnerConfig{Dir: "apps/api", Run: "go run ./cmd/api"},
		},
	}}

	orch, err := NewPlanOrchestrator(&config.Config{}, config.NewEnvironment(nil, ".", "."), plan)
	if err != nil {
		t.Fatalf("NewPlanOrchestrator failed: %v", err)
	}
	if len(orch.entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(orch.entries))
	}
	if orch.entries[0].DetectPlugin() != "compose" || orch.entries[0].Order != 10 {
		t.Fatalf("compose entry not materialized: %+v", orch.entries[0])
	}
	if got := orch.composeServices["compose"]; len(got) != 2 || got[0] != "postgres" || got[1] != "redis" {
		t.Fatalf("compose services = %v", got)
	}
	if orch.entries[1].DetectPlugin() != "process" || orch.entries[1].Process.Command != "go run ./cmd/api" {
		t.Fatalf("native entry not materialized: %+v", orch.entries[1])
	}
}

// TestPlanEntryRunnerHonoredOverDefault locks TASK-039 Probe 1:
// plan entries[].runner must win over stack default_runner at execution materialization.
func TestPlanEntryRunnerHonoredOverDefault(t *testing.T) {
	stackEntry := &config.LifecycleEntry{
		Name:          "s1",
		DefaultRunner: "script",
		Runners: map[string]any{
			"script":  &config.ScriptPluginConfig{Up: "echo RAN_SCRIPT_RUNNER"},
			"process": &config.ProcessPluginConfig{Command: "echo RAN_PROCESS_RUNNER"},
		},
	}
	cfg := &config.Config{
		Stack: map[string]*config.LifecycleEntry{"s1": stackEntry},
		Plans: map[string]*config.PlanConfig{
			"p1": {Entries: []config.PlanEntry{{Name: "s1", Runner: "process", Order: 10}}},
		},
	}

	plan, err := ResolvePlan(cfg, "p1", nil)
	if err != nil {
		t.Fatalf("ResolvePlan: %v", err)
	}
	if plan.Entries[0].Runner != "process" {
		t.Fatalf("resolved runner = %q, want process", plan.Entries[0].Runner)
	}

	orch, err := NewPlanOrchestrator(cfg, config.NewEnvironment(nil, ".", "."), plan)
	if err != nil {
		t.Fatalf("NewPlanOrchestrator: %v", err)
	}
	if got := orch.entries[0].DetectPlugin(); got != "process" {
		t.Fatalf("materialized plugin = %q, want process (not script default)", got)
	}
	if orch.entries[0].Process == nil || orch.entries[0].Process.Command != "echo RAN_PROCESS_RUNNER" {
		t.Fatalf("process config not materialized: %+v", orch.entries[0].Process)
	}
	if orch.entries[0].Script != nil {
		t.Fatalf("script config must not be present when plan chose process")
	}
}

// TestPlanEntryRunnerWithoutDefault locks TASK-039 Probe 3:
// plan runner alone is enough when stack has no default_runner.
func TestPlanEntryRunnerWithoutDefault(t *testing.T) {
	stackEntry := &config.LifecycleEntry{
		Name: "s1",
		Runners: map[string]any{
			"script":  &config.ScriptPluginConfig{Up: "echo RAN_SCRIPT_RUNNER"},
			"process": &config.ProcessPluginConfig{Command: "echo RAN_PROCESS_RUNNER"},
		},
	}
	cfg := &config.Config{
		Stack: map[string]*config.LifecycleEntry{"s1": stackEntry},
		Plans: map[string]*config.PlanConfig{
			"p1": {Entries: []config.PlanEntry{{Name: "s1", Runner: "script", Order: 10}}},
		},
	}

	plan, err := ResolvePlan(cfg, "p1", nil)
	if err != nil {
		t.Fatalf("ResolvePlan: %v", err)
	}
	orch, err := NewPlanOrchestrator(cfg, config.NewEnvironment(nil, ".", "."), plan)
	if err != nil {
		t.Fatalf("NewPlanOrchestrator: %v", err)
	}
	if got := orch.entries[0].DetectPlugin(); got != "script" {
		t.Fatalf("materialized plugin = %q, want script", got)
	}
}

// TestPlanEntryUndeclaredRunnerRejected locks TASK-039 Probe 2 control.
func TestPlanEntryUndeclaredRunnerRejected(t *testing.T) {
	cfg := &config.Config{
		Stack: map[string]*config.LifecycleEntry{
			"s1": {
				Name:          "s1",
				DefaultRunner: "script",
				Runners:       map[string]any{"script": &config.ScriptPluginConfig{Up: "echo ok"}},
			},
		},
		Plans: map[string]*config.PlanConfig{
			"p1": {Entries: []config.PlanEntry{{Name: "s1", Runner: "helm", Order: 10}}},
		},
	}
	_, err := ResolvePlan(cfg, "p1", nil)
	if err == nil {
		t.Fatal("ResolvePlan expected error for undeclared runner helm")
	}
	if !strings.Contains(err.Error(), "not declared") {
		t.Fatalf("error = %v, want not declared", err)
	}
}
