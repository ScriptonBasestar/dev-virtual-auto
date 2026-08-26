package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestPlanPresetPolicyShipsInPromptCorpus guards the decision table consumed by
// both improve flows. DVA does not enforce capability semantics in the schema,
// so losing this prompt contract would silently return generation to ad-hoc choices.
func TestPlanPresetPolicyShipsInPromptCorpus(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")
	paths := []string{
		filepath.Join(root, "agent-mesh-flows", "shared", "library", "naming-presets.md"),
		filepath.Join(root, "internal", "cli", "library_reference.txt"),
	}
	required := []string{
		"## Capability Closure",
		"## Deterministic Plan Matrix",
		"default_plan: local-infra",
		"`dva up *`",
		"`capability_bindings`",
		"does not compose or inherit plans",
	}

	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", repoPath(path), err)
		}
		for _, fragment := range required {
			if !strings.Contains(string(content), fragment) {
				t.Errorf("%s is missing plan-policy contract %q", repoPath(path), fragment)
			}
		}
	}
}

func TestGuidedFlowUsesPlanAndCapabilityContract(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")
	read := func(rel string) string {
		t.Helper()
		path := filepath.Join(root, rel)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", repoPath(path), err)
		}
		return string(content)
	}

	top := read("agent-mesh-flows/dva-improve-guided.yaml")
	if strings.Contains(top, "check_existing_analysis") {
		t.Error("guided flow reuses analysis by file existence and can ignore changed bindings")
	}
	if strings.Count(top, "capability_bindings") < 3 {
		t.Error("guided flow does not declare and forward capability_bindings")
	}

	analyze := read("agent-mesh-flows/dva-improve-guided/00-analyze.yaml")
	for _, required := range []string{"recommended_plans", "capability_bindings", "accepted|conflict|unverified"} {
		if !strings.Contains(analyze, required) {
			t.Errorf("analysis flow is missing %q", required)
		}
	}

	configure := read("agent-mesh-flows/dva-improve-guided/30-configure.yaml")
	if !strings.Contains(configure, "### capability bindings:") {
		t.Error("configure flow does not materialize approved bindings")
	}

	execute := read("agent-mesh-flows/dva-improve-guided/40-execute.yaml")
	for _, required := range []string{"if OUTPUT=$(dva up \"$PLAN\" 2>&1)", "exit \"$STATUS\""} {
		if !strings.Contains(execute, required) {
			t.Errorf("execute flow does not preserve named lifecycle failure: missing %q", required)
		}
	}

	discover := read("agent-mesh-flows/dva-discover.yaml")
	if strings.Count(discover, "capability_bindings") < 3 {
		t.Error("discover flow does not declare and forward capability_bindings")
	}

	automatic := read("agent-mesh-flows/dva-improve.yaml")
	if strings.Count(automatic, "capability_bindings") < 3 {
		t.Error("automatic improve flow does not accept and consume capability_bindings")
	}

	corpus := strings.Join([]string{top, analyze, configure, execute, discover}, "\n")
	for _, stale := range []string{"recommended_modes", "--mode", "param.mode"} {
		if strings.Contains(corpus, stale) {
			t.Errorf("guided flow corpus still contains migration-only contract %q", stale)
		}
	}
}
