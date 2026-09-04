package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/lifecycle"
)

const compositionProcessFixtureConfig = `version: "0.1.0"
stack:
  svc-a:
    default_runner: native
    runners:
      native:
        run: sleep 999
  svc-b:
    default_runner: native
    runners:
      native:
        run: sleep 999
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

// killLeftoverCompositionProcess is a t.Cleanup safety net for compositionProcessFixtureConfig
// tests: it best-effort kills whatever a pid file still names, in case the test fails before
// its own runCompositionDown call reaps the background `sleep 999`.
func killLeftoverCompositionProcess(t *testing.T, fileDir, name string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fileDir, config.DotDirName, config.PidsDirName, name+".pid"))
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return
	}
	if proc, err := os.FindProcess(pid); err == nil {
		_ = proc.Kill()
	}
}

// TestCompositionUpDownExecuteRealOrchestrator proves up/down on a composition plan now run
// TASK-291's real orchestrator instead of the former TASK-291-not-implemented stub: up starts
// every composed child in wave order (observable via a live PID file per child), and down
// actually tears every up child down (PID files removed), not just validates flags and
// returns a canned error.
func TestCompositionUpDownExecuteRealOrchestrator(t *testing.T) {
	c := loadTestConfig(t, compositionProcessFixtureConfig)
	el := planEnv(config.NewEnvironment(nil, c.FileDir(), c.FileDir()))
	pidPath := func(name string) string {
		return filepath.Join(c.FileDir(), config.DotDirName, config.PidsDirName, name+".pid")
	}
	t.Cleanup(func() {
		killLeftoverCompositionProcess(t, c.FileDir(), "svc-a")
		killLeftoverCompositionProcess(t, c.FileDir(), "svc-b")
	})

	if err := runCompositionUp(c, el, "release", nil); err != nil {
		t.Fatalf("runCompositionUp: %v", err)
	}
	for _, name := range []string{"svc-a", "svc-b"} {
		if _, err := os.Stat(pidPath(name)); err != nil {
			t.Fatalf("expected %s to have a pid file after up: %v", name, err)
		}
	}

	// --project a-plan scopes which child's teardown is destructive (TASK-260 §4.4's worked
	// example), not which children come down: composition_orchestrator.go's teardown walks
	// every up child in LIFO order regardless of scope. Both children are expected to be torn
	// down here — compositionDestructiveOptions (unit-tested below) is what actually proves
	// the scoping itself.
	if err := runCompositionDown(c, el, "release", []string{"--purge", "--project", "a-plan", "--force"}); err != nil {
		t.Fatalf("runCompositionDown: %v", err)
	}
	for _, name := range []string{"svc-a", "svc-b"} {
		if _, err := os.Stat(pidPath(name)); !os.IsNotExist(err) {
			t.Fatalf("expected %s's pid file to be removed after down, stat err = %v", name, err)
		}
	}
}

// TestCompositionDestructiveOptionsScopePerChild unit-tests compositionDestructiveOptions
// directly: TASK-260 §4.4's worked example ("api/deploy만 volume까지 제거하고 web/deploy는
// 건드리지 않는다") requires that only the --project-scoped child's ChildDownOptions carries
// Volumes/RemoveImages — every other composed child that comes down in the same call must get
// the zero value. This is the part of "--project scoping" that composition_orchestrator.go's
// frozen teardown actually leaves to the caller to narrow; which children are torn down at all
// is not scoped (see TestCompositionUpDownExecuteRealOrchestrator).
func TestCompositionDestructiveOptionsScopePerChild(t *testing.T) {
	_, comp := compositionFixture(t)

	t.Run("no scope means no destructive options for anyone", func(t *testing.T) {
		flags, err := validateCompositionFlagScope(comp, "release", "down", nil)
		if err != nil {
			t.Fatalf("validateCompositionFlagScope: %v", err)
		}
		if got := compositionDestructiveOptions(flags); got != nil {
			t.Fatalf("compositionDestructiveOptions = %v, want nil", got)
		}
	})

	t.Run("--volumes --project a-plan scopes only a-plan", func(t *testing.T) {
		flags, err := validateCompositionFlagScope(comp, "release", "down", []string{"--volumes", "--project", "a-plan"})
		if err != nil {
			t.Fatalf("validateCompositionFlagScope: %v", err)
		}
		got := compositionDestructiveOptions(flags)
		want := map[string]lifecycle.ChildDownOptions{"a-plan": {Volumes: true, RemoveImages: false}}
		if len(got) != len(want) || got["a-plan"] != want["a-plan"] {
			t.Fatalf("compositionDestructiveOptions = %+v, want %+v", got, want)
		}
		if _, ok := got["b-plan"]; ok {
			t.Fatalf("compositionDestructiveOptions scoped b-plan too: %+v", got)
		}
	})

	t.Run("--purge implies both volumes and image removal for the scoped child only", func(t *testing.T) {
		flags, err := validateCompositionFlagScope(comp, "release", "down", []string{"--purge", "--project", "b-plan"})
		if err != nil {
			t.Fatalf("validateCompositionFlagScope: %v", err)
		}
		got := compositionDestructiveOptions(flags)
		want := map[string]lifecycle.ChildDownOptions{"b-plan": {Volumes: true, RemoveImages: true}}
		if len(got) != len(want) || got["b-plan"] != want["b-plan"] {
			t.Fatalf("compositionDestructiveOptions = %+v, want %+v", got, want)
		}
		if _, ok := got["a-plan"]; ok {
			t.Fatalf("compositionDestructiveOptions scoped a-plan too: %+v", got)
		}
	})
}

// compositionRollbackFixtureConfig composes a child that always succeeds (svc-good, wave 0)
// ahead of one that always fails (svc-bad, wave 1), so composition up is guaranteed to fail
// after svc-good has already come up — the scenario TASK-260 §5.1's automatic LIFO rollback
// (and --no-rollback's opt-out, §4.4) exists for. Both use the script runner: the rollback
// path (CompositionOrchestrator.Up's failure branch) calls exec.Down unconditionally for every
// succeeded child, with no IsUp gate — unlike the teardown path a real `down` walks — so a
// fire-and-forget down script is enough to observe it, without a long-lived process.
const compositionRollbackFixtureConfig = `version: "0.1.0"
stack:
  svc-good:
    default_runner: script
    runners:
      script:
        up: touch up-good
        down: echo good >> rollback-order
  svc-bad:
    default_runner: script
    runners:
      script:
        up: exit 1
plans:
  good-plan:
    entries:
      - name: svc-good
  bad-plan:
    entries:
      - name: svc-bad
  release:
    composes:
      - plan: good-plan
        order: 0
      - plan: bad-plan
        order: 1
        depends_on: ["good-plan"]
`

// TestCompositionUpRollbackAndNoRollback proves composition up's automatic LIFO rollback rolls
// back an already-succeeded child on a later child's failure by default, that --no-rollback
// leaves it up instead (TASK-260 §4.4, §5.1), and that both outcomes report as a plain error
// (exit code reflects Outcome == "failed", TASK-260 §5.6 — no new exit-code taxonomy).
func TestCompositionUpRollbackAndNoRollback(t *testing.T) {
	t.Run("default rolls back the already-succeeded child", func(t *testing.T) {
		c := loadTestConfig(t, compositionRollbackFixtureConfig)
		el := planEnv(config.NewEnvironment(nil, c.FileDir(), c.FileDir()))

		err := runCompositionUp(c, el, "release", nil)
		if err == nil {
			t.Fatal("expected up to fail: svc-bad always fails")
		}
		data, readErr := os.ReadFile(filepath.Join(c.FileDir(), "rollback-order"))
		if readErr != nil {
			t.Fatalf("expected svc-good's rollback down to have run: %v", readErr)
		}
		if !strings.Contains(string(data), "good") {
			t.Fatalf("rollback-order = %q, want it to record svc-good's rollback", data)
		}
	})

	t.Run("--no-rollback leaves the already-succeeded child up", func(t *testing.T) {
		c := loadTestConfig(t, compositionRollbackFixtureConfig)
		el := planEnv(config.NewEnvironment(nil, c.FileDir(), c.FileDir()))

		err := runCompositionUp(c, el, "release", []string{"--no-rollback"})
		if err == nil {
			t.Fatal("expected up to fail: svc-bad always fails")
		}
		if _, statErr := os.Stat(filepath.Join(c.FileDir(), "rollback-order")); !os.IsNotExist(statErr) {
			t.Fatalf("--no-rollback must not roll back svc-good, but rollback-order exists (stat err = %v)", statErr)
		}
		if _, statErr := os.Stat(filepath.Join(c.FileDir(), "up-good")); statErr != nil {
			t.Fatalf("expected svc-good's up marker to still be present under --no-rollback: %v", statErr)
		}
	})
}

// TestCompositionStatusReportsFailedChild closes a coverage gap noted in review: the "failed"
// per-child state and outcome (composition_flags.go's runCompositionStatus, delegating to
// lifecycle.CompositionOrchestrator.Status, TASK-260 §5.3) had no test, because every prior
// fixture that made a child unrunnable made the whole root config fail to load first — never
// reaching status classification at all. A composed child owned by a SEPARATE subproject config
// can be unrunnable on its own (a required env_file that does not exist) while the root config,
// the composition resolve, and the sibling local child all load and query cleanly — the one path
// that reaches compositionChildEnvironment's `resolvePlanRuntime`-incomplete branch without the
// fixture ever failing to load.
func TestCompositionStatusReportsFailedChild(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, config.FileName), `version: "0.1.0"
stack:
  local:
    default_runner: script
    runners:
      script:
        up: touch up-local
plans:
  local-plan:
    entries:
      - name: local
  release:
    composes:
      - plan: local-plan
        order: 0
      - plan: backend/deploy
        order: 1
subprojects:
  backend:
    path: backend
    import:
      plans:
        - name: deploy
`)
	writeFile(t, filepath.Join(root, "backend", config.FileName), `version: "0.1.0"
env_file:
  files: missing.env
  required: true
stack:
  api:
    default_runner: script
    runners:
      script:
        up: touch up-api
plans:
  deploy:
    entries:
      - name: api
`)

	c, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load: %v (backend's missing required env_file must not fail the root load)", err)
	}
	el := planEnv(config.NewEnvironment(nil, c.FileDir(), c.FileDir()))

	var statusErr error
	out := captureStdout(t, func() {
		statusErr = runCompositionStatus(c, el, "release")
	})
	if statusErr == nil {
		t.Fatal("expected runCompositionStatus to report an error: backend/deploy is unrunnable")
	}
	if !strings.Contains(out, "outcome: failed") {
		t.Errorf("text status outcome must be failed:\n%s", out)
	}

	oldJSON := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = oldJSON }()

	out = captureStdout(t, func() {
		statusErr = runCompositionStatus(c, el, "release")
	})
	if statusErr == nil {
		t.Fatal("expected runCompositionStatus --json to report an error: backend/deploy is unrunnable")
	}

	var report lifecycle.CompositionReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", out, err)
	}
	if report.Outcome != "failed" {
		t.Errorf("outcome = %q, want failed", report.Outcome)
	}
	if len(report.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(report.Children))
	}
	for _, child := range report.Children {
		switch child.Plan {
		case "local-plan":
			if child.State != "not_started" {
				t.Errorf("local-plan state = %q, want not_started (it was never up, but it IS runnable)", child.State)
			}
		case "deploy":
			if child.State != "failed" {
				t.Errorf("backend/deploy state = %q, want failed", child.State)
			}
			if child.Error == "" {
				t.Errorf("backend/deploy must carry a non-empty error explaining why it is unrunnable")
			}
			if child.Project != "backend" {
				t.Errorf("backend/deploy project = %q, want backend", child.Project)
			}
		default:
			t.Errorf("unexpected child %+v", child)
		}
	}
}

// TestCompositionUpDryRunPerformsNoRealExecution closes B1 from review: --dry-run is consumed
// by parseDvaFlags (compose.go) into the package-level dryRun global before
// validateCompositionFlagScope ever sees extraArgs, so newCompositionExecutor is the only place
// left that can honor it — it must thread dryRun into both PlanChildExecutor instances it
// builds. Uses compositionProcessFixtureConfig so a real up would be observable (a live PID
// file per child); dry-run must leave neither child started.
func TestCompositionUpDryRunPerformsNoRealExecution(t *testing.T) {
	c := loadTestConfig(t, compositionProcessFixtureConfig)
	el := planEnv(config.NewEnvironment(nil, c.FileDir(), c.FileDir()))
	pidPath := func(name string) string {
		return filepath.Join(c.FileDir(), config.DotDirName, config.PidsDirName, name+".pid")
	}
	t.Cleanup(func() {
		killLeftoverCompositionProcess(t, c.FileDir(), "svc-a")
		killLeftoverCompositionProcess(t, c.FileDir(), "svc-b")
	})

	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	if err := runCompositionUp(c, el, "release", nil); err != nil {
		t.Fatalf("runCompositionUp with dryRun=true: %v", err)
	}
	for _, name := range []string{"svc-a", "svc-b"} {
		if _, err := os.Stat(pidPath(name)); !os.IsNotExist(err) {
			t.Fatalf("expected %s to have no pid file under --dry-run, stat err = %v", name, err)
		}
	}
}

// TestCompositionDownDryRunPerformsNoRealTeardown closes the reviewer's specifically-reproduced
// dangerous case: `dva down <composition> --project <child> --purge --force --dry-run` must not
// perform any real teardown. Brings the composition up for real first (dryRun=false), then
// attempts the destructive down under dryRun=true and asserts both children are still alive
// (pid files untouched) and no confirmation prompt is reached (--force alone already waives it,
// but --dry-run must independently waive it too per the runPlanDown precedent this mirrors).
func TestCompositionDownDryRunPerformsNoRealTeardown(t *testing.T) {
	c := loadTestConfig(t, compositionProcessFixtureConfig)
	el := planEnv(config.NewEnvironment(nil, c.FileDir(), c.FileDir()))
	pidPath := func(name string) string {
		return filepath.Join(c.FileDir(), config.DotDirName, config.PidsDirName, name+".pid")
	}
	t.Cleanup(func() {
		killLeftoverCompositionProcess(t, c.FileDir(), "svc-a")
		killLeftoverCompositionProcess(t, c.FileDir(), "svc-b")
	})

	if err := runCompositionUp(c, el, "release", nil); err != nil {
		t.Fatalf("runCompositionUp: %v", err)
	}
	for _, name := range []string{"svc-a", "svc-b"} {
		if _, err := os.Stat(pidPath(name)); err != nil {
			t.Fatalf("expected %s to have a pid file after up: %v", name, err)
		}
	}

	oldDryRun := dryRun
	dryRun = true
	defer func() { dryRun = oldDryRun }()

	if err := runCompositionDown(c, el, "release", []string{"--purge", "--project", "a-plan", "--force"}); err != nil {
		t.Fatalf("runCompositionDown with dryRun=true: %v", err)
	}
	for _, name := range []string{"svc-a", "svc-b"} {
		if _, err := os.Stat(pidPath(name)); err != nil {
			t.Fatalf("expected %s's pid file to remain after --dry-run down: %v", name, err)
		}
	}
}
