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

// The guard keys off plans being configured. Without plans a positional argument
// was never a plan name, so the legacy whole-stack path must stay reachable.
func TestUpWithoutPlansKeepsLegacyPath(t *testing.T) {
	useConfig(t, noPlanStackConfig)

	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	if err := upCmd.RunE(upCmd, []string{}); err != nil {
		t.Fatalf("'dva up' with no plans must start the stack: %v", err)
	}
	if err := upCmd.RunE(upCmd, []string{"s1"}); err != nil {
		t.Fatalf("'dva up s1' with no plans defined must keep its previous behavior: %v", err)
	}
}

// The guard reads only args[0], the sole position detectPlanRoute treats as a
// plan name. A leading flag means the invocation was never plan-routed, so it
// keeps its existing behavior — including the flag values that follow, which do
// not look like flags and must not be mistaken for a plan name.
func TestUpPlanGuardOnlyInspectsPlanNameSlot(t *testing.T) {
	useConfig(t, planStackConfig)

	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	// --dev is not routed to the plan (parsePlanFlags does not accept it), so it
	// reaches the legacy whole-stack path and must stay there.
	if err := upCmd.RunE(upCmd, []string{"--dev"}); err != nil {
		t.Fatalf("'dva up --dev' must keep the whole-stack path: %v", err)
	}
	for _, args := range [][]string{
		{"--force"},
		{"--no-wait"},
		{"--docker"},
		{"--var", "FOO=bare"}, // FOO=bare is a flag value, not a plan name
	} {
		if err := upCmd.RunE(upCmd, args); err != nil {
			t.Errorf("'dva up %s' must not be read as a plan name: %v", strings.Join(args, " "), err)
		}
	}
}
