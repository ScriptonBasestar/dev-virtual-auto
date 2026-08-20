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
		{"--force"},
		{"--no-wait"},
		{"--mode", "dev"},
		{"--var", "FOO=bare"},
		// Deliberately not a real flag. The rows above read `--dev` and `--docker`
		// until docs/43 removed them, and the fact that those rows kept passing after
		// the flags were gone is itself the property worth pinning: `up` sets
		// DisableFlagParsing, so this guard inspects raw argv and fires on any leading
		// '-' token before cobra ever validates it. A user who mistypes a flag under a
		// default plan gets told to name the plan, not that the flag is unknown.
		{"--no-such-flag"},
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

// A `--` in the plan-name slot is a separator, so the token after it is the plan name —
// the same reading detectPlanRoute uses. Left unconsumed the terminator took the
// leading-dash early return with it and this guard went quiet: `dva status -- -- s1` ran a
// full status and exited 0, where `dva status -- s1` refused with "plan 's1' not found".
//
// Asserted differentially against the unseparated form rather than against the message,
// because the claim is that the separator changes nothing about what the name means.
// TASK-210.
func TestRejectUnknownPlanArg_TerminatorDoesNotDisarmTheCheck(t *testing.T) {
	c := &config.Config{Plans: map[string]*config.PlanConfig{"p1": {}}}

	plain := rejectUnknownPlanArg(c, []string{"s1"})
	separated := rejectUnknownPlanArg(c, []string{"--", "s1"})
	switch {
	case (plain == nil) != (separated == nil):
		t.Fatalf("rejectUnknownPlanArg(--, s1) = %v; rejectUnknownPlanArg(s1) = %v; a separator does not change what the name after it means", separated, plain)
	case plain != nil && plain.Error() != separated.Error():
		t.Fatalf("rejectUnknownPlanArg(--, s1) = %q; rejectUnknownPlanArg(s1) = %q; same name, same refusal", separated, plain)
	}

	// A terminator names nothing on its own, so there is nothing to reject.
	if err := rejectUnknownPlanArg(c, []string{"--"}); err != nil {
		t.Fatalf("a lone terminator names no plan: %v", err)
	}
	// The leading-dash early return still applies to what FOLLOWS the separator. This guard
	// only ever says "plan not found", which is the wrong sentence for a dash-shaped token;
	// the command's own name guard owns that one.
	if err := rejectUnknownPlanArg(c, []string{"--", "--force"}); err != nil {
		t.Fatalf("a dash-shaped token is not this guard's to name: %v", err)
	}
	// Not asserted here: `--, p1` with p1 declared. This guard is fallthrough-only — every
	// caller runs detectPlanRoute first, which routes a declared plan and never reaches it —
	// so it does not re-check the name against c.Plans, and calling it with a real plan name
	// produces "plan 'p1' not found. Available: p1". A test asserting that call would pin a
	// message no user can reach. Found by writing it and reading the failure. TASK-210.
}

func TestRejectSuppressedDefaultPlan_LeadingFlag(t *testing.T) {
	c := &config.Config{Plans: map[string]*config.PlanConfig{"p1": {}}}

	err := rejectSuppressedDefaultPlan(c, "up", []string{"--force"})
	if err == nil {
		t.Fatal("expected error when flags suppress the default plan")
	}
	if got := err.Error(); !strings.Contains(got, "dva up p1 --force") {
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
	if err := rejectSuppressedDefaultPlan(c, "up", []string{"--force"}); err != nil {
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
	name, extra, ok := detectPlanRoute(c, []string{"--force"})
	if ok || name != "" || extra != nil {
		t.Fatalf("detectPlanRoute(--force) = (%q, %v, %v), want (\"\", nil, false)", name, extra, ok)
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

	// Shapes, not a flag inventory: a bare flag, a flag whose value is a separate
	// token, and the same value inlined. `--dev` led this list until docs/43 removed
	// it; unlike the sibling test above, this config has no plans, so the guard does
	// not fire and args reach parseDvaFlags — an unknown flag fails here for the
	// ordinary reason, which is not what this test is about. `--force` already covers
	// the bare-flag shape.
	for _, args := range [][]string{
		{"--force"},
		{"--var", "FOO=bare"},
		{"--var=FOO=bare"},
	} {
		if err := upCmd.RunE(upCmd, args); err != nil {
			t.Errorf("'dva up %s' must not be read as a positional argument: %v", strings.Join(args, " "), err)
		}
	}
}
