package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

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

// TestLoneTerminatorMatchesTheBareForm is TASK-216's ruling in one table: `dva <verb> --`
// means what `dva <verb>` means, for up, down and stop, in every config shape. TASK-207 and
// TASK-210 had ruled the identity restart-local, and 12 of the 18 verb x fixture pairs
// disagreed with the bare form as a result — 9 of them by refusing where a bare invocation
// tore the whole stack down, which is the shape `dva down -- "$@"` lands in when "$@" is
// empty.
//
// restart is here as a control, not as new coverage. It already held the identity before this
// card, so its rows failing means the harness moved, not that the ruling regressed; and it is
// the only verb that reaches the identity through a different helper (dropFlagTerminator on
// the name list, rather than dropLeadingTerminator on the raw args), so keeping it in the same
// table is what stops the two implementations drifting apart unnoticed.
//
// Differential rather than expected-string, which is the point of the criterion: the assertion
// is that the two forms agree with EACH OTHER, so rewording any of these messages leaves it
// passing and reopening the divergence fails it. A hardcoded-message version would have had to
// be rewritten by the very change it exists to catch — TestRestartBareTerminatorMeansABareRestart
// records that happening once already, where a literal expectation read a measured divergence
// as agreement.
//
// Markers are compared as well as the error, because on the teardown verbs "refused" and "ran
// nothing" are not distinguishable from the exit code alone: a variant that swallowed `--` as
// an unmatchable name would exit 0 having done nothing, which no error comparison separates
// from the whole-stack teardown a bare `dva down` performs.
func TestLoneTerminatorMatchesTheBareForm(t *testing.T) {
	verbs := []struct {
		name string
		cmd  *cobra.Command
	}{
		{"up", upCmd},
		{"down", downCmd},
		{"stop", stopCmd},
		{"restart", restartCmd},
	}
	// The four shapes TASK-216 measured, reusing restart's fixture writers rather than a
	// parallel set: they are the configs the identity was first ruled against, and the down
	// hooks they now carry are what make the teardown rows observable at all.
	fixtures := []struct {
		shape string
		write func(*testing.T) string
	}{
		{"no plans", writeRestartProbeConfig},
		{"two plans, no default_plan", writeRestartPlanProbeConfig},
		{"explicit default_plan", writeRestartDefaultPlanProbeConfig},
		{"lone plan promoted to default", writeRestartLonePlanProbeConfig},
	}
	markers := []string{"s1_up", "s1_stop", "s1_down", "s2_up", "s2_stop", "s2_down"}

	// Positive control, counted PER VERB. Agreement is the assertion, and two invocations that
	// both do nothing agree perfectly — so a fixture that could not reach a lifecycle hook, or
	// a marker list that named files no hook writes, would pass this whole table without
	// measuring anything. Counted rather than asserted per row, because the refusing shapes
	// are SUPPOSED to leave no markers; what must not happen is a whole VERB being nothing but
	// those.
	//
	// Per verb and not one total, because a single global counter is satisfied by the loudest
	// column and then reports for the quiet ones. Measured: neutering only the `down:` hooks in
	// the four fixture writers (8 substitutions) left all 16 subtests passing under a global
	// counter — the `up` rows kept it non-zero while the entire `down` column silently compared
	// two empty marker sets. Per verb, that same edit fails on `down`, which is the column the
	// 9 silent-direction rows of this card live in.
	ranSomething := map[string]int{}

	for _, v := range verbs {
		for _, fx := range fixtures {
			t.Run(v.name+"/"+fx.shape, func(t *testing.T) {
				bareDir := fx.write(t)
				bareErr := v.cmd.RunE(v.cmd, nil)
				bareRan := ranMarkers(t, bareDir)

				termDir := fx.write(t)
				termErr := v.cmd.RunE(v.cmd, []string{"--"})
				termRan := ranMarkers(t, termDir)

				if len(bareRan) > 0 {
					ranSomething[v.name]++
				}

				switch {
				case bareErr == nil && termErr != nil:
					t.Fatalf("dva %s --: %v; a bare `dva %s` succeeds in this config, so the terminator form must too", v.name, termErr, v.name)
				case bareErr != nil && termErr == nil:
					t.Fatalf("dva %s -- exited 0 having run %v, but a bare `dva %s` here is refused with %v; `--` must never do more than the bare form is permitted to", v.name, termRan, v.name, bareErr)
				case bareErr != nil && termErr != nil && bareErr.Error() != termErr.Error():
					t.Fatalf("dva %s --: %q; a bare `dva %s` says %q, and \"no names given\" must be the same refusal", v.name, termErr, v.name, bareErr)
				}
				for _, m := range markers {
					if bareRan[m] != termRan[m] {
						t.Errorf("dva %s --: %s ran=%v; a bare `dva %s` ran=%v", v.name, m, termRan[m], v.name, bareRan[m])
					}
				}
			})
		}
	}

	for _, v := range verbs {
		if ranSomething[v.name] == 0 {
			t.Errorf("no %s row reached a lifecycle hook across %d fixtures; the %s column above agrees only because both forms did nothing", v.name, len(fixtures), v.name)
		}
	}
}

// TestLeadingTerminatorIsConsumedOnceNotRepeatedly pins the half of the ruling the table above
// cannot see. `dva <verb> --` ≡ `dva <verb>` is upheld there by comparing the two forms; a
// variant that dropped EVERY leading terminator rather than the first would satisfy every one
// of those rows, because they only ever pass one. Measured before this test existed: rewriting
// dropLeadingTerminator as a loop passed `go test ./internal/cli/` whole-package green while
// contradicting dropFlagTerminator's doc comment and USAGE.md:218.
//
// The fixture is the shape where a bare invocation SUCCEEDS, so "still refused" is a real
// verdict rather than one refusal standing in for another: `dva up --` starts the stack here,
// and `dva up -- --` must not, because the second `--` is an argument and these verbs take
// none. Markers are checked too — under a drop-all variant the doubled form does not merely
// exit 0, it runs the whole stack.
func TestLeadingTerminatorIsConsumedOnceNotRepeatedly(t *testing.T) {
	verbs := []struct {
		name string
		cmd  *cobra.Command
	}{
		{"up", upCmd},
		{"down", downCmd},
		{"stop", stopCmd},
		{"restart", restartCmd},
	}

	for _, v := range verbs {
		t.Run(v.name, func(t *testing.T) {
			bareDir := writeRestartProbeConfig(t)
			if err := v.cmd.RunE(v.cmd, []string{"--"}); err != nil {
				t.Fatalf("dva %s --: %v; this fixture is chosen because the single terminator form succeeds in it", v.name, err)
			}
			if len(ranMarkers(t, bareDir)) == 0 {
				t.Fatalf("dva %s -- ran no lifecycle hook here; the contrast below would be between two refusals", v.name)
			}

			doubleDir := writeRestartProbeConfig(t)
			err := v.cmd.RunE(v.cmd, []string{"--", "--"})
			ran := ranMarkers(t, doubleDir)
			if err == nil {
				t.Fatalf("dva %s -- -- exited 0 having run %v; only the leading terminator is a separator, so the second one is an argument and %s takes none", v.name, ran, v.name)
			}
			if len(ran) > 0 {
				t.Errorf("dva %s -- -- was refused with %v but still ran %v", v.name, err, ran)
			}
		})
	}
}

// TestSecondTerminatorMeetsThePlanGuardNotTheFlagGuard pins the four rows TASK-217's own
// comment said could not exist. That comment claimed up/down/stop drop the terminator at
// their call sites and that only `dva build` reaches requirePlanSelection with one intact.
// Reverting that single line on a two-plan no-default fixture moves five rows, and four of
// them are not build: `up -- --`, `down -- --`, `stop -- --` and `logs -- --`. The call
// sites drop the FIRST terminator, so a second one arrives here exactly as build's only one
// does.
//
// TestLeadingTerminatorIsConsumedOnceNotRepeatedly cannot see this. It runs on the
// default-plan fixture, where both readings refuse and run nothing, and it asserts only
// that something refused. The claim here is narrower and is the one the revert breaks: the
// refusal must come from the plan guard, in the same words a bare verb gets, rather than
// from the unknown-flag guard one layer out.
//
// `logs -- --` is the fourth moved row and is deliberately NOT a subtest here. Under the
// revert it reaches execComposePassthrough, and ExecReplace panics the test binary on
// purpose (TASK-144) rather than letting syscall.Exec swallow the run. A subtest for it
// would trade one recorded failure for the loss of every test after it —
// TestBuildLoneTerminatorMeansABareBuild already aborts the suite that way when this line
// regresses, which is why "the revert fails one test" must be read off a run that finished.
func TestSecondTerminatorMeetsThePlanGuardNotTheFlagGuard(t *testing.T) {
	verbs := []struct {
		name string
		cmd  *cobra.Command
	}{
		{"up", upCmd},
		{"down", downCmd},
		{"stop", stopCmd},
	}

	for _, v := range verbs {
		t.Run(v.name, func(t *testing.T) {
			writeRestartPlanProbeConfig(t)
			bareErr := v.cmd.RunE(v.cmd, nil)
			if bareErr == nil {
				t.Fatalf("dva %s: exited 0 in a two-plan config with no default_plan; that refusal is the whole premise of this test", v.name)
			}

			doubleDir := writeRestartPlanProbeConfig(t)
			termErr := v.cmd.RunE(v.cmd, []string{"--", "--"})
			if termErr == nil {
				t.Fatalf("dva %s -- --: exited 0 having run %v; a bare %s here is refused with %v", v.name, ranMarkers(t, doubleDir), v.name, bareErr)
			}
			if termErr.Error() != bareErr.Error() {
				t.Errorf("dva %s -- --: %q; a bare %s says %q. The second terminator is an argument this verb does not take, so both must reach the same guard", v.name, termErr, v.name, bareErr)
			}
			if !strings.Contains(termErr.Error(), "multiple plans configured") {
				t.Errorf("dva %s -- --: %q; the two forms agree but neither is the plan guard, so this subtest would pass on any shared failure", v.name, termErr)
			}
		})
	}
}

// TestSecondTerminatorDoesNotDisarmBuildsPlanGuard is TASK-224's differential. Build consumes
// one separator at its own call site; requirePlanSelection consumes its own plan-name-slot
// separator. Therefore `build -- --` reaches the same empty selection and same refusal as a
// bare build, rather than passing a literal service name to Compose.
func TestSecondTerminatorDoesNotDisarmBuildsPlanGuard(t *testing.T) {
	buildTwoPlanComposeProbe(t)
	bareErr := buildCmd.RunE(buildCmd, nil)

	buildTwoPlanComposeProbe(t)
	secondTerminatorErr := buildCmd.RunE(buildCmd, []string{"--", "--"})

	if bareErr == nil {
		t.Fatal("build: exited 0 in a two-plan config with no default_plan; the differential has no guard to compare")
	}
	if secondTerminatorErr == nil {
		t.Fatalf("build -- --: exited 0, but bare build is refused with %v", bareErr)
	}
	if secondTerminatorErr.Error() != bareErr.Error() {
		t.Fatalf("build -- --: %q; bare build says %q", secondTerminatorErr, bareErr)
	}
	if !strings.Contains(secondTerminatorErr.Error(), "multiple plans configured") {
		t.Fatalf("build -- --: %q; both forms agree but not at the plan guard", secondTerminatorErr)
	}
}

// TestBuildTerminatorPassthroughAndTriplePolicy holds the two controls that keep TASK-224
// narrow. A single separator still exposes the next token to Compose, while a triple leaves two
// tokens for Compose after build consumes exactly one of its own.
func TestBuildTerminatorPassthroughAndTriplePolicy(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want []string
	}{
		{"service name", []string{"--", "web"}, []string{"build", "web"}},
		{"compose flag", []string{"--", "--no-cache"}, []string{"build", "--no-cache"}},
		{"triple terminator", []string{"--", "--", "--"}, []string{"build", "--", "--"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var argv func() []string
			if tc.name == "triple terminator" {
				argv = buildTwoPlanComposeProbe(t)
			} else {
				argv = composePassthroughFixtureWith(t, buildFixtureYAML)
			}

			if err := buildCmd.RunE(buildCmd, tc.args); err != nil {
				t.Fatalf("build %v: %v", tc.args, err)
			}
			got := argv()
			if len(got) != 1 {
				t.Fatalf("docker invocations = %v, want one argv", got)
			}
			tokens := strings.Fields(got[0])
			buildAt := slices.Index(tokens, "build")
			if buildAt < 0 || !slices.Equal(tokens[buildAt:], tc.want) {
				t.Errorf("docker argv = %q, want build tail %q", tokens, tc.want)
			}
		})
	}
}

func buildTwoPlanComposeProbe(t *testing.T) func() []string {
	t.Helper()
	return composePassthroughFixtureWith(t, `version: "0.1.44"
stack:
  s1:
    default_runner: compose
    runners:
      compose:
        files: [docker-compose.yml]
  s2:
    default_runner: compose
    runners:
      compose:
        files: [docker-compose.yml]
plans:
  p1:
    entries:
      - name: s1
  p2:
    entries:
      - name: s2
`)
}

// TestUpLoneDashAgreesWithABareUp is TASK-218's differential, in the shape the card
// specifies: two plans, no default_plan. There a bare `dva up` refuses — it will not guess
// which plan — while `dva up -` started every entry in the stack and reported success. One
// character turned a refusal into a whole-stack start.
//
// Asserted on what RAN rather than on the messages, because the two invocations are not
// answered by the same guard and never will be: a bare up is refused by requirePlanSelection
// ("multiple plans configured"), a dash by the name guard ("plan '-' not found"). Those are
// both correct and both different. The claim is narrower and is the one the escalation
// violates — `-` must not buy an action the bare form is refused.
//
// The no-plans shape is pinned separately by
// TestUpPositionalGuardReportsALoneDashWithNoPlans, where a different guard fires; keeping
// them apart means a revert of either one fails on its own subtest.
func TestUpLoneDashAgreesWithABareUp(t *testing.T) {
	bareDir := writeRestartPlanProbeConfig(t)
	bareErr := upCmd.RunE(upCmd, nil)
	bareRan := ranMarkers(t, bareDir)

	dashDir := writeRestartPlanProbeConfig(t)
	dashErr := upCmd.RunE(upCmd, []string{"-"})
	dashRan := ranMarkers(t, dashDir)

	if bareErr == nil {
		t.Fatalf("up: exited 0 having run %v; this fixture declares two plans and no default_plan, and a bare up refusing to guess is the premise of the differential", bareRan)
	}
	if dashErr == nil {
		t.Fatalf("up -: exited 0 having run %v, but a bare up here is refused with %v; a lone dash must not buy an action the bare form cannot have", dashRan, bareErr)
	}
	for _, m := range []string{"s1_up", "s2_up"} {
		if bareRan[m] != dashRan[m] {
			t.Errorf("up -: %s ran=%v; a bare up ran=%v", m, dashRan[m], bareRan[m])
		}
	}
	if len(dashRan) != 0 {
		t.Errorf("up -: refused with %v but still ran %v; the guard must stop the command before any entry is touched", dashErr, dashRan)
	}
}
