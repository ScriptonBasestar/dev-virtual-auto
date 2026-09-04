//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/lifecycle"
)

// These fixtures reproduce, end-to-end through the real internal/config and
// internal/lifecycle APIs (the same layer internal/cli's composition_flags.go wraps),
// the four scenarios TASK-260 §5 requires: two-project success (§5 scenario 1), a
// composition-of-composition rejected before any child starts (§5 scenario 2, §3.3), a
// rollback failure that preserves the original error (§5 scenario 3, §5.2), and a
// resumable partial state recovered by a plain re-invocation (§5 scenario 4, §5.4).
//
// They drive lifecycle.ResolveCompositionPlan + lifecycle.NewCompositionOrchestrator +
// lifecycle.PlanChildExecutor directly rather than spawning a real `dva` subprocess: the
// CLI layer's own flag-string parsing is already covered by
// internal/cli/composition_flags_test.go, and docker/kubectl are not available in this
// environment, so the script runner is used throughout for controllable, real child
// process execution.

func compositionWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// compositionEnvironmentFor builds the Environment callback lifecycle.PlanChildExecutor
// requires: each composed child runs against its own owning project's directory
// (TASK-260 §4.2 — the root's CWD never flows into a child), constructed exactly the way
// NewPlanOrchestrator requires (env.CfgDir() == owner.FileDir()).
func compositionEnvironmentFor(root *config.Config) func(child *lifecycle.ExecutionPlan) (*config.Config, *config.Environment, error) {
	return func(child *lifecycle.ExecutionPlan) (*config.Config, *config.Environment, error) {
		owner := child.OwnerConfig(root)
		if owner == nil {
			return nil, nil, fmt.Errorf("child %q: owner config is nil", child.Name)
		}
		return owner, config.NewEnvironment(nil, owner.FileDir(), owner.FileDir()), nil
	}
}

// TestCompositionFixtureTwoProjectSuccess is TASK-260 §3's accepted fixture (an api/web
// pair, each with an imported "deploy" plan, composed by a root "release" plan with a
// depends_on) run through §5 scenario 1: `up` succeeds, waves execute in declared
// depends_on order, and both children report state "up".
func TestCompositionFixtureTwoProjectSuccess(t *testing.T) {
	root := t.TempDir()

	compositionWriteFile(t, filepath.Join(root, "api", config.FileName), `version: "0.1.0"
stack:
  api-server:
    default_runner: script
    runners:
      script:
        up: sh -c 'echo api >> ../order-log'
plans:
  deploy:
    entries:
      - name: api-server
`)
	compositionWriteFile(t, filepath.Join(root, "web", config.FileName), `version: "0.1.0"
stack:
  web-server:
    default_runner: script
    runners:
      script:
        up: sh -c 'echo web >> ../order-log'
plans:
  deploy:
    entries:
      - name: web-server
`)
	compositionWriteFile(t, filepath.Join(root, config.FileName), `version: "0.1.0"
subprojects:
  api:
    path: api
    import:
      plans:
        - name: deploy
  web:
    path: web
    import:
      plans:
        - name: deploy
plans:
  release:
    composes:
      - plan: api/deploy
        order: 0
      - plan: web/deploy
        order: 1
        depends_on: ["api/deploy"]
`)

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	compPlan, err := lifecycle.ResolveCompositionPlan(cfg, "release")
	if err != nil {
		t.Fatalf("ResolveCompositionPlan: %v", err)
	}

	exec := &lifecycle.PlanChildExecutor{Environment: compositionEnvironmentFor(cfg)}
	orch, err := lifecycle.NewCompositionOrchestrator(compPlan, exec)
	if err != nil {
		t.Fatalf("NewCompositionOrchestrator: %v", err)
	}

	report, err := orch.Up(context.Background(), lifecycle.CompositionUpOptions{})
	if err != nil {
		t.Fatalf("composition Up failed: %v", err)
	}
	if report.Outcome != lifecycle.CompositionOutcomeUp {
		t.Errorf("outcome = %q, want %q", report.Outcome, lifecycle.CompositionOutcomeUp)
	}
	if len(report.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(report.Children))
	}
	for _, child := range report.Children {
		if child.State != lifecycle.ChildStateUp {
			t.Errorf("child %s/%s state = %q, want %q", child.Project, child.Plan, child.State, lifecycle.ChildStateUp)
		}
	}

	orderLog, readErr := os.ReadFile(filepath.Join(root, "order-log"))
	if readErr != nil {
		t.Fatalf("read order-log: %v", readErr)
	}
	apiIdx := strings.Index(string(orderLog), "api")
	webIdx := strings.Index(string(orderLog), "web")
	if apiIdx < 0 || webIdx < 0 || apiIdx > webIdx {
		t.Errorf("order-log = %q, want api before web (wave 0 before wave 1, declared depends_on order)", orderLog)
	}
}

// TestCompositionFixtureRejectedCycle is TASK-260 §3's rejected fixture (a composition
// plan composing another composition plan) run through §5 scenario 2: config.Load rejects
// it — validateCompositionPlans runs inside config.Load's pipeline — before any child
// plan is resolved or started, so zero children are ever touched.
func TestCompositionFixtureRejectedCycle(t *testing.T) {
	root := t.TempDir()

	compositionWriteFile(t, filepath.Join(root, "api", config.FileName), `version: "0.1.0"
stack:
  api-server:
    default_runner: script
    runners:
      script:
        up: touch api-up
plans:
  deploy:
    entries:
      - name: api-server
`)
	compositionWriteFile(t, filepath.Join(root, "web", config.FileName), `version: "0.1.0"
stack:
  web-server:
    default_runner: script
    runners:
      script:
        up: touch web-up
plans:
  deploy:
    entries:
      - name: web-server
`)
	compositionWriteFile(t, filepath.Join(root, config.FileName), `version: "0.1.0"
subprojects:
  api:
    path: api
    import:
      plans:
        - name: deploy
  web:
    path: web
    import:
      plans:
        - name: deploy
plans:
  release:
    composes:
      - plan: api/deploy
        order: 0
      - plan: web/deploy
        order: 1
  release-all:
    composes:
      - plan: release
`)

	_, err := config.Load(root)
	if err == nil {
		t.Fatal("expected config.Load to reject release-all composing release (a composition plan)")
	}
	if !strings.Contains(err.Error(), "composition plan") || !strings.Contains(err.Error(), "cannot compose another composition") {
		t.Errorf("error = %q, want it to name the rejection reason (TASK-260 §3.3)", err.Error())
	}

	for _, marker := range []string{filepath.Join(root, "api", "api-up"), filepath.Join(root, "web", "web-up")} {
		if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
			t.Errorf("marker %s exists — a child was started despite the rejected composition (want zero children started)", marker)
		}
	}
}

// TestCompositionFixtureRollbackFailurePreservesError is §5 scenario 3 (TASK-260 §5.2):
// api/deploy succeeds, web/deploy fails, and the automatic rollback of api/deploy also
// fails — a rollback failure on a *different* child than the one whose up originally
// failed. The primary error must stay web-server's original failure message unchanged
// ("original-error preservation"), and api/deploy must be reported rollback_failed.
//
// Drives the real lifecycle.PlanChildExecutor (TASK-295 fixed Orchestrator.Down to
// surface per-entry teardown failures instead of warning and swallowing them, so this no
// longer needs a test-local Down override to observe the rollback failure).
func TestCompositionFixtureRollbackFailurePreservesError(t *testing.T) {
	cfg, compPlan := rollbackFailureFixture(t)

	exec := &lifecycle.PlanChildExecutor{Environment: compositionEnvironmentFor(cfg)}
	orch, err := lifecycle.NewCompositionOrchestrator(compPlan, exec)
	if err != nil {
		t.Fatalf("NewCompositionOrchestrator: %v", err)
	}

	report, upErr := orch.Up(context.Background(), lifecycle.CompositionUpOptions{})
	if upErr == nil {
		t.Fatal("expected composition Up to fail: web-server always fails")
	}
	if !strings.Contains(upErr.Error(), `entry "web-server" up failed`) {
		t.Errorf("primary error = %q, want it to name web-server's original up failure unchanged", upErr.Error())
	}

	compErr, ok := asCompositionError(upErr)
	if !ok {
		t.Fatalf("error is not a *lifecycle.CompositionError: %T", upErr)
	}
	if compErr.Error() != upErr.Error() {
		t.Errorf("CompositionError.Error() = %q, want it byte-identical to the returned error", compErr.Error())
	}

	if report.Outcome != lifecycle.CompositionOutcomeFailed {
		t.Errorf("outcome = %q, want %q", report.Outcome, lifecycle.CompositionOutcomeFailed)
	}
	if len(report.Rollback.Failed) != 1 || report.Rollback.Failed[0] != "api/deploy" {
		t.Errorf("rollback.failed = %v, want [\"api/deploy\"]", report.Rollback.Failed)
	}

	var apiState, webState string
	for _, child := range report.Children {
		switch child.Plan {
		case "deploy":
			if child.Project == "api" {
				apiState = child.State
			} else if child.Project == "web" {
				webState = child.State
			}
		}
	}
	if apiState != lifecycle.ChildStateRollbackFailed {
		t.Errorf("api/deploy state = %q, want %q", apiState, lifecycle.ChildStateRollbackFailed)
	}
	if webState != lifecycle.ChildStateFailed {
		t.Errorf("web/deploy state = %q, want %q", webState, lifecycle.ChildStateFailed)
	}
}

// TestCompositionFixtureResumesAfterRollbackFailure is §5 scenario 4 (TASK-260 §5.4):
// immediately after scenario 3 leaves api/deploy still actually up (its rollback down
// failed) and web/deploy never came up, a plain re-invocation of the same composition
// command — no new flag, no persisted state file — resolves fresh and completes: api's
// idempotent up treats "already up" as a pass, and only web/deploy is genuinely retried.
func TestCompositionFixtureResumesAfterRollbackFailure(t *testing.T) {
	cfg, compPlan := rollbackFailureFixture(t)
	root := cfg.FileDir()

	exec := &lifecycle.PlanChildExecutor{Environment: compositionEnvironmentFor(cfg)}
	orch, err := lifecycle.NewCompositionOrchestrator(compPlan, exec)
	if err != nil {
		t.Fatalf("NewCompositionOrchestrator: %v", err)
	}

	if _, upErr := orch.Up(context.Background(), lifecycle.CompositionUpOptions{}); upErr == nil {
		t.Fatal("expected the first attempt to fail and leave a rollback failure, per scenario 3")
	}
	if _, statErr := os.Stat(filepath.Join(root, "api", "api-up")); statErr != nil {
		t.Fatalf("expected api-server's up marker to still be present (rollback down failed): %v", statErr)
	}

	// web-server's script fails exactly once (scenario 3's original failure) and
	// succeeds on every later invocation — simulating a transient failure now cleared,
	// without any new CLI flag or persisted state, per §5.4.
	if _, statErr := os.Stat(filepath.Join(root, "web", "web-attempted")); statErr != nil {
		t.Fatalf("expected web-server's up to have recorded its first attempt: %v", statErr)
	}

	// A plain re-invocation re-resolves the composition plan from scratch (§5.4's "root
	// re-resolves and asks each child again" — no cached plan or state is reused).
	cfg2, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load (resume): %v", err)
	}
	compPlan2, err := lifecycle.ResolveCompositionPlan(cfg2, "release")
	if err != nil {
		t.Fatalf("ResolveCompositionPlan (resume): %v", err)
	}
	exec2 := &lifecycle.PlanChildExecutor{Environment: compositionEnvironmentFor(cfg2)}
	orch2, err := lifecycle.NewCompositionOrchestrator(compPlan2, exec2)
	if err != nil {
		t.Fatalf("NewCompositionOrchestrator (resume): %v", err)
	}

	report, upErr := orch2.Up(context.Background(), lifecycle.CompositionUpOptions{})
	if upErr != nil {
		t.Fatalf("resumed composition Up failed: %v", upErr)
	}
	if report.Outcome != lifecycle.CompositionOutcomeUp {
		t.Errorf("outcome = %q, want %q", report.Outcome, lifecycle.CompositionOutcomeUp)
	}
	for _, child := range report.Children {
		if child.State != lifecycle.ChildStateUp {
			t.Errorf("child %s/%s state = %q, want %q after resume", child.Project, child.Plan, child.State, lifecycle.ChildStateUp)
		}
	}
}

// rollbackFailureFixture builds and resolves the shared fixture scenarios 3 and 4 need:
// api/deploy succeeds and its down fails; web/deploy fails once (recording the attempt)
// then succeeds on any later invocation, so scenario 4 can resume it without a second
// fixture. depends_on forces api/deploy into wave 0 and web/deploy into wave 1, matching
// TASK-260 §3's accepted fixture shape.
func rollbackFailureFixture(t *testing.T) (*config.Config, *lifecycle.CompositionPlan) {
	t.Helper()
	root := t.TempDir()

	compositionWriteFile(t, filepath.Join(root, "api", config.FileName), `version: "0.1.0"
stack:
  api-server:
    default_runner: script
    runners:
      script:
        up: touch api-up
        down: exit 1
plans:
  deploy:
    entries:
      - name: api-server
`)
	compositionWriteFile(t, filepath.Join(root, "web", config.FileName), `version: "0.1.0"
stack:
  web-server:
    default_runner: script
    runners:
      script:
        up: sh -c 'test -f web-attempted && exit 0 || { touch web-attempted; exit 1; }'
plans:
  deploy:
    entries:
      - name: web-server
`)
	compositionWriteFile(t, filepath.Join(root, config.FileName), `version: "0.1.0"
subprojects:
  api:
    path: api
    import:
      plans:
        - name: deploy
  web:
    path: web
    import:
      plans:
        - name: deploy
plans:
  release:
    composes:
      - plan: api/deploy
        order: 0
      - plan: web/deploy
        order: 1
        depends_on: ["api/deploy"]
`)

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	compPlan, err := lifecycle.ResolveCompositionPlan(cfg, "release")
	if err != nil {
		t.Fatalf("ResolveCompositionPlan: %v", err)
	}
	return cfg, compPlan
}

func asCompositionError(err error) (*lifecycle.CompositionError, bool) {
	compErr, ok := err.(*lifecycle.CompositionError)
	return compErr, ok
}
