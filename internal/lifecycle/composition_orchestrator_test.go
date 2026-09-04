package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// fakeChildExecutor records the exact sequence of child calls a composition makes and
// detects any overlap between them. Composition promises sequential execution
// (TASK-260 §4.1), so the recorded order is the contract under test.
type fakeChildExecutor struct {
	mu sync.Mutex

	calls []string

	inFlight    int
	maxInFlight int

	upErr    map[string]error
	readyErr map[string]error
	downErr  map[string]error
	stopErr  map[string]error

	live    map[string]bool
	liveErr map[string]error
}

func newFakeChildExecutor() *fakeChildExecutor {
	return &fakeChildExecutor{
		upErr:    map[string]error{},
		readyErr: map[string]error{},
		downErr:  map[string]error{},
		stopErr:  map[string]error{},
		live:     map[string]bool{},
		liveErr:  map[string]error{},
	}
}

func (f *fakeChildExecutor) enter(call string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
	f.inFlight++
	if f.inFlight > f.maxInFlight {
		f.maxInFlight = f.inFlight
	}
}

func (f *fakeChildExecutor) leave() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inFlight--
}

func (f *fakeChildExecutor) Up(_ context.Context, child *ExecutionPlan) error {
	f.enter("up:" + child.Name)
	defer f.leave()
	if err := f.upErr[child.Name]; err != nil {
		return err
	}
	f.mu.Lock()
	f.live[child.Name] = true
	f.mu.Unlock()
	return nil
}

func (f *fakeChildExecutor) WaitReady(_ context.Context, child *ExecutionPlan) error {
	f.enter("wait:" + child.Name)
	defer f.leave()
	return f.readyErr[child.Name]
}

func (f *fakeChildExecutor) Down(_ context.Context, child *ExecutionPlan, opts ChildDownOptions) error {
	label := "down:" + child.Name
	if opts.Volumes {
		label += ":volumes"
	}
	if opts.RemoveImages {
		label += ":images"
	}
	f.enter(label)
	defer f.leave()
	if err := f.downErr[child.Name]; err != nil {
		return err
	}
	f.mu.Lock()
	f.live[child.Name] = false
	f.mu.Unlock()
	return nil
}

func (f *fakeChildExecutor) Stop(_ context.Context, child *ExecutionPlan) error {
	f.enter("stop:" + child.Name)
	defer f.leave()
	return f.stopErr[child.Name]
}

func (f *fakeChildExecutor) IsUp(_ context.Context, child *ExecutionPlan) (bool, error) {
	f.enter("isup:" + child.Name)
	defer f.leave()
	if err := f.liveErr[child.Name]; err != nil {
		return false, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.live[child.Name], nil
}

func (f *fakeChildExecutor) recorded() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

// resolveTestComposition writes a root dva.yml with three leaf plans (a-plan and b-plan
// share wave 0, c-plan depends on b-plan) and resolves its composition plan.
func resolveTestComposition(t *testing.T) *CompositionPlan {
	t.Helper()
	dir := t.TempDir()
	writeImportedPlanConfig(t, dir, `
version: "0.1.0"
stack:
  a:
    default_runner: process
    runners:
      process:
        command: echo A
  b:
    default_runner: process
    runners:
      process:
        command: echo B
  c:
    default_runner: process
    runners:
      process:
        command: echo C
plans:
  a-plan:
    entries:
      - name: a
  b-plan:
    entries:
      - name: b
  c-plan:
    entries:
      - name: c
  release:
    composes:
      - plan: a-plan
        order: 0
      - plan: b-plan
        order: 1
      - plan: c-plan
        order: 2
        depends_on: ["b-plan"]
`)
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	plan, err := ResolveCompositionPlan(cfg, "release")
	if err != nil {
		t.Fatalf("ResolveCompositionPlan: %v", err)
	}
	return plan
}

func newTestCompositionOrchestrator(t *testing.T, plan *CompositionPlan, exec CompositionChildExecutor) *CompositionOrchestrator {
	t.Helper()
	orch, err := NewCompositionOrchestrator(plan, exec)
	if err != nil {
		t.Fatalf("NewCompositionOrchestrator: %v", err)
	}
	return orch
}

func requireCalls(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("child calls =\n  %v\nwant\n  %v", got, want)
	}
}

// TestCompositionUpExecutesWavesSequentially covers TASK-260 §4.1 (wave is ordering, not
// concurrency) and §4.5 (the readiness gate sits at the wave boundary, and --no-wait
// removes it).
func TestCompositionUpExecutesWavesSequentially(t *testing.T) {
	plan := resolveTestComposition(t)

	// a-plan and b-plan share wave 0; c-plan depends on b-plan so it is wave 1.
	if plan.Entries[0].Wave != 0 || plan.Entries[1].Wave != 0 || plan.Entries[2].Wave != 1 {
		t.Fatalf("fixture waves = %d,%d,%d, want 0,0,1",
			plan.Entries[0].Wave, plan.Entries[1].Wave, plan.Entries[2].Wave)
	}

	exec := newFakeChildExecutor()
	orch := newTestCompositionOrchestrator(t, plan, exec)

	report, err := orch.Up(context.Background(), CompositionUpOptions{})
	if err != nil {
		t.Fatalf("Up: %v", err)
	}

	// Both wave-0 children run before the wave-0 readiness gate, and the wave-1 child
	// starts only after that gate passes.
	requireCalls(t, exec.recorded(), []string{
		"up:a-plan", "up:b-plan",
		"wait:a-plan", "wait:b-plan",
		"up:c-plan",
		"wait:c-plan",
	})
	if exec.maxInFlight != 1 {
		t.Fatalf("max concurrent child calls = %d, want 1 — composition never runs two children at once", exec.maxInFlight)
	}
	if report.Outcome != CompositionOutcomeUp {
		t.Fatalf("outcome = %q, want %q", report.Outcome, CompositionOutcomeUp)
	}
	for _, child := range report.Children {
		if child.State != ChildStateUp {
			t.Fatalf("child %q state = %q, want %q", child.Plan, child.State, ChildStateUp)
		}
	}

	// --no-wait drops every readiness gate but changes nothing about ordering.
	noWait := newFakeChildExecutor()
	noWaitOrch := newTestCompositionOrchestrator(t, plan, noWait)
	if _, err := noWaitOrch.Up(context.Background(), CompositionUpOptions{NoWait: true}); err != nil {
		t.Fatalf("Up --no-wait: %v", err)
	}
	requireCalls(t, noWait.recorded(), []string{"up:a-plan", "up:b-plan", "up:c-plan"})
}

// TestCompositionUpRollsBackSucceededChildrenOnFailure covers TASK-260 §5.2: no further
// child is started, and every already-succeeded child is torn down in LIFO order with a
// plain `down`.
func TestCompositionUpRollsBackSucceededChildrenOnFailure(t *testing.T) {
	plan := resolveTestComposition(t)

	exec := newFakeChildExecutor()
	boom := errors.New(`entry "c" up failed: exit status 1`)
	exec.upErr["c-plan"] = boom
	orch := newTestCompositionOrchestrator(t, plan, exec)

	report, err := orch.Up(context.Background(), CompositionUpOptions{})
	if err == nil {
		t.Fatal("Up succeeded, want the child failure to surface")
	}

	// b-plan succeeded last, so it goes down first. The rollback `down` carries no
	// volume/image removal (§5.2) — a label with ":volumes" would prove otherwise.
	requireCalls(t, exec.recorded(), []string{
		"up:a-plan", "up:b-plan",
		"wait:a-plan", "wait:b-plan",
		"up:c-plan",
		"down:b-plan", "down:a-plan",
	})

	if report.Outcome != CompositionOutcomeFailed {
		t.Fatalf("outcome = %q, want %q", report.Outcome, CompositionOutcomeFailed)
	}
	wantAttempted := []string{"b-plan", "a-plan"}
	requireCalls(t, report.Rollback.Attempted, wantAttempted)
	requireCalls(t, report.Rollback.Succeeded, wantAttempted)
	if len(report.Rollback.Failed) != 0 {
		t.Fatalf("rollback.failed = %v, want empty", report.Rollback.Failed)
	}

	wantStates := map[string]string{
		"a-plan": ChildStateRolledBack,
		"b-plan": ChildStateRolledBack,
		"c-plan": ChildStateFailed,
	}
	for _, child := range report.Children {
		if got := child.State; got != wantStates[child.Plan] {
			t.Fatalf("child %q state = %q, want %q", child.Plan, got, wantStates[child.Plan])
		}
	}
	if !errors.Is(err, boom) {
		t.Fatalf("returned error = %v, want the original child failure", err)
	}
}

// TestCompositionRollbackFailurePreservesOriginalError covers TASK-260 §5.2's
// "original-error preservation": a rollback `down` that fails must not alter the primary
// error, only add a secondary diagnostic naming the affected child.
func TestCompositionRollbackFailurePreservesOriginalError(t *testing.T) {
	plan := resolveTestComposition(t)

	exec := newFakeChildExecutor()
	primary := errors.New(`entry "c" up failed: container exited with code 1`)
	exec.upErr["c-plan"] = primary
	exec.downErr["a-plan"] = errors.New("compose down: no such project")
	orch := newTestCompositionOrchestrator(t, plan, exec)

	report, err := orch.Up(context.Background(), CompositionUpOptions{})
	if err == nil {
		t.Fatal("Up succeeded, want the child failure to surface")
	}

	if err.Error() != primary.Error() {
		t.Fatalf("primary error = %q, want the original failure unchanged (%q)", err.Error(), primary.Error())
	}
	if report.Error != primary.Error() {
		t.Fatalf("report.error = %q, want %q", report.Error, primary.Error())
	}

	var compErr *CompositionError
	if !errors.As(err, &compErr) {
		t.Fatalf("error type = %T, want *CompositionError", err)
	}
	if len(compErr.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %v, want exactly one secondary diagnostic", compErr.Diagnostics)
	}
	diag := compErr.Diagnostics[0]
	if !strings.Contains(diag, "a-plan") {
		t.Fatalf("diagnostic %q does not name the affected child", diag)
	}
	if !strings.Contains(diag, "no such project") {
		t.Fatalf("diagnostic %q does not carry the rollback failure", diag)
	}
	// The rollback failure must not have leaked into the primary error message.
	if strings.Contains(err.Error(), "no such project") {
		t.Fatalf("primary error %q was rewritten with the rollback failure", err.Error())
	}

	requireCalls(t, report.Rollback.Attempted, []string{"b-plan", "a-plan"})
	requireCalls(t, report.Rollback.Succeeded, []string{"b-plan"})
	requireCalls(t, report.Rollback.Failed, []string{"a-plan"})

	wantStates := map[string]string{
		"a-plan": ChildStateRollbackFailed,
		"b-plan": ChildStateRolledBack,
		"c-plan": ChildStateFailed,
	}
	for _, child := range report.Children {
		if got := child.State; got != wantStates[child.Plan] {
			t.Fatalf("child %q state = %q, want %q", child.Plan, got, wantStates[child.Plan])
		}
	}
}

// TestCompositionNoRollbackFlagSkipsTeardown covers the opt-out approved on 2026-09-04
// (TASK-260 §4.4): succeeded children are left running for inspection.
func TestCompositionNoRollbackFlagSkipsTeardown(t *testing.T) {
	plan := resolveTestComposition(t)

	exec := newFakeChildExecutor()
	primary := errors.New(`entry "c" up failed: exit status 1`)
	exec.upErr["c-plan"] = primary
	orch := newTestCompositionOrchestrator(t, plan, exec)

	report, err := orch.Up(context.Background(), CompositionUpOptions{NoRollback: true})
	if err == nil {
		t.Fatal("Up succeeded, want the child failure to surface")
	}
	if err.Error() != primary.Error() {
		t.Fatalf("primary error = %q, want %q", err.Error(), primary.Error())
	}

	for _, call := range exec.recorded() {
		if strings.HasPrefix(call, "down:") {
			t.Fatalf("--no-rollback still tore a child down: %v", exec.recorded())
		}
	}
	if len(report.Rollback.Attempted) != 0 {
		t.Fatalf("rollback.attempted = %v, want empty under --no-rollback", report.Rollback.Attempted)
	}

	wantStates := map[string]string{
		"a-plan": ChildStateUp,
		"b-plan": ChildStateUp,
		"c-plan": ChildStateFailed,
	}
	for _, child := range report.Children {
		if got := child.State; got != wantStates[child.Plan] {
			t.Fatalf("child %q state = %q, want %q", child.Plan, got, wantStates[child.Plan])
		}
	}
}

// TestCompositionDownIsLIFOAndReentrant covers TASK-260 §4.3 (reverse-wave teardown) and
// §5.4/§6.3 (live child state, not a cached record, decides what a later `down` touches).
func TestCompositionDownIsLIFOAndReentrant(t *testing.T) {
	plan := resolveTestComposition(t)

	// First: a failed up whose rollback of a-plan fails, leaving a-plan up.
	exec := newFakeChildExecutor()
	exec.upErr["c-plan"] = errors.New(`entry "c" up failed: exit status 1`)
	exec.downErr["a-plan"] = errors.New("compose down: transient daemon error")
	orch := newTestCompositionOrchestrator(t, plan, exec)
	if _, err := orch.Up(context.Background(), CompositionUpOptions{}); err == nil {
		t.Fatal("Up succeeded, want the child failure to surface")
	}

	// Now the user runs `dva down release`. Only a-plan is still up, and the orchestrator
	// must find that out by asking each child rather than replaying the run's record.
	exec.mu.Lock()
	exec.calls = nil
	exec.mu.Unlock()
	delete(exec.downErr, "a-plan")

	report, err := orch.Down(context.Background(), CompositionDownOptions{})
	if err != nil {
		t.Fatalf("Down: %v", err)
	}
	requireCalls(t, exec.recorded(), []string{
		"isup:c-plan", "isup:b-plan", "isup:a-plan", "down:a-plan",
	})
	if report.Outcome != CompositionOutcomeDown {
		t.Fatalf("outcome = %q, want %q", report.Outcome, CompositionOutcomeDown)
	}

	// Re-invoking `down` is a no-op: every child now reports itself down.
	exec.mu.Lock()
	exec.calls = nil
	exec.mu.Unlock()
	if _, err := orch.Down(context.Background(), CompositionDownOptions{}); err != nil {
		t.Fatalf("second Down: %v", err)
	}
	requireCalls(t, exec.recorded(), []string{"isup:c-plan", "isup:b-plan", "isup:a-plan"})

	// A full teardown of a fully-up composition walks children in strict reverse-wave
	// order, and `stop` uses the same ordering with the child's resources preserved.
	full := newFakeChildExecutor()
	fullOrch := newTestCompositionOrchestrator(t, plan, full)
	if _, err := fullOrch.Up(context.Background(), CompositionUpOptions{NoWait: true}); err != nil {
		t.Fatalf("Up: %v", err)
	}
	full.mu.Lock()
	full.calls = nil
	full.mu.Unlock()
	if _, err := fullOrch.Down(context.Background(), CompositionDownOptions{}); err != nil {
		t.Fatalf("Down: %v", err)
	}
	requireCalls(t, full.recorded(), []string{
		"isup:c-plan", "down:c-plan",
		"isup:b-plan", "down:b-plan",
		"isup:a-plan", "down:a-plan",
	})

	stop := newFakeChildExecutor()
	stopOrch := newTestCompositionOrchestrator(t, plan, stop)
	if _, err := stopOrch.Up(context.Background(), CompositionUpOptions{NoWait: true}); err != nil {
		t.Fatalf("Up: %v", err)
	}
	stop.mu.Lock()
	stop.calls = nil
	stop.mu.Unlock()
	if _, err := stopOrch.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	requireCalls(t, stop.recorded(), []string{
		"isup:c-plan", "stop:c-plan",
		"isup:b-plan", "stop:b-plan",
		"isup:a-plan", "stop:a-plan",
	})
}

// TestCompositionPartialStateReportShape checks the report against TASK-260 §5.3's
// literal JSON shape on both paths, and pins single-project Up's unchanged
// no-automatic-rollback behavior (this task must not alter it).
func TestCompositionPartialStateReportShape(t *testing.T) {
	dir := t.TempDir()
	for _, project := range []string{"api", "web"} {
		writeImportedPlanConfig(t, filepath.Join(dir, project), fmt.Sprintf(`
version: "0.1.0"
stack:
  %s-server:
    default_runner: process
    runners:
      process:
        command: echo %s
plans:
  deploy:
    entries:
      - name: %s-server
`, project, project, project))
	}
	writeImportedPlanConfig(t, dir, `
version: "0.1.0"
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
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	plan, err := ResolveCompositionPlan(cfg, "release")
	if err != nil {
		t.Fatalf("ResolveCompositionPlan: %v", err)
	}

	// Failure path — the shape TASK-260 §5.3 spells out field by field.
	exec := newFakeChildExecutor()
	primary := errors.New(`entry "web-server" up failed: exit status 1`)
	exec.upErr["web/deploy"] = primary
	orch := newTestCompositionOrchestrator(t, plan, exec)

	failReport, err := orch.Up(context.Background(), CompositionUpOptions{NoWait: true})
	if err == nil {
		t.Fatal("Up succeeded, want the child failure to surface")
	}

	got := decodeReport(t, failReport)
	want := map[string]any{
		"plan":    "release",
		"kind":    "composition",
		"outcome": "failed",
		"children": []any{
			map[string]any{"project": "api", "plan": "deploy", "wave": 0.0, "state": "rolled_back"},
			map[string]any{"project": "web", "plan": "deploy", "wave": 1.0, "state": "failed", "error": primary.Error()},
		},
		"rollback": map[string]any{
			"attempted": []any{"api/deploy"},
			"succeeded": []any{"api/deploy"},
			"failed":    []any{},
		},
		"error": primary.Error(),
	}
	if diff := reportDiff(got, want); diff != "" {
		t.Fatalf("failure report shape mismatch: %s", diff)
	}
	if diff := reportDiff(decodeMap(t, failReport.Map()), want); diff != "" {
		t.Fatalf("failure report Map() shape mismatch: %s", diff)
	}

	// Success path — same shape, reused verbatim (§5.5).
	okExec := newFakeChildExecutor()
	okOrch := newTestCompositionOrchestrator(t, plan, okExec)
	okReport, err := okOrch.Up(context.Background(), CompositionUpOptions{NoWait: true})
	if err != nil {
		t.Fatalf("Up: %v", err)
	}
	wantOK := map[string]any{
		"plan":    "release",
		"kind":    "composition",
		"outcome": "up",
		"children": []any{
			map[string]any{"project": "api", "plan": "deploy", "wave": 0.0, "state": "up"},
			map[string]any{"project": "web", "plan": "deploy", "wave": 1.0, "state": "up"},
		},
		"rollback": map[string]any{"attempted": []any{}, "succeeded": []any{}, "failed": []any{}},
	}
	if diff := reportDiff(decodeReport(t, okReport), wantOK); diff != "" {
		t.Fatalf("success report shape mismatch: %s", diff)
	}

	// status reuses the same shape against live child state (§5.5).
	statusReport, err := okOrch.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if diff := reportDiff(decodeReport(t, statusReport), wantOK); diff != "" {
		t.Fatalf("status report shape mismatch: %s", diff)
	}

	assertSingleProjectUpDoesNotRollBack(t)
}

// assertSingleProjectUpDoesNotRollBack is the regression fixture TASK-291 requires: a
// single-project Up whose second entry fails must leave the first entry exactly as it
// was. TASK-260 §5.1 records this as today's behavior and the automatic rollback it
// introduces is for composition only.
func assertSingleProjectUpDoesNotRollBack(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	marker := filepath.Join(dir, "first.up")

	entries := map[string]*config.LifecycleEntry{
		"first": {
			Order: 1,
			Script: &config.ScriptPluginConfig{
				Up:   "touch " + marker,
				Down: "rm -f " + marker,
			},
		},
		"second": {
			Order:  2,
			Script: &config.ScriptPluginConfig{Up: "exit 1"},
		},
	}
	orch := NewOrchestrator(&config.Config{Stack: entries}, config.NewEnvironment(nil, dir, dir))

	if err := orch.Up(context.Background(), UpOptions{}); err == nil {
		t.Fatal("single-project Up succeeded, want the second entry to fail")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("single-project Up rolled back the succeeded entry (marker %s gone: %v) — its no-rollback behavior must not change", marker, err)
	}
}

func decodeReport(t *testing.T, report *CompositionReport) map[string]any {
	t.Helper()
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	return out
}

func decodeMap(t *testing.T, m map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal report map: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal report map: %v", err)
	}
	return out
}

func reportDiff(got, want map[string]any) string {
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) == string(wantJSON) {
		return ""
	}
	return fmt.Sprintf("\n got: %s\nwant: %s", gotJSON, wantJSON)
}
