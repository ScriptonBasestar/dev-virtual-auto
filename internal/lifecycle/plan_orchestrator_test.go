package lifecycle

import (
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
