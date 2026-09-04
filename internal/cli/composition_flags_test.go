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
// that a composition invocation carrying only these flags reaches the TASK-291 stub rather
// than being rejected as an unsupported or out-of-scope flag.
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

	t.Run("both flags together reach the TASK-291 stub, not a flag rejection", func(t *testing.T) {
		el := planEnv(config.NewEnvironment(nil, "", ""))
		err := runCompositionUp(compositionFixtureConfigLoaded(t), el, "release", []string{"--no-wait", "--no-rollback"})
		if err == nil {
			t.Fatal("expected the TASK-291 not-implemented stub error, got nil")
		}
		if !strings.Contains(err.Error(), "TASK-291") {
			t.Fatalf("error = %q, want it to name the TASK-291 gap (proves flags propagated through validation instead of being rejected)", err)
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

	t.Run("nothing starts on rejection", func(t *testing.T) {
		el := planEnv(config.NewEnvironment(nil, "", ""))
		c := compositionFixtureConfigLoaded(t)
		if err := runCompositionDown(c, el, "release", []string{"--purge"}); err == nil {
			t.Fatal("expected rejection, got nil")
		}
		if _, statErr := os.Stat(filepath.Join(c.FileDir(), "build-order")); !os.IsNotExist(statErr) {
			t.Fatal("a child appears to have run despite the missing --project scope")
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

	t.Run("status text output", func(t *testing.T) {
		c := loadTestConfig(t, compositionFixtureConfig)
		el := planEnv(config.NewEnvironment(nil, c.FileDir(), c.FileDir()))

		var statusErr error
		out := captureStdout(t, func() {
			statusErr = runCompositionStatus(c, el, "release")
		})
		if statusErr != nil {
			t.Fatalf("runCompositionStatus: %v", statusErr)
		}
		if !strings.Contains(out, "a-plan") || !strings.Contains(out, "b-plan") {
			t.Errorf("text status missing both children:\n%s", out)
		}
		if !strings.Contains(out, "outcome: up") {
			t.Errorf("text status missing outcome:\n%s", out)
		}
	})

	t.Run("status --json reuses TASK-260 §5.3's shape", func(t *testing.T) {
		c := loadTestConfig(t, compositionFixtureConfig)
		el := planEnv(config.NewEnvironment(nil, c.FileDir(), c.FileDir()))

		oldJSON := jsonOutput
		jsonOutput = true
		defer func() { jsonOutput = oldJSON }()

		var statusErr error
		out := captureStdout(t, func() {
			statusErr = runCompositionStatus(c, el, "release")
		})
		if statusErr != nil {
			t.Fatalf("runCompositionStatus --json: %v", statusErr)
		}

		var report compositionStatusReport
		if err := json.Unmarshal([]byte(out), &report); err != nil {
			t.Fatalf("json.Unmarshal(%q): %v", out, err)
		}
		if report.Plan != "release" {
			t.Errorf("plan = %q, want release", report.Plan)
		}
		if report.Kind != "composition" {
			t.Errorf("kind = %q, want composition", report.Kind)
		}
		if report.Outcome != "up" {
			t.Errorf("outcome = %q, want up", report.Outcome)
		}
		if report.DvaVersion != config.Version {
			t.Errorf("dva_version = %q, want %q", report.DvaVersion, config.Version)
		}
		if len(report.Children) != 2 {
			t.Fatalf("children = %d, want 2", len(report.Children))
		}
		wantProjects := map[string]string{"a-plan": "a-plan", "b-plan": "b-plan"}
		for _, child := range report.Children {
			if wantPlan, ok := wantProjects[child.Project]; !ok || child.Plan != wantPlan {
				t.Errorf("unexpected child %+v", child)
			}
			if child.State != "up" {
				t.Errorf("child %s state = %q, want up", child.Project, child.State)
			}
		}
		if report.Rollback.Attempted == nil || report.Rollback.Succeeded == nil || report.Rollback.Failed == nil {
			t.Errorf("rollback report fields must be present-but-empty arrays, got %+v", report.Rollback)
		}
	})
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
	if err := runCompositionUp(c, el, "release", nil); err != nil {
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
