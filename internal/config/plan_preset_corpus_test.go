package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func readPlanFlowFile(t *testing.T, rel string) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")
	path := filepath.Join(root, rel)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", repoPath(path), err)
	}
	return string(content)
}

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
	top := readPlanFlowFile(t, "agent-mesh-flows/dva-improve-guided.yaml")
	if strings.Contains(top, "check_existing_analysis") {
		t.Error("guided flow reuses analysis by file existence and can ignore changed bindings")
	}
	if strings.Count(top, "capability_bindings") < 3 {
		t.Error("guided flow does not declare and forward capability_bindings")
	}

	analyze := readPlanFlowFile(t, "agent-mesh-flows/dva-improve-guided/00-analyze.yaml")
	for _, required := range []string{"recommended_plans", "capability_bindings", "accepted|conflict|unverified"} {
		if !strings.Contains(analyze, required) {
			t.Errorf("analysis flow is missing %q", required)
		}
	}

	configure := readPlanFlowFile(t, "agent-mesh-flows/dva-improve-guided/30-configure.yaml")
	if !strings.Contains(configure, "### capability bindings:") {
		t.Error("configure flow does not materialize approved bindings")
	}

	execute := readPlanFlowFile(t, "agent-mesh-flows/dva-improve-guided/40-execute.yaml")
	for _, required := range []string{"if OUTPUT=$(dva up \"$PLAN\" 2>&1)", "exit \"$STATUS\""} {
		if !strings.Contains(execute, required) {
			t.Errorf("execute flow does not preserve named lifecycle failure: missing %q", required)
		}
	}

	discover := readPlanFlowFile(t, "agent-mesh-flows/dva-discover.yaml")
	if strings.Count(discover, "capability_bindings") < 3 {
		t.Error("discover flow does not declare and forward capability_bindings")
	}

	automatic := readPlanFlowFile(t, "agent-mesh-flows/dva-improve.yaml")
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

func TestGuidedFlowPreservesReviewedProposal(t *testing.T) {
	top := readPlanFlowFile(t, "agent-mesh-flows/dva-improve-guided.yaml")
	verify := readPlanFlowFile(t, "agent-mesh-flows/dva-improve-guided/10-verify.yaml")
	for _, contract := range []string{"명시적 batch(-y)", "-y는 caller의 명시적 auto-approval"} {
		if !strings.Contains(top, contract) {
			t.Errorf("guided flow does not state Agent Mesh batch approval semantics %q", contract)
		}
	}
	for _, required := range []string{
		"auto_decide: [present_proposal]",
		"json_mode: true",
		`"selected_plan"`,
		"{{present_proposal.output | b64encode}}",
	} {
		if !strings.Contains(verify, required) {
			t.Errorf("guided approval handoff is missing %q", required)
		}
	}
	for _, stale := range []string{
		"confirm_before: [present_proposal]",
		`cp "$REPORT_DIR/00-analysis-report.json" "$REPORT_DIR/10-proposal-approved.json"`,
	} {
		if strings.Contains(verify, stale) {
			t.Errorf("guided approval handoff still contains stale behavior %q", stale)
		}
	}
}

func TestGuidedFlowResolvesAndValidatesApprovedPlan(t *testing.T) {
	execute := readPlanFlowFile(t, "agent-mesh-flows/dva-improve-guided/40-execute.yaml")
	for _, required := range []string{
		`OVERRIDE=$(printf '%s' '{{param.plan | b64encode}}' | base64 -d)`,
		`PROPOSAL="tmp/improve-guided/10-proposal-approved.json"`,
		`PLAN=$(jq -r '.selected_plan // empty' "$PROPOSAL")`,
		`(.plans | has($plan))`,
		`PLAN=$(printf '%s' '{{resolve_plan.plan | b64encode}}' | base64 -d)`,
		`dva up "$PLAN"`,
	} {
		if !strings.Contains(execute, required) {
			t.Errorf("guided execution handoff is missing %q", required)
		}
	}
	startAt := strings.Index(execute, "  - id: start_services")
	if startAt < 0 {
		t.Fatal("guided execution has no start_services mutation step")
	}
	start := execute[startAt:]
	upAt := strings.Index(start, `dva up "$PLAN"`)
	for _, gate := range []string{
		`dva config validate --strict`,
		`(.plans | has($plan))`,
		`lifecycle execution is blocked`,
	} {
		gateAt := strings.Index(start, gate)
		if gateAt < 0 || gateAt > upAt {
			t.Errorf("start_services does not enforce %q before dva up", gate)
		}
	}
	for _, swallowed := range []string{
		"dva config validate 2>&1 || true",
		"dva config validate --strict 2>&1 || true",
	} {
		if strings.Contains(execute, swallowed) {
			t.Errorf("guided execution still swallows validation failure %q", swallowed)
		}
	}
}

func TestAutomaticFlowRequiresExplicitDiscoveryReport(t *testing.T) {
	automatic := readPlanFlowFile(t, "agent-mesh-flows/dva-improve.yaml")
	parameter := `  - name: discovery_report
    description: "Explicit path to a precomputed discovery report JSON; empty performs a fresh scan"
    default: ""`
	if !strings.Contains(automatic, parameter) {
		t.Error("automatic flow does not require an explicitly supplied discovery report")
	}
	if strings.Contains(automatic, `default: "tmp/improve-guided/00-analysis-report.json"`) {
		t.Error("automatic flow still defaults to a potentially stale discovery report")
	}
	if !strings.Contains(automatic, `{{param.discovery_report | b64encode}}`) {
		t.Error("automatic flow does not safely interpolate the explicit report path")
	}
	for _, required := range []string{
		"explicit discovery report does not exist",
		"explicit discovery report is not one JSON object",
		`length == 1 and (.[0] | type) == "object"`,
	} {
		if !strings.Contains(automatic, required) {
			t.Errorf("automatic flow does not fail closed for explicit report errors: missing %q", required)
		}
	}
}
