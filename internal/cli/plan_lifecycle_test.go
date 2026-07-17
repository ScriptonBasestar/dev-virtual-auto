package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestRequirePlanSelection_MultiplePlansRequireName(t *testing.T) {
	c := &config.Config{Plans: map[string]*config.PlanConfig{
		"full-stack": {},
		"infra":      {},
	}}

	err := requirePlanSelection(c, "up", nil)
	if err == nil {
		t.Fatal("expected multiple plans without a name to fail")
	}
	if got := err.Error(); !strings.Contains(got, "dva up <full-stack|infra>") {
		t.Fatalf("error = %q, want sorted plan hint", got)
	}
}

func TestRequirePlanSelection_AllowsSingleDefaultPlan(t *testing.T) {
	c := &config.Config{Plans: map[string]*config.PlanConfig{"infra": {}}}
	if err := requirePlanSelection(c, "up", nil); err != nil {
		t.Fatalf("single default plan failed: %v", err)
	}
}

func TestRequirePlanSelection_AllowsLegacyOrNamedArgs(t *testing.T) {
	c := &config.Config{Plans: map[string]*config.PlanConfig{
		"full-stack": {},
		"infra":      {},
	}}
	for _, args := range [][]string{{"infra"}, {"--mode", "legacy"}} {
		if err := requirePlanSelection(c, "up", args); err != nil {
			t.Fatalf("args %v failed: %v", args, err)
		}
	}
}

func TestRunPlanRestartDryRunPreservesNativeProcessState(t *testing.T) {
	c := loadTestConfig(t, `version: "0.1.44"
stack:
  api:
    default_runner: native
    runners:
      native:
        run: sleep 999
plans:
  local-dev:
    entries:
      - name: api
        runner: native
`)
	e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())
	pidDir := filepath.Join(c.FileDir(), config.DotDirName, config.PidsDirName)
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		t.Fatal(err)
	}
	pidFile := filepath.Join(pidDir, "api.pid")
	if err := os.WriteFile(pidFile, []byte("invalid-pid"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := runPlanRestart(c, e, "local-dev", []string{"--dry-run"}); err != nil {
		t.Fatalf("plan restart dry-run failed: %v", err)
	}
	if _, err := os.Stat(pidFile); err != nil {
		t.Fatalf("plan restart dry-run removed PID state: %v", err)
	}
}

const planStackConfig = `version: "0.1.0"
stack:
  s1:
    default_runner: script
    runners:
      script:
        up: echo MARKERS1
  s2:
    default_runner: script
    runners:
      script:
        up: echo MARKERS2
plans:
  p1:
    entries:
      - name: s1
`

const noPlanStackConfig = `version: "0.1.0"
stack:
  s1:
    default_runner: script
    runners:
      script:
        up: echo MARKERS1
`

// useConfig writes yml as dva.yml in a temp cwd and clears the cached config so
// the command under test cannot walk up to the repository's own dva.yml.
func useConfig(t *testing.T, yml string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(yml), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Chdir(dir)
	cfg = nil
	t.Cleanup(func() { cfg = nil })
}

func TestUpRejectsUnknownPlanName(t *testing.T) {
	useConfig(t, planStackConfig)

	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	err := upCmd.RunE(upCmd, []string{"p1-typo"})
	if err == nil {
		t.Fatal("'dva up p1-typo' returned nil; an unknown plan name must not silently start the whole stack")
	}
	if !strings.Contains(err.Error(), "p1-typo") {
		t.Errorf("error must name the unknown argument, got: %v", err)
	}
	if !strings.Contains(err.Error(), "p1") {
		t.Errorf("error should list the available plans, got: %v", err)
	}
}

func TestUpAcceptsKnownPlanName(t *testing.T) {
	useConfig(t, planStackConfig)

	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	if err := upCmd.RunE(upCmd, []string{"p1"}); err != nil {
		t.Fatalf("'dva up p1' must resolve the real plan: %v", err)
	}
}

// Bare `dva up` with exactly one plan must still use DefaultPlan (no flags).
func TestUpBareUsesDefaultPlan(t *testing.T) {
	useConfig(t, planStackConfig)

	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	if err := upCmd.RunE(upCmd, []string{}); err != nil {
		t.Fatalf("'dva up' with one plan must use DefaultPlan: %v", err)
	}
}

// Without plans, bare 'dva up' starting the whole stack is the legitimate path
// and must stay reachable — it is what the command advertises.
func TestUpWithoutPlansStartsWholeStack(t *testing.T) {
	useConfig(t, noPlanStackConfig)

	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	if err := upCmd.RunE(upCmd, []string{}); err != nil {
		t.Fatalf("'dva up' with no plans must start the stack: %v", err)
	}
}

// Without plans a positional argument is not a plan name and is not an entry
// filter either ('up [OPTIONS]' advertises neither), so it must not be silently
// dropped and widened into starting every entry. s1 is a REAL entry name: even a
// name that exists is not something 'up' accepts.
func TestUpWithoutPlansRejectsPositionalArg(t *testing.T) {
	useConfig(t, noPlanStackConfig)

	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	for _, name := range []string{"notarealthing", "s1"} {
		err := upCmd.RunE(upCmd, []string{name})
		if err == nil {
			t.Fatalf("'dva up %s' with no plans returned nil; a stray argument must not start the whole stack", name)
		}
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error must name the rejected argument %q, got: %v", name, err)
		}
		// With zero plans there is nothing to list; an empty "Available:" would
		// be a worse message than none.
		if strings.Contains(err.Error(), "Available:") {
			t.Errorf("error must not print an Available: plan list when no plans exist, got: %v", err)
		}
	}
}

// When exactly one plan exists, bare `dva up` uses DefaultPlan. A leading flag
// used to skip that route and silently start the whole stack (TASK-028). Option B
// refuses that fallthrough and requires the plan name to be written out.
func TestUpRejectsFlagsThatSuppressDefaultPlan(t *testing.T) {
	useConfig(t, planStackConfig)

	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	for _, args := range [][]string{
		{"--dev"},
		{"--force"},
		{"--no-wait"},
		{"--docker"},
		{"--var", "FOO=bare"},
	} {
		err := upCmd.RunE(upCmd, args)
		if err == nil {
			t.Fatalf("'dva up %s' with a default plan returned nil; must not silently start the whole stack", strings.Join(args, " "))
		}
		msg := err.Error()
		if !strings.Contains(msg, "p1") {
			t.Errorf("error must name the default plan, got: %v", err)
		}
		if !strings.Contains(msg, "dva up p1") {
			t.Errorf("error must show explicit plan form, got: %v", err)
		}
	}
}

// Flag values after a leading flag are not plan names. The guard still fires on
// the leading flag (default plan suppressed), not on the value token.
func TestUpPlanGuardOnlyInspectsPlanNameSlot(t *testing.T) {
	c := &config.Config{Plans: map[string]*config.PlanConfig{"p1": {}}}
	// FOO=bare alone is not a flag; with a default plan it is an unknown plan name.
	if err := rejectUnknownPlanArg(c, []string{"FOO=bare"}); err == nil {
		t.Fatal("non-flag token must still be treated as a plan-name slot")
	}
}

func TestRejectSuppressedDefaultPlan_LeadingFlag(t *testing.T) {
	c := &config.Config{Plans: map[string]*config.PlanConfig{"p1": {}}}

	err := rejectSuppressedDefaultPlan(c, "up", []string{"--dev"})
	if err == nil {
		t.Fatal("expected error when flags suppress the default plan")
	}
	if got := err.Error(); !strings.Contains(got, "dva up p1 --dev") {
		t.Fatalf("error = %q, want explicit plan hint", got)
	}
}

func TestRejectSuppressedDefaultPlan_BareUpStillAllowed(t *testing.T) {
	c := &config.Config{Plans: map[string]*config.PlanConfig{"p1": {}}}
	if err := rejectSuppressedDefaultPlan(c, "up", nil); err != nil {
		t.Fatalf("bare args must not suppress default plan: %v", err)
	}
}

func TestRejectSuppressedDefaultPlan_MultiplePlansNoDefault(t *testing.T) {
	c := &config.Config{Plans: map[string]*config.PlanConfig{
		"p1": {},
		"p2": {},
	}}
	// No DefaultPlan; multi-plan bare up is requirePlanSelection's job.
	if err := rejectSuppressedDefaultPlan(c, "up", []string{"--dev"}); err != nil {
		t.Fatalf("without a default plan, flag fallthrough is not this guard: %v", err)
	}
}

func TestDetectPlanRoute_BareArgsUsesDefaultPlan(t *testing.T) {
	c := &config.Config{Plans: map[string]*config.PlanConfig{"p1": {}}}
	name, extra, ok := detectPlanRoute(c, nil)
	if !ok || name != "p1" || len(extra) != 0 {
		t.Fatalf("detectPlanRoute(nil) = (%q, %v, %v), want (p1, nil, true)", name, extra, ok)
	}
}

func TestDetectPlanRoute_LeadingFlagDoesNotSelectDefault(t *testing.T) {
	c := &config.Config{Plans: map[string]*config.PlanConfig{"p1": {}}}
	name, extra, ok := detectPlanRoute(c, []string{"--dev"})
	if ok || name != "" || extra != nil {
		t.Fatalf("detectPlanRoute(--dev) = (%q, %v, %v), want (\"\", nil, false)", name, extra, ok)
	}
}

// The no-plans guard reads args[0] only, exactly as the plans-configured one
// does. A flag value like FOO=bare does not look like a flag and must not be
// mistaken for a positional argument now that the guard fires without plans.
func TestUpWithoutPlansGuardOnlyInspectsPlanNameSlot(t *testing.T) {
	useConfig(t, noPlanStackConfig)

	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	for _, args := range [][]string{
		{"--dev"},
		{"--force"},
		{"--var", "FOO=bare"},
		{"--var=FOO=bare"},
	} {
		if err := upCmd.RunE(upCmd, args); err != nil {
			t.Errorf("'dva up %s' must not be read as a positional argument: %v", strings.Join(args, " "), err)
		}
	}
}
