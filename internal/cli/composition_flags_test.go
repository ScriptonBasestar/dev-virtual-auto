package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/lifecycle"
)

// compositionFixtureConfig is a two-child local composition plan: svc-a's plan (a-plan) in
// wave 0, svc-b's plan (b-plan) in wave 1 depending on it. Both stack entries use the native
// runner so tests need no docker/compose availability, and each declares a build command so
// TestCompositionAggregateLogsStatusBuild can pin build order without starting anything.
const compositionFixtureConfig = `version: "0.1.0"
stack:
  svc-a:
    default_runner: native
    runners:
      native:
        run: "true"
        build: 'echo a >> build-order'
  svc-b:
    default_runner: native
    runners:
      native:
        run: "true"
        build: 'echo b >> build-order'
plans:
  a-plan:
    entries:
      - name: svc-a
  b-plan:
    entries:
      - name: svc-b
  release:
    composes:
      - plan: a-plan
        order: 0
      - plan: b-plan
        order: 1
        depends_on: ["a-plan"]
`

func compositionFixture(t *testing.T) (*config.Config, *lifecycle.CompositionPlan) {
	t.Helper()
	c := loadTestConfig(t, compositionFixtureConfig)
	comp, err := lifecycle.ResolveCompositionPlan(c, "release")
	if err != nil {
		t.Fatalf("ResolveCompositionPlan: %v", err)
	}
	return c, comp
}

// TestCompositionPropagateAllFlags proves --no-wait and --no-rollback pass validation
// unchanged (propagate-to-all, TASK-260 §4.4) on every lifecycle verb where they apply, and
// that a composition invocation carrying only these flags reaches real orchestrator
// execution rather than being rejected as an unsupported or out-of-scope flag.
func TestCompositionPropagateAllFlags(t *testing.T) {
	_, comp := compositionFixture(t)

	t.Run("--no-wait propagates on up and restart", func(t *testing.T) {
		for _, verb := range []string{"up", "restart"} {
			flags, err := validateCompositionFlagScope(comp, "release", verb, []string{"--no-wait"})
			if err != nil {
				t.Fatalf("verb %s: --no-wait rejected: %v", verb, err)
			}
			if !flags.noWait {
				t.Fatalf("verb %s: flags.noWait = false, want true", verb)
			}
		}
	})

	t.Run("--no-wait rejected where the single-plan parser also rejects it", func(t *testing.T) {
		for _, verb := range []string{"down", "stop"} {
			if _, err := validateCompositionFlagScope(comp, "release", verb, []string{"--no-wait"}); err == nil {
				t.Fatalf("verb %s: expected --no-wait to be rejected, got nil", verb)
			}
		}
	})

	t.Run("--no-rollback propagates on up only", func(t *testing.T) {
		flags, err := validateCompositionFlagScope(comp, "release", "up", []string{"--no-rollback"})
		if err != nil {
			t.Fatalf("--no-rollback rejected on up: %v", err)
		}
		if !flags.noRollback {
			t.Fatal("flags.noRollback = false, want true")
		}
		for _, verb := range []string{"down", "stop", "restart", "build", "logs"} {
			if _, err := validateCompositionFlagScope(comp, "release", verb, []string{"--no-rollback"}); err == nil {
				t.Fatalf("verb %s: expected --no-rollback to be rejected (only up rolls back), got nil", verb)
			}
		}
	})

	t.Run("both flags together pass validation and reach real execution, not a flag rejection", func(t *testing.T) {
		c := compositionFixtureConfigLoaded(t)
		el := planEnv(config.NewEnvironment(nil, c.FileDir(), c.FileDir()))
		err := runCompositionUp(c, el, "release", []string{"--no-wait", "--no-rollback"})
		if err != nil && (strings.Contains(err.Error(), "unsupported flag") || strings.Contains(err.Error(), "does not accept")) {
			t.Fatalf("expected --no-wait/--no-rollback to pass validation and reach real execution, got a flag-scope rejection: %v", err)
		}
	})
}

func compositionFixtureConfigLoaded(t *testing.T) *config.Config {
	t.Helper()
	return loadTestConfig(t, compositionFixtureConfig)
}

// TestCompositionRejectsWholeStackAndVarFlags proves --var, the tag selectors, --mode and
// --env are rejected before any child starts, and that the errors name the plan (and, for
// --var, point at CompositionEntry.Vars) per TASK-260 §4.4.
func TestCompositionRejectsWholeStackAndVarFlags(t *testing.T) {
	_, comp := compositionFixture(t)

	cases := []struct {
		name       string
		args       []string
		wantVarRef bool
	}{
		{"--var bare", []string{"--var", "KEY=VAL"}, true},
		{"--var inline", []string{"--var=KEY=VAL"}, true},
		{"--mode long", []string{"--mode", "dev"}, false},
		{"--mode inline", []string{"--mode=dev"}, false},
		{"-M short", []string{"-M", "dev"}, false},
		{"--env long", []string{"--env", "dev"}, false},
		{"-E short", []string{"-E", "dev"}, false},
		{"--tag", []string{"--tag", "web"}, false},
		{"--tags", []string{"--tags", "web"}, false},
		{"-T short", []string{"-T", "web"}, false},
		{"--exclude-tag", []string{"--exclude-tag", "web"}, false},
		{"--exclude-tags", []string{"--exclude-tags", "web"}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validateCompositionFlagScope(comp, "release", "up", tc.args)
			if err == nil {
				t.Fatalf("args %v: expected rejection before any child starts, got nil", tc.args)
			}
			if !strings.Contains(err.Error(), "release") {
				t.Errorf("error = %q, must name the plan %q", err, "release")
			}
			if tc.wantVarRef && !strings.Contains(err.Error(), "CompositionEntry.Vars") {
				t.Errorf("error = %q, must point at CompositionEntry.Vars for --var", err)
			}
		})
	}
}

// TestCompositionDestructiveFlagsRequireProjectScope proves --force/--volumes/--purge are
// refused before any child starts unless scoped with --project to exactly one composed
// child, and that a scoped, valid invocation resolves to that child (TASK-260 §4.4's
// require-explicit-scope row and worked example).
func TestCompositionDestructiveFlagsRequireProjectScope(t *testing.T) {
	_, comp := compositionFixture(t)

	t.Run("missing scope refuses before any child starts", func(t *testing.T) {
		for _, flag := range []string{"--force", "--volumes", "-v", "--purge"} {
			if _, err := validateCompositionFlagScope(comp, "release", "down", []string{flag}); err == nil {
				t.Fatalf("flag %s without --project: expected rejection, got nil", flag)
			} else if !strings.Contains(err.Error(), "--project") {
				t.Errorf("flag %s: error = %q, must mention --project", flag, err)
			}
		}
	})

	t.Run("scope naming an actual child resolves", func(t *testing.T) {
		flags, err := validateCompositionFlagScope(comp, "release", "down", []string{"--purge", "--project", "a-plan"})
		if err != nil {
			t.Fatalf("valid scope rejected: %v", err)
		}
		if flags.scopedChild == nil {
			t.Fatal("scopedChild = nil, want the resolved a-plan entry")
		}
		if got := flags.scopedChild.ChildPlan.Name; got != "a-plan" {
			t.Fatalf("scopedChild = %q, want a-plan", got)
		}
	})

	t.Run("scope naming an unknown child refuses", func(t *testing.T) {
		if _, err := validateCompositionFlagScope(comp, "release", "down", []string{"--volumes", "--project", "no-such-plan"}); err == nil {
			t.Fatal("expected unknown scope to be rejected, got nil")
		} else if !strings.Contains(err.Error(), "unknown scope") {
			t.Errorf("error = %q, want it to say 'unknown scope'", err)
		}
	})

	t.Run("--project without a destructive flag is rejected", func(t *testing.T) {
		if _, err := validateCompositionFlagScope(comp, "release", "up", []string{"--project", "a-plan"}); err == nil {
			t.Fatal("expected --project without --force/--volumes/--purge to be rejected, got nil")
		}
	})

	t.Run("purge and volumes are down-only, even with a valid --project scope", func(t *testing.T) {
		for _, verb := range []string{"up", "stop", "restart", "build", "logs"} {
			for _, flag := range []string{"--purge", "--volumes"} {
				_, err := validateCompositionFlagScope(comp, "release", verb, []string{flag, "--project", "a-plan"})
				if err == nil {
					t.Fatalf("verb %s flag %s: expected rejection (down-only), got nil", verb, flag)
				}
				if !strings.Contains(err.Error(), "only supported by down") {
					t.Errorf("verb %s flag %s: error = %q, want it to say 'only supported by down'", verb, flag, err)
				}
			}
		}
	})

	t.Run("purge and volumes remain valid on down with a scope", func(t *testing.T) {
		for _, flag := range []string{"--purge", "--volumes"} {
			if _, err := validateCompositionFlagScope(comp, "release", "down", []string{flag, "--project", "a-plan"}); err != nil {
				t.Errorf("flag %s on down: unexpected rejection: %v", flag, err)
			}
		}
	})

	t.Run("nothing starts on rejection", func(t *testing.T) {
		el := planEnv(config.NewEnvironment(nil, "", ""))
		c := compositionFixtureConfigLoaded(t)
		// build is fully implemented (unlike up/down/stop/restart, which are TASK-291
		// stubs and touch nothing regardless of validation outcome) and would append to
		// build-order for every composed child if the down-only guard let --purge reach
		// the build loop. This is a genuinely falsifiable check of "rejected before any
		// child starts" — unlike asserting build-order is absent after a rejected `down`,
		// which passes trivially since `down` never writes build-order at all.
		if err := runCompositionBuild(c, el, "release", []string{"--purge"}); err == nil {
			t.Fatal("expected --purge to be rejected on build (down-only flag), got nil")
		}
		if _, statErr := os.Stat(filepath.Join(c.FileDir(), "build-order")); !os.IsNotExist(statErr) {
			t.Fatal("a child appears to have run despite the rejected --purge flag")
		}
	})
}

// TestCompositionAggregateLogsStatusBuild proves logs are labeled per child project, build
// runs children in the same wave order as up without any rollback machinery, and status
// returns TASK-260 §5.3's partial-state JSON shape (reused for the success case per §5.5) in
// both text and --json output.
func TestCompositionAggregateLogsStatusBuild(t *testing.T) {
	t.Run("build follows wave order, non-destructive", func(t *testing.T) {
		c := loadTestConfig(t, compositionFixtureConfig)
		el := planEnv(config.NewEnvironment(nil, c.FileDir(), c.FileDir()))
		if err := runCompositionBuild(c, el, "release", nil); err != nil {
			t.Fatalf("runCompositionBuild: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(c.FileDir(), "build-order"))
		if err != nil {
			t.Fatalf("read build-order: %v", err)
		}
		if got := strings.Fields(string(data)); strings.Join(got, ",") != "a,b" {
			t.Fatalf("build order = %v, want wave order [a b]", got)
		}
	})

	t.Run("logs are prefixed per composed child project", func(t *testing.T) {
		c := loadTestConfig(t, compositionFixtureConfig)
		el := planEnv(config.NewEnvironment(nil, c.FileDir(), c.FileDir()))
		writeEntryLog(t, c, "svc-a", "log-from-a")
		writeEntryLog(t, c, "svc-b", "log-from-b")

		var logErr error
		out := captureStdout(t, func() {
			logErr = runCompositionLogs(c, el, "release", nil)
		})
		if logErr != nil {
			t.Fatalf("runCompositionLogs: %v", logErr)
		}
		if !strings.Contains(out, "[project: a-plan]") {
			t.Errorf("output missing project label for a-plan:\n%s", out)
		}
		if !strings.Contains(out, "[project: b-plan]") {
			t.Errorf("output missing project label for b-plan:\n%s", out)
		}
		if !strings.Contains(out, "log-from-a") || !strings.Contains(out, "log-from-b") {
			t.Errorf("output missing aggregated child log content:\n%s", out)
		}
		if idxA, idxB := strings.Index(out, "a-plan"), strings.Index(out, "b-plan"); idxA == -1 || idxB == -1 || idxA > idxB {
			t.Errorf("output did not present children in wave order:\n%s", out)
		}
	})

	t.Run("status text output reports failed outcome for a fully-down composition", func(t *testing.T) {
		// Nothing in this fixture was ever started (up is a TASK-291 stub, so no child's
		// process was launched) — per-child state still says so via TASK-260 §5.3's
		// "not_started" state, but the composition-level outcome must be the frozen
		// "failed" (TASK-297): a fully-down composition is not a success.
		c := loadTestConfig(t, compositionFixtureConfig)
		el := planEnv(config.NewEnvironment(nil, c.FileDir(), c.FileDir()))

		var statusErr error
		out := captureStdout(t, func() {
			statusErr = runCompositionStatus(c, el, "release")
		})
		if statusErr == nil {
			t.Fatal("expected runCompositionStatus to report an error: composition is fully down")
		}
		if !strings.Contains(out, "a-plan") || !strings.Contains(out, "b-plan") {
			t.Errorf("text status missing both children:\n%s", out)
		}
		if !strings.Contains(out, "not_started") {
			t.Errorf("text status must report not_started for children that were never up:\n%s", out)
		}
		if strings.Contains(out, ": up\n") {
			t.Errorf("text status must not claim a never-started child is up:\n%s", out)
		}
		if !strings.Contains(out, "outcome: failed") {
			t.Errorf("text status outcome must be failed for a fully-down composition:\n%s", out)
		}
	})

	t.Run("status --json reuses TASK-260 §5.3's shape and reports honest per-child state", func(t *testing.T) {
		c := loadTestConfig(t, compositionFixtureConfig)
		el := planEnv(config.NewEnvironment(nil, c.FileDir(), c.FileDir()))

		oldJSON := jsonOutput
		jsonOutput = true
		defer func() { jsonOutput = oldJSON }()

		var statusErr error
		out := captureStdout(t, func() {
			statusErr = runCompositionStatus(c, el, "release")
		})
		if statusErr == nil {
			t.Fatal("expected runCompositionStatus --json to report an error: composition is fully down")
		}

		var report struct {
			lifecycle.CompositionReport
			DvaVersion string `json:"dva_version"`
		}
		if err := json.Unmarshal([]byte(out), &report); err != nil {
			t.Fatalf("json.Unmarshal(%q): %v", out, err)
		}
		if report.Plan != "release" {
			t.Errorf("plan = %q, want release", report.Plan)
		}
		if report.Kind != "composition" {
			t.Errorf("kind = %q, want composition", report.Kind)
		}
		// Nothing was ever started in this fixture (up is a TASK-291 stub) — the real
		// per-entry status is "stopped" for every native/process child, so every child's
		// state must say "not_started", never "up", and the composition-level outcome
		// must be the frozen "failed" (TASK-297; a fully-down composition is not a
		// success even though no child query itself errored).
		if report.Outcome != "failed" {
			t.Errorf("outcome = %q, want failed (nothing in this fixture was ever started)", report.Outcome)
		}
		if report.DvaVersion != config.Version {
			t.Errorf("dva_version = %q, want %q", report.DvaVersion, config.Version)
		}
		if len(report.Children) != 2 {
			t.Fatalf("children = %d, want 2", len(report.Children))
		}
		// A composed child with no "/" in its name is a local leaf plan with no owning
		// subproject, so its Project column is empty (lifecycle.splitChildLabel) — the
		// full name lives in Plan instead.
		wantPlans := map[string]bool{"a-plan": true, "b-plan": true}
		for _, child := range report.Children {
			if child.Project != "" {
				t.Errorf("unexpected child %+v: local plan must have an empty Project", child)
			}
			if !wantPlans[child.Plan] {
				t.Errorf("unexpected child %+v", child)
			}
			if child.State != "not_started" {
				t.Errorf("child %s state = %q, want not_started (fixture never ran anything)", child.Plan, child.State)
			}
		}
		if report.Rollback.Attempted == nil || report.Rollback.Succeeded == nil || report.Rollback.Failed == nil {
			t.Errorf("rollback report fields must be present-but-empty arrays, got %+v", report.Rollback)
		}
	})
}

// TestCompositionStatusExitsNonzeroWhenDown pins TASK-297's fix directly: a composition where
// every child query succeeds but nothing is running must exit non-zero (never the old
// out-of-contract "not_started"/exit-0 outcome). The text- and --json-mode assertions above
// already cover the full report shape; this only pins the error/exit-code contract itself.
func TestCompositionStatusExitsNonzeroWhenDown(t *testing.T) {
	c := loadTestConfig(t, compositionFixtureConfig)
	el := planEnv(config.NewEnvironment(nil, c.FileDir(), c.FileDir()))

	var statusErr error
	captureStdout(t, func() {
		statusErr = runCompositionStatus(c, el, "release")
	})
	if statusErr == nil {
		t.Fatal("expected runCompositionStatus to report an error for a fully-down composition")
	}
}

// TestCompositionExitCodesStayFlat proves composition rejection/validation errors are plain
// errors — never a custom exit-code-carrying type — so root.go's existing flat os.Exit(1)
// convention applies to composition plans without any new plumbing (TASK-260 §4's explicit
// "no new exit-code taxonomy").
func TestCompositionExitCodesStayFlat(t *testing.T) {
	c, comp := compositionFixture(t)
	el := planEnv(config.NewEnvironment(nil, c.FileDir(), c.FileDir()))

	type exitCoder interface{ ExitCode() int }

	errs := []error{}
	if _, err := validateCompositionFlagScope(comp, "release", "up", []string{"--var", "K=V"}); err != nil {
		errs = append(errs, err)
	}
	if _, err := validateCompositionFlagScope(comp, "release", "down", []string{"--purge"}); err != nil {
		errs = append(errs, err)
	}
	if _, err := validateCompositionFlagScope(comp, "release", "down", []string{"--purge", "--project", "no-such"}); err != nil {
		errs = append(errs, err)
	}
	// A rejected flag, not nil: since the orchestrator wiring landed, nil extraArgs would
	// make this attempt real execution against the native "true" fixture instead of
	// exercising a deterministic error path — --tag is rejected before any child starts
	// regardless, keeping this case about runCompositionUp's error type, not its runtime
	// outcome.
	if err := runCompositionUp(c, el, "release", []string{"--tag", "x"}); err != nil {
		errs = append(errs, err)
	}
	if err := runCompositionDown(c, el, "release", []string{"--mode", "dev"}); err != nil {
		errs = append(errs, err)
	}

	if len(errs) != 5 {
		t.Fatalf("expected 5 populated error cases to check, got %d", len(errs))
	}
	for i, err := range errs {
		if _, ok := err.(exitCoder); ok {
			t.Errorf("case %d: error %v carries a custom ExitCode() — composition must stay on the flat os.Exit(1) convention", i, err)
		}
	}
}

// compositionProcessFixtureConfig composes two long-lived native/process children so a real
// `down` after `up` has something to observably tear down: teardown's IsUp check
// (composition_orchestrator.go's teardown) only reports a child up while its process is
// actually alive (ProcessPlugin.Status backed by a live PID), unlike the script runner
// (fire-and-forget, Status always empty — see lifecycle.ScriptPlugin.Status) or a command
// that exits immediately (this same package's compositionFixtureConfig's `run: "true"`,
// which is gone from its pidfile by the time a follow-up down would query it).
