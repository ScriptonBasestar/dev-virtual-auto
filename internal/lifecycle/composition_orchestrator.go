package lifecycle

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// Child execution states, fixed by TASK-260 §5.3 ("state는 up/failed/rolled_back/
// rollback_failed/not_started 중 하나로 고정한다"). No sixth value exists.
const (
	ChildStateUp             = "up"
	ChildStateFailed         = "failed"
	ChildStateRolledBack     = "rolled_back"
	ChildStateRollbackFailed = "rollback_failed"
	ChildStateNotStarted     = "not_started"
)

// Report outcomes. TASK-260 §5.3 shows "failed" and §5.5 shows "up"; §5.6 maps every
// non-"up" outcome to exit code 1, so teardown verbs report "down" on full success and
// "failed" otherwise.
const (
	CompositionOutcomeUp     = "up"
	CompositionOutcomeDown   = "down"
	CompositionOutcomeFailed = "failed"
)

// CompositionChildReport is one row of the partial-state report's children[] array.
type CompositionChildReport struct {
	Project string `json:"project"`
	Plan    string `json:"plan"`
	Wave    int    `json:"wave"`
	State   string `json:"state"`
	Error   string `json:"error,omitempty"`
}

// CompositionRollbackReport is the report's rollback block. The three lists hold child
// labels (the composed plan name, e.g. "api/deploy") as TASK-260 §5.3 shows.
type CompositionRollbackReport struct {
	Attempted []string `json:"attempted"`
	Succeeded []string `json:"succeeded"`
	Failed    []string `json:"failed"`
}

// CompositionReport is the partial-state report frozen by TASK-260 §5.3, reused
// unchanged for the success path and for `status` (§5.5). The `dva_version` field of
// §5.3's example is added by the command layer that prints it (TASK-292), which owns
// version reporting for every other command already.
type CompositionReport struct {
	Plan     string                    `json:"plan"`
	Kind     string                    `json:"kind"`
	Outcome  string                    `json:"outcome"`
	Children []CompositionChildReport  `json:"children"`
	Rollback CompositionRollbackReport `json:"rollback"`
	Error    string                    `json:"error,omitempty"`
}

// Map renders the report in the map[string]any form status.go established and TASK-260
// §5.3 names, for output.PrintJSON.
func (r *CompositionReport) Map() map[string]any {
	children := make([]map[string]any, 0, len(r.Children))
	for _, c := range r.Children {
		row := map[string]any{
			"project": c.Project,
			"plan":    c.Plan,
			"wave":    c.Wave,
			"state":   c.State,
		}
		if c.Error != "" {
			row["error"] = c.Error
		}
		children = append(children, row)
	}
	out := map[string]any{
		"plan":     r.Plan,
		"kind":     r.Kind,
		"outcome":  r.Outcome,
		"children": children,
		"rollback": map[string]any{
			"attempted": stringsOrEmpty(r.Rollback.Attempted),
			"succeeded": stringsOrEmpty(r.Rollback.Succeeded),
			"failed":    stringsOrEmpty(r.Rollback.Failed),
		},
	}
	if r.Error != "" {
		out["error"] = r.Error
	}
	return out
}

func stringsOrEmpty(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

// CompositionError carries a composition failure without rewriting it. Error() returns
// the primary failure's message byte-for-byte, so TASK-260 §5.2's "original-error
// preservation" holds even when a rollback `down` failed afterwards: those failures are
// reachable only through Diagnostics and the report.
type CompositionError struct {
	Err         error
	Diagnostics []string
	Report      *CompositionReport
}

func (e *CompositionError) Error() string { return e.Err.Error() }

func (e *CompositionError) Unwrap() error { return e.Err }

// ChildDownOptions selects how destructive one child's teardown is. Rollback always
// passes the zero value: TASK-260 §5.2 forbids volume/image removal on the rollback path
// regardless of what the failed `up` invocation carried.
type ChildDownOptions struct {
	Volumes      bool
	RemoveImages bool
}

// CompositionChildExecutor runs one lifecycle verb against a single composed child.
// The composition orchestrator owns ordering, rollback, and reporting; the executor owns
// how a child is actually reached.
type CompositionChildExecutor interface {
	Up(ctx context.Context, child *ExecutionPlan) error
	// WaitReady blocks until the child's own readiness/health checks pass, returning an
	// error when they do not (TASK-260 §4.5's wave-boundary gate).
	WaitReady(ctx context.Context, child *ExecutionPlan) error
	Down(ctx context.Context, child *ExecutionPlan, opts ChildDownOptions) error
	Stop(ctx context.Context, child *ExecutionPlan) error
	// IsUp queries the child's live execution state. TASK-260 §5.4/§6.3 make this the
	// only source of truth for teardown — no cached record from an earlier run is
	// consulted.
	IsUp(ctx context.Context, child *ExecutionPlan) (bool, error)
}

// CompositionUpOptions configures composition `up`.
type CompositionUpOptions struct {
	// NoWait skips every child's readiness gate (TASK-260 §4.4, propagate-to-all).
	NoWait bool
	// NoRollback skips the automatic LIFO rollback entirely, leaving whatever succeeded
	// in place for inspection (TASK-260 §4.4 and "Open question — resolved 2026-09-04").
	NoRollback bool
}

// CompositionDownOptions configures composition `down`. Destructive holds per-child
// teardown options keyed by composed plan name — TASK-260 §4.4 requires `--volumes`/
// `--purge` to name an explicit child, so there is no composition-wide destructive
// switch. A nil map tears every child down plainly.
type CompositionDownOptions struct {
	Destructive map[string]ChildDownOptions
}

// CompositionOrchestrator executes a resolved CompositionPlan.
//
// It is a peer of Orchestrator, not a replacement: single-project execution keeps
// running through Orchestrator.Up unchanged, including its no-automatic-rollback
// behavior (TASK-260 §5.1 introduces rollback for composition only).
type CompositionOrchestrator struct {
	plan   *CompositionPlan
	exec   CompositionChildExecutor
	logger *slog.Logger
}

func NewCompositionOrchestrator(plan *CompositionPlan, exec CompositionChildExecutor) (*CompositionOrchestrator, error) {
	if plan == nil {
		return nil, fmt.Errorf("composition plan is nil")
	}
	if exec == nil {
		return nil, fmt.Errorf("composition child executor is nil")
	}
	return &CompositionOrchestrator{plan: plan, exec: exec, logger: slog.Default()}, nil
}

// Up runs every composed child in wave order, sequentially within a wave, gating on each
// wave's readiness before the next wave starts (TASK-260 §4.1, §4.5). On any failure it
// tears down the already-succeeded children in LIFO order unless opts.NoRollback is set.
//
// The report is returned on both paths. The error, when non-nil, is a *CompositionError
// whose message is the original failure unchanged.
func (o *CompositionOrchestrator) Up(ctx context.Context, opts CompositionUpOptions) (*CompositionReport, error) {
	report := o.newReport(CompositionOutcomeUp)

	// Children whose up succeeded, in the order they succeeded. Rollback walks this
	// backwards, which is what "LIFO" means here.
	var succeeded []int
	failedAt := -1
	var primary error

	for _, wave := range o.waveIndexes() {
		for _, i := range wave {
			child := o.plan.Entries[i].ChildPlan
			o.logger.Info("composition child up", "plan", o.plan.Name, "child", child.Name, "wave", o.plan.Entries[i].Wave)
			if err := o.exec.Up(ctx, child); err != nil {
				failedAt, primary = i, err
				break
			}
			report.Children[i].State = ChildStateUp
			succeeded = append(succeeded, i)
		}
		if primary != nil {
			break
		}
		if opts.NoWait {
			continue
		}
		for _, i := range wave {
			child := o.plan.Entries[i].ChildPlan
			if err := o.exec.WaitReady(ctx, child); err != nil {
				failedAt, primary = i, err
				break
			}
		}
		if primary != nil {
			break
		}
	}

	if primary == nil {
		return report, nil
	}

	// The failing child's report row is marked "failed" regardless of which pass caught
	// it. For an exec.Up failure, failedAt was never appended to succeeded, so it is
	// already excluded from rollback below. For a WaitReady failure, its exec.Up did
	// succeed and it stayed in succeeded — TASK-260 §5.2 requires rollback to tear down
	// every child whose up succeeded in the affected waves, with no carve-out for the one
	// whose readiness check is what triggered the rollback, so it is left in succeeded
	// and rolled back alongside its wave-mates below.
	report.Outcome = CompositionOutcomeFailed
	report.Error = primary.Error()
	report.Children[failedAt].State = ChildStateFailed
	report.Children[failedAt].Error = primary.Error()

	compErr := &CompositionError{Err: primary, Report: report}
	if opts.NoRollback {
		return report, compErr
	}

	// Rollback outlives the cancellation that may have caused the failure. TASK-260 §4.5
	// treats cancellation as a form of failure and still requires the already-succeeded
	// children to be torn down; on the caller's own ctx every rollback `down` would fail
	// instantly with "context canceled" and leave the whole composition up.
	rollbackCtx := context.WithoutCancel(ctx)

	for _, idx := range slices.Backward(succeeded) {
		child := o.plan.Entries[idx].ChildPlan
		label := child.Name
		report.Rollback.Attempted = append(report.Rollback.Attempted, label)
		o.logger.Info("composition rollback", "plan", o.plan.Name, "child", label)
		if err := o.exec.Down(rollbackCtx, child, ChildDownOptions{}); err != nil {
			report.Rollback.Failed = append(report.Rollback.Failed, label)
			report.Children[idx].State = ChildStateRollbackFailed
			report.Children[idx].Error = err.Error()
			compErr.Diagnostics = append(compErr.Diagnostics,
				fmt.Sprintf("rollback of %s failed: %v — %s may still be up, manual verification required", label, err, label))
			continue
		}
		report.Rollback.Succeeded = append(report.Rollback.Succeeded, label)
		report.Children[idx].State = ChildStateRolledBack
	}

	return report, compErr
}

// Down tears every composed child down in reverse-wave (LIFO) order, whatever put them
// there (TASK-260 §4.3). Each child's live state is queried first, so a `down` after a
// failed automatic rollback acts on what is actually running rather than on any record
// from that run (§5.4, §6.3).
func (o *CompositionOrchestrator) Down(ctx context.Context, opts CompositionDownOptions) (*CompositionReport, error) {
	return o.teardown(ctx, "down", func(ctx context.Context, child *ExecutionPlan) error {
		return o.exec.Down(ctx, child, opts.Destructive[child.Name])
	})
}

// Stop is Down's ordering with the child's resources preserved (TASK-260 §4.3).
func (o *CompositionOrchestrator) Stop(ctx context.Context) (*CompositionReport, error) {
	return o.teardown(ctx, "stop", o.exec.Stop)
}

// teardown walks children in reverse order, skipping the ones that are not up.
//
// A torn-down child is reported as "rolled_back" and a failed teardown as
// "rollback_failed": TASK-260 §5.3 fixes the five state values, so a teardown verb
// reuses the two that mean "we brought it down" and "we tried and it did not go down"
// rather than inventing a sixth. The rollback block stays empty — it reports the
// automatic rollback of a failed `up`, which an explicit teardown is not.
func (o *CompositionOrchestrator) teardown(ctx context.Context, verb string, run func(context.Context, *ExecutionPlan) error) (*CompositionReport, error) {
	report := o.newReport(CompositionOutcomeDown)
	var failures []string

	for i, entry := range slices.Backward(o.plan.Entries) {
		child := entry.ChildPlan
		up, err := o.exec.IsUp(ctx, child)
		if err != nil {
			// An unreadable state is not evidence the child is down. Tear it down anyway
			// and let the child's idempotent teardown absorb a no-op (§6.3).
			o.logger.Warn("composition child state query failed", "plan", o.plan.Name, "child", child.Name, "error", err)
			up = true
		}
		if !up {
			continue
		}
		o.logger.Info("composition child "+verb, "plan", o.plan.Name, "child", child.Name)
		if err := run(ctx, child); err != nil {
			report.Children[i].State = ChildStateRollbackFailed
			report.Children[i].Error = err.Error()
			failures = append(failures, fmt.Sprintf("%s: %v", child.Name, err))
			continue
		}
		report.Children[i].State = ChildStateRolledBack
	}

	if len(failures) > 0 {
		report.Outcome = CompositionOutcomeFailed
		err := fmt.Errorf("composition %s %q failed for %s", verb, o.plan.Name, strings.Join(failures, "; "))
		report.Error = err.Error()
		return report, &CompositionError{Err: err, Report: report}
	}
	return report, nil
}

// Status reports live child state in the same shape as the execution report
// (TASK-260 §5.5). Outcome is "up" only when every child is up, matching §5.6's flat
// "anything short of full success is exit 1" — so a non-nil *CompositionError is
// returned whenever Outcome is not "up", the same way Up/teardown already signal
// failure to their caller (TASK-297: this used to report a non-conforming "not_started"
// outcome with a nil error, exiting 0 for a fully-down composition).
func (o *CompositionOrchestrator) Status(ctx context.Context) (*CompositionReport, error) {
	report := o.newReport(CompositionOutcomeUp)
	allUp := true
	var childErrs []string

	for i := range o.plan.Entries {
		child := o.plan.Entries[i].ChildPlan
		up, err := o.exec.IsUp(ctx, child)
		switch {
		case err != nil:
			report.Children[i].State = ChildStateFailed
			report.Children[i].Error = err.Error()
			childErrs = append(childErrs, fmt.Sprintf("%s: %v", child.Name, err))
			allUp = false
		case up:
			report.Children[i].State = ChildStateUp
		default:
			allUp = false
		}
	}

	if allUp {
		return report, nil
	}

	report.Outcome = CompositionOutcomeFailed
	err := fmt.Errorf("composition %q is not fully up", o.plan.Name)
	if len(childErrs) > 0 {
		err = fmt.Errorf("composition %q: %s", o.plan.Name, strings.Join(childErrs, "; "))
	}
	report.Error = err.Error()
	return report, &CompositionError{Err: err, Report: report}
}

// newReport seeds one row per composed child, in wave order, every child "not_started" —
// the state TASK-260 §5.3 gives a child no verb reached.
func (o *CompositionOrchestrator) newReport(outcome string) *CompositionReport {
	return NewCompositionReportSkeleton(o.plan, outcome)
}

// NewCompositionReportSkeleton seeds one row per composed child, in wave order, every child
// "not_started" — the state TASK-260 §5.3 gives a child no verb reached. Exported so callers
// that build a report without going through CompositionOrchestrator (e.g. composition
// `restart`'s per-child loop in internal/cli, which deliberately has no Restart method on
// this orchestrator per TASK-291's frozen surface) still get the same §5.3 project/plan
// label split as Up/Down/Stop/Status, instead of a second, independently-maintained split.
func NewCompositionReportSkeleton(plan *CompositionPlan, outcome string) *CompositionReport {
	report := &CompositionReport{
		Plan:     plan.Name,
		Kind:     "composition",
		Outcome:  outcome,
		Children: make([]CompositionChildReport, 0, len(plan.Entries)),
		Rollback: CompositionRollbackReport{Attempted: []string{}, Succeeded: []string{}, Failed: []string{}},
	}
	for _, entry := range plan.Entries {
		project, planPart := splitChildLabel(entry.ChildPlan.Name)
		report.Children = append(report.Children, CompositionChildReport{
			Project: project,
			Plan:    planPart,
			Wave:    entry.Wave,
			State:   ChildStateNotStarted,
		})
	}
	return report
}

// waveIndexes groups plan entry indexes by wave, preserving the resolver's sort. Entries
// are already ordered by (wave, order, name), so a wave is one contiguous run.
func (o *CompositionOrchestrator) waveIndexes() [][]int {
	var waves [][]int
	for i, entry := range o.plan.Entries {
		if i > 0 && entry.Wave == o.plan.Entries[i-1].Wave {
			waves[len(waves)-1] = append(waves[len(waves)-1], i)
			continue
		}
		waves = append(waves, []int{i})
	}
	return waves
}

// splitChildLabel splits an imported canonical name ("api/deploy") into the project and
// plan columns TASK-260 §5.3 reports. A local leaf plan has no owning subproject, so its
// project column is empty.
func splitChildLabel(name string) (project, plan string) {
	if project, plan, ok := strings.Cut(name, "/"); ok {
		return project, plan
	}
	return "", name
}

// PlanChildExecutor reaches each composed child through the same per-plan Orchestrator a
// standalone `dva up <child>` uses, so a child behaves identically whether it is invoked
// directly or composed (TASK-260 §6.2).
//
// Environment resolves the child's owning config and environment. The caller supplies it
// because env_file loading and var precedence live in the command layer; the child's own
// project root is what that layer must return (§4.2 — the root's CWD never flows into a
// child).
type PlanChildExecutor struct {
	Environment func(child *ExecutionPlan) (*config.Config, *config.Environment, error)
	DryRun      bool
	Force       bool
}

func (e *PlanChildExecutor) orchestrator(child *ExecutionPlan) (*Orchestrator, error) {
	if e.Environment == nil {
		return nil, fmt.Errorf("composition child executor has no Environment resolver")
	}
	cfg, env, err := e.Environment(child)
	if err != nil {
		return nil, fmt.Errorf("child %q: %w", child.Name, err)
	}
	return NewPlanOrchestrator(cfg, env, child)
}

func (e *PlanChildExecutor) Up(ctx context.Context, child *ExecutionPlan) error {
	orch, err := e.orchestrator(child)
	if err != nil {
		return err
	}
	// Wait is false: the readiness gate belongs to the wave boundary (§4.5), not to each
	// entry inside a child.
	return orch.Up(ctx, UpOptions{
		DryRun: e.DryRun,
		Force:  e.Force,
		Names:  executionPlanEntryNames(child),
		Env:    child.EnvironmentName,
	})
}

func (e *PlanChildExecutor) WaitReady(ctx context.Context, child *ExecutionPlan) error {
	if e.DryRun {
		return nil
	}
	orch, err := e.orchestrator(child)
	if err != nil {
		return err
	}
	return orch.waitEntriesReady(ctx, executionPlanEntryNames(child))
}

func (e *PlanChildExecutor) Down(ctx context.Context, child *ExecutionPlan, opts ChildDownOptions) error {
	orch, err := e.orchestrator(child)
	if err != nil {
		return err
	}
	return orch.Down(ctx, DownOptions{
		DryRun:       e.DryRun,
		Volumes:      opts.Volumes,
		RemoveImages: opts.RemoveImages,
		Names:        executionPlanEntryNames(child),
		Env:          child.EnvironmentName,
	})
}

func (e *PlanChildExecutor) Stop(ctx context.Context, child *ExecutionPlan) error {
	orch, err := e.orchestrator(child)
	if err != nil {
		return err
	}
	return orch.Stop(ctx, StopOptions{
		DryRun: e.DryRun,
		Names:  executionPlanEntryNames(child),
		Env:    child.EnvironmentName,
	})
}

func (e *PlanChildExecutor) IsUp(ctx context.Context, child *ExecutionPlan) (bool, error) {
	if e.DryRun {
		return true, nil
	}
	orch, err := e.orchestrator(child)
	if err != nil {
		return false, err
	}
	status, err := orch.Status(ctx)
	if err != nil {
		return false, err
	}
	for _, entry := range status.Entries {
		for _, svc := range entry.Services {
			if serviceLooksRunning(svc.State) {
				return true, nil
			}
		}
	}
	return false, nil
}

// waitEntriesReady blocks on the health checks of the named entries and fails when any
// stays unready. Orchestrator.Up only warns in that situation; composition needs a real
// gate because the next wave's children assume the previous wave is serving (§4.5). Up's
// own behavior is untouched.
func (o *Orchestrator) waitEntriesReady(ctx context.Context, names []string) error {
	entries, err := o.filterEntries(names, nil, nil, "", "")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if len(entry.HealthChecks) == 0 {
			continue
		}
		entryEnv := o.env.Clone()
		entryEnv.MergeVars(entry.Vars)
		for _, result := range o.hc.WaitUntilReadyWithContext(ctx, entry.HealthChecks, entryEnv.WorkDir(), entryEnv) {
			if !result.Ready {
				return fmt.Errorf("entry %q health check %q not ready", entry.Name, result.Name)
			}
		}
	}
	return nil
}

func executionPlanEntryNames(plan *ExecutionPlan) []string {
	if plan == nil || len(plan.Entries) == 0 {
		return nil
	}
	names := make([]string, 0, len(plan.Entries))
	for _, entry := range plan.Entries {
		names = append(names, entry.Name)
	}
	return names
}
