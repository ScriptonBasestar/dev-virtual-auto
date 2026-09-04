package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/lifecycle"
	"github.com/ScriptonBasestar/dva/internal/output"
)

// isCompositionPlan reports whether name resolves to a composes: plan (TASK-260 §3.1),
// mirroring the same PlanConfig.Composes check the config-layer validator already applies.
// Every lifecycle-verb RunE calls this immediately after detectPlanRoute succeeds, so a
// composition plan is routed to this file's handlers instead of the single-plan ones in
// plan_lifecycle.go.
func isCompositionPlan(c *config.Config, name string) bool {
	if c == nil {
		return false
	}
	plan := c.Plans[name]
	return plan != nil && len(plan.Composes) > 0
}

// compositionChildProject extracts the project label from a composed child's name
// (lifecycle.CompositionPlanEntry.ChildPlan.Name, e.g. "api/deploy" -> "api"). A child with
// no "/" is a local plan composed by name, and its project label is its own name — there is
// no narrower qualifier to strip.
func compositionChildProject(childName string) string {
	if i := strings.IndexByte(childName, '/'); i > 0 {
		return childName[:i]
	}
	return childName
}

// compositionChildNames lists a composition's children in resolved (wave, order, name) order,
// for error messages that enumerate what --project may name.
func compositionChildNames(comp *lifecycle.CompositionPlan) []string {
	names := make([]string, 0, len(comp.Entries))
	for _, entry := range comp.Entries {
		names = append(names, entry.ChildPlan.Name)
	}
	return names
}

// findCompositionChildByProject resolves a --project value to one composed child, matching
// either the full child plan name ("api/deploy") or its project label ("api") per TASK-260
// §4.4's worked example (`dva down release --project api --volumes`). Zero matches is an
// unknown scope; more than one is ambiguous — both refuse rather than guess, since this feeds
// a destructive flag's scope.
func findCompositionChildByProject(comp *lifecycle.CompositionPlan, project string) (*lifecycle.CompositionPlanEntry, error) {
	project = strings.TrimSpace(project)
	if project == "" {
		return nil, fmt.Errorf("--project requires a non-empty child name")
	}
	var matches []*lifecycle.CompositionPlanEntry
	for i := range comp.Entries {
		entry := &comp.Entries[i]
		name := entry.ChildPlan.Name
		if name == project || compositionChildProject(name) == project {
			matches = append(matches, entry)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("unknown scope %q: not a composed child of this plan (children: %s)", project, strings.Join(compositionChildNames(comp), ", "))
	case 1:
		return matches[0], nil
	default:
		return nil, fmt.Errorf("ambiguous scope %q: matches more than one composed child (%s); name the full child plan (project/plan)", project, strings.Join(compositionChildNames(comp), ", "))
	}
}

// compositionFlags is the parsed, scope-checked result of a composition plan invocation's
// argv, per TASK-260 §4.4's propagate/reject/require-scope table.
type compositionFlags struct {
	noWait      bool
	noRollback  bool
	force       bool
	volumes     bool
	purge       bool
	scopedChild *lifecycle.CompositionPlanEntry // set when --force/--volumes/--purge named a valid --project child
}

// validateCompositionFlagScope reads a composition plan's argv and enforces §4.4 before any
// child is touched:
//
//   - --no-wait, --no-rollback: propagate-to-all — accepted (verb-gated the same way the
//     single-plan parser gates --no-wait) and carried through in the returned struct.
//   - --var, --mode/-M, --env/-E, --tag/--tags/-T, --exclude-tag/--exclude-tags: reject. These
//     are whole-stack-path flags already rejected once a plan is named (compose.go's Long
//     text); composition is a plan path too, so the same rule applies without a new one.
//   - --force, --volumes/-v, --purge: require-explicit-scope. --project <child> must resolve to
//     exactly one composed child, or the whole invocation is refused before any child starts.
//
// Every branch returns before starting anything, so a caller that gets a nil error is safe to
// proceed and a caller that gets a non-nil one has started nothing.
func validateCompositionFlagScope(comp *lifecycle.CompositionPlan, planName, verb string, args []string) (compositionFlags, error) {
	var flags compositionFlags
	var projectValue string
	var hasProject bool

	for i := 0; i < len(args); i++ {
		a := args[i]
		if !isFlagToken(a) {
			return flags, fmt.Errorf("unexpected argument for composition plan %q: %s", planName, a)
		}
		name, value, hasValue := splitFlagToken(a)
		switch name {
		case "--no-wait":
			if !waitApplicableVerbs[verb] {
				return flags, fmt.Errorf("composition plan %q: unsupported flag for %s: --no-wait", planName, verb)
			}
			flags.noWait = true
		case "--no-rollback":
			if verb != "up" {
				return flags, fmt.Errorf("composition plan %q: --no-rollback only applies to up (it opts out of up's automatic rollback)", planName)
			}
			flags.noRollback = true
		case "--var":
			return flags, fmt.Errorf("composition plan %q does not accept --var: which composed child would it apply to is ambiguous; set composes[].vars for that child instead (config.CompositionEntry.Vars)", planName)
		case "--mode", "-M":
			return flags, fmt.Errorf("composition plan %q does not accept --mode: it is a whole-stack-path flag, already rejected once any plan is named", planName)
		case "--env", "-E":
			return flags, fmt.Errorf("composition plan %q does not accept --env: it is a whole-stack-path flag, already rejected once any plan is named", planName)
		case "--tag", "--tags", "-T":
			return flags, fmt.Errorf("composition plan %q does not accept --tag: it is a whole-stack-path flag, already rejected once any plan is named", planName)
		case "--exclude-tag", "--exclude-tags":
			return flags, fmt.Errorf("composition plan %q does not accept --exclude-tag: it is a whole-stack-path flag, already rejected once any plan is named", planName)
		case "--force":
			flags.force = true
		case "-v", "--volumes":
			flags.volumes = true
		case "--purge":
			flags.purge = true
		case "--project":
			v, consumed, ok := flagValue(args, i, len(args), value, hasValue)
			if !ok {
				return flags, fmt.Errorf("--project requires a value")
			}
			i += consumed
			projectValue = v
			hasProject = true
		default:
			return flags, fmt.Errorf("composition plan %q: unsupported flag: %s", planName, a)
		}
	}

	// --purge and --volumes are down-only, mirroring plan_lifecycle.go's rejectDownOnlyFlags:
	// accepting either on any other verb and then silently not acting on it (because the
	// verb has nothing to remove) is the one outcome that must not happen — the operator
	// would have no way to tell whether the removal it asked for actually ran. This is
	// checked before the --project scope requirement below so a scoped-but-wrong-verb
	// invocation is still refused, not merely a missing-scope one.
	if verb != "down" {
		switch {
		case flags.purge:
			return flags, fmt.Errorf("composition plan %q: --purge is only supported by down", planName)
		case flags.volumes:
			return flags, fmt.Errorf("composition plan %q: --volumes is only supported by down", planName)
		}
	}

	if flags.force || flags.volumes || flags.purge {
		if !hasProject {
			return flags, fmt.Errorf("composition plan %q: --force/--volumes/--purge require --project <child> naming one composed child; refusing before any child starts", planName)
		}
		entry, err := findCompositionChildByProject(comp, projectValue)
		if err != nil {
			return flags, fmt.Errorf("composition plan %q: %w", planName, err)
		}
		flags.scopedChild = entry
	} else if hasProject {
		return flags, fmt.Errorf("composition plan %q: --project only applies alongside --force, --volumes, or --purge", planName)
	}

	return flags, nil
}

// compositionChildEnvironment resolves each composed child's owning config/environment so
// lifecycle.PlanChildExecutor can build a per-child *lifecycle.Orchestrator without this file
// re-deriving that resolution logic.
func compositionChildEnvironment(c *config.Config, el *envLoad) func(child *lifecycle.ExecutionPlan) (*config.Config, *config.Environment, error) {
	return func(child *lifecycle.ExecutionPlan) (*config.Config, *config.Environment, error) {
		runtime, err := resolvePlanRuntime(c, el, child.Name, nil)
		if err != nil {
			return nil, nil, err
		}
		if runtime.report.Incomplete() {
			return nil, nil, fmt.Errorf("environment inputs incomplete for %q", child.Name)
		}
		return runtime.config, runtime.env, nil
	}
}

// compositionExecutor implements lifecycle.CompositionChildExecutor by delegating to
// lifecycle.PlanChildExecutor per child, applying --force to only the --project-scoped child
// (TASK-260 §4.4: force is require-explicit-scope, not propagate-to-all — but
// lifecycle.PlanChildExecutor.Force is a single flat bool covering every child it touches, so
// this wraps two instances and picks between them by child name instead of widening the frozen
// orchestrator's executor interface).
type compositionExecutor struct {
	forced     *lifecycle.PlanChildExecutor
	unforced   *lifecycle.PlanChildExecutor
	forceChild string
}

// newCompositionExecutor sets PlanChildExecutor.DryRun from the same package-level dryRun
// global the single-plan path reads (root.go's --dry-run persistent flag, or compose.go's
// parseDvaFlags for the DisableFlagParsing commands composition verbs also use). --dry-run
// never reaches validateCompositionFlagScope's switch at all — parseDvaFlags consumes it into
// the global and strips it from extraArgs before this file sees anything — so the only place
// composition dry-run can be honored is here, matching PlanChildExecutor's existing DryRun
// handling in Up/WaitReady/Down/Stop/IsUp (TASK-291).
func newCompositionExecutor(c *config.Config, el *envLoad, forceChild string) *compositionExecutor {
	env := compositionChildEnvironment(c, el)
	return &compositionExecutor{
		forced:     &lifecycle.PlanChildExecutor{Environment: env, Force: true, DryRun: dryRun},
		unforced:   &lifecycle.PlanChildExecutor{Environment: env, Force: false, DryRun: dryRun},
		forceChild: forceChild,
	}
}

func (e *compositionExecutor) pick(child *lifecycle.ExecutionPlan) *lifecycle.PlanChildExecutor {
	if e.forceChild != "" && child.Name == e.forceChild {
		return e.forced
	}
	return e.unforced
}

func (e *compositionExecutor) Up(ctx context.Context, child *lifecycle.ExecutionPlan) error {
	return e.pick(child).Up(ctx, child)
}

func (e *compositionExecutor) WaitReady(ctx context.Context, child *lifecycle.ExecutionPlan) error {
	return e.pick(child).WaitReady(ctx, child)
}

func (e *compositionExecutor) Down(ctx context.Context, child *lifecycle.ExecutionPlan, opts lifecycle.ChildDownOptions) error {
	return e.pick(child).Down(ctx, child, opts)
}

func (e *compositionExecutor) Stop(ctx context.Context, child *lifecycle.ExecutionPlan) error {
	return e.pick(child).Stop(ctx, child)
}

func (e *compositionExecutor) IsUp(ctx context.Context, child *lifecycle.ExecutionPlan) (bool, error) {
	return e.pick(child).IsUp(ctx, child)
}

// scopedChildName returns the --project-scoped child's full name ("api/deploy"), or "" when
// the invocation carried no destructive scope.
func scopedChildName(flags compositionFlags) string {
	if flags.scopedChild == nil {
		return ""
	}
	return flags.scopedChild.ChildPlan.Name
}

// compositionDestructiveOptions builds the per-child down options a --project-scoped
// destructive flag applies to (TASK-260 §4.4's worked example: "api/deploy만 volume까지
// 제거하고 web/deploy는 건드리지 않는다" — only the named child's ChildDownOptions carries
// Volumes/RemoveImages; every other child that comes down in the same call gets the zero
// value). --purge implies volume removal in addition to image removal, matching the
// single-plan path's `Volumes: flags.volumes || flags.purge` (plan_lifecycle.go).
func compositionDestructiveOptions(flags compositionFlags) map[string]lifecycle.ChildDownOptions {
	if flags.scopedChild == nil {
		return nil
	}
	return map[string]lifecycle.ChildDownOptions{
		flags.scopedChild.ChildPlan.Name: {
			Volumes:      flags.volumes || flags.purge,
			RemoveImages: flags.purge,
		},
	}
}

// renderCompositionReport prints a *lifecycle.CompositionReport in TASK-260 §5.3's shape (plus
// dva_version) for --json, or a text summary for the human path. It always returns runErr
// unchanged — TASK-260 §5.6's flat 0/1 mapping comes from *that* error, not from anything
// rendering could fail on.
func renderCompositionReport(report *lifecycle.CompositionReport, runErr error) error {
	if report == nil {
		return runErr
	}
	if jsonOutput {
		doc := report.Map()
		doc["dva_version"] = config.Version
		if err := output.PrintJSON(doc); err != nil && runErr == nil {
			return err
		}
	} else {
		printCompositionReportText(report)
	}
	return runErr
}

func printCompositionReportText(report *lifecycle.CompositionReport) {
	fmt.Printf("Composition plan: %s (dva v%s)\n", report.Plan, config.Version)
	for _, child := range report.Children {
		fmt.Printf("\n  [%s] %s (wave %d): %s\n", child.Project, child.Plan, child.Wave, child.State)
		if child.Error != "" {
			fmt.Printf("    error: %s\n", child.Error)
		}
	}
	if len(report.Rollback.Attempted) > 0 {
		fmt.Printf("\nrollback: attempted=%s succeeded=%s failed=%s\n",
			strings.Join(report.Rollback.Attempted, ", "),
			strings.Join(report.Rollback.Succeeded, ", "),
			strings.Join(report.Rollback.Failed, ", "))
	}
	fmt.Printf("\noutcome: %s\n", report.Outcome)
	if report.Error != "" {
		fmt.Printf("error: %s\n", report.Error)
	}
}

func runCompositionUp(c *config.Config, el *envLoad, planName string, extraArgs []string) error {
	comp, err := lifecycle.ResolveCompositionPlan(c, planName)
	if err != nil {
		return err
	}
	flags, err := validateCompositionFlagScope(comp, planName, "up", extraArgs)
	if err != nil {
		return err
	}

	orch, err := lifecycle.NewCompositionOrchestrator(comp, newCompositionExecutor(c, el, scopedChildName(flags)))
	if err != nil {
		return err
	}
	report, runErr := orch.Up(context.Background(), lifecycle.CompositionUpOptions{
		NoWait:     flags.noWait,
		NoRollback: flags.noRollback,
	})
	return renderCompositionReport(report, runErr)
}

func runCompositionDown(c *config.Config, el *envLoad, planName string, extraArgs []string) error {
	comp, err := lifecycle.ResolveCompositionPlan(c, planName)
	if err != nil {
		return err
	}
	flags, err := validateCompositionFlagScope(comp, planName, "down", extraArgs)
	if err != nil {
		return err
	}

	// Mirrors runPlanDown (plan_lifecycle.go): only --purge prompts, matching --force's role
	// as the single waiver both paths recognize. --volumes alone tears down without a prompt
	// on the single-plan path today too — this is not a new asymmetry TASK-292 introduces.
	// --dry-run also waives it, same as runPlanDown's effectiveDryRun check: nothing destructive
	// is about to happen, so there is nothing to confirm.
	if flags.purge && !flags.force && !dryRun {
		proceed, err := confirmDestruction(fmt.Sprintf("dva down %s --project %s --purge", planName, scopedChildName(flags)), true, true)
		if err != nil {
			return err
		}
		if !proceed {
			return nil
		}
	}

	orch, err := lifecycle.NewCompositionOrchestrator(comp, newCompositionExecutor(c, el, scopedChildName(flags)))
	if err != nil {
		return err
	}
	report, runErr := orch.Down(context.Background(), lifecycle.CompositionDownOptions{
		Destructive: compositionDestructiveOptions(flags),
	})
	return renderCompositionReport(report, runErr)
}

func runCompositionStop(c *config.Config, el *envLoad, planName string, extraArgs []string) error {
	comp, err := lifecycle.ResolveCompositionPlan(c, planName)
	if err != nil {
		return err
	}
	flags, err := validateCompositionFlagScope(comp, planName, "stop", extraArgs)
	if err != nil {
		return err
	}

	orch, err := lifecycle.NewCompositionOrchestrator(comp, newCompositionExecutor(c, el, scopedChildName(flags)))
	if err != nil {
		return err
	}
	report, runErr := orch.Stop(context.Background())
	return renderCompositionReport(report, runErr)
}

// runCompositionRestart stops then brings the composition back up, mirroring
// lifecycle.Orchestrator.Restart's own stop-then-up pattern at the single-plan level:
// lifecycle.CompositionOrchestrator has no Restart of its own — TASK-291's frozen surface is
// Up/Down/Stop/Status only (composition_orchestrator.go; not to be widened here). A failed
// stop returns immediately without attempting the up half, the same short-circuit
// lifecycle.Orchestrator.Restart uses for a single plan.
func runCompositionRestart(c *config.Config, el *envLoad, planName string, extraArgs []string) error {
	comp, err := lifecycle.ResolveCompositionPlan(c, planName)
	if err != nil {
		return err
	}
	flags, err := validateCompositionFlagScope(comp, planName, "restart", extraArgs)
	if err != nil {
		return err
	}

	orch, err := lifecycle.NewCompositionOrchestrator(comp, newCompositionExecutor(c, el, scopedChildName(flags)))
	if err != nil {
		return err
	}

	stopReport, stopErr := orch.Stop(context.Background())
	if stopErr != nil {
		return renderCompositionReport(stopReport, stopErr)
	}

	upReport, upErr := orch.Up(context.Background(), lifecycle.CompositionUpOptions{
		NoWait:     flags.noWait,
		NoRollback: flags.noRollback,
	})
	return renderCompositionReport(upReport, upErr)
}

// runCompositionBuild builds every composed child in wave order (§4.3: same order as up, no
// rollback — build is not destructive and creates no execution state to unwind). Unlike
// up/down/stop/restart this is fully implemented: build has no orchestrator dependency, it is
// a sequential loop over the existing single-plan runPlanBuild.
func runCompositionBuild(c *config.Config, el *envLoad, planName string, extraArgs []string) error {
	comp, err := lifecycle.ResolveCompositionPlan(c, planName)
	if err != nil {
		return err
	}
	if _, err := validateCompositionFlagScope(comp, planName, "build", extraArgs); err != nil {
		return err
	}
	if len(extraArgs) > 0 {
		return fmt.Errorf("composition plan %q builds every composed child; passthrough build arguments are not supported here — run 'dva build %s' directly to target one child",
			planName, compositionChildNames(comp)[0])
	}

	for _, entry := range comp.Entries {
		childName := entry.ChildPlan.Name
		fmt.Fprintf(os.Stderr, "[composition: %s / %s]\n", planName, childName)
		if err := runPlanBuild(c, el, childName, nil); err != nil {
			return fmt.Errorf("composed child %q: %w", childName, err)
		}
	}
	return nil
}

// runCompositionLogs shows every composed child's logs in wave order, each prefixed with its
// project label (§4.3: "project 라벨 필수" is the only frozen part of the contract; the exact
// interleaving is left as an implementation detail, so a sequential loop satisfies it).
//
// forceSubprocess is set for the duration of the loop because execComposePassthroughForEntry
// otherwise ends in dvaexec.ExecReplace — a process-replacing exec that would make the first
// compose-backed child's logs command become the whole dva process and never return, silently
// dropping every subsequent child. hooks.go already uses this same global for an analogous
// after-hook problem; this reuses it rather than inventing a second execution mode.
func runCompositionLogs(c *config.Config, el *envLoad, planName string, extraArgs []string) error {
	comp, err := lifecycle.ResolveCompositionPlan(c, planName)
	if err != nil {
		return err
	}
	if _, err := validateCompositionFlagScope(comp, planName, "logs", extraArgs); err != nil {
		return err
	}
	if len(extraArgs) > 0 {
		return fmt.Errorf("composition plan %q aggregates every composed child's logs; passthrough arguments are not supported here — run 'dva logs %s' directly to target one child",
			planName, compositionChildNames(comp)[0])
	}

	forceSubprocess = true
	defer func() { forceSubprocess = false }()

	var errs []error
	for _, entry := range comp.Entries {
		childName := entry.ChildPlan.Name
		fmt.Printf("[project: %s]\n", compositionChildProject(childName))
		if err := runPlanLogs(c, el, childName, nil); err != nil {
			errs = append(errs, fmt.Errorf("composed child %q: %w", childName, err))
		}
	}
	return errors.Join(errs...)
}

// runCompositionStatus queries every composed child through CompositionOrchestrator.Status,
// the same orchestrator Up/Down/Stop/Restart already delegate to (TASK-297: this used to be a
// second, parallel implementation of TASK-260 §5.3/§5.5's report that computed its own
// out-of-contract "not_started" outcome with exit code 0 for a fully-down composition; now
// there is exactly one Outcome-computation path, and renderCompositionReport reuses the same
// Up/Down/Stop rendering and flat exit-code convention for status too).
func runCompositionStatus(c *config.Config, el *envLoad, planName string) error {
	comp, err := lifecycle.ResolveCompositionPlan(c, planName)
	if err != nil {
		return err
	}

	// status is a read-only query, never an execution verb — dry-run has no coherent meaning
	// here (runPlanStatus, the single-plan equivalent, never consults it either), and honoring
	// it would fabricate "up" child states on a fully-down composition (found in review of
	// TASK-297: newCompositionExecutor's DryRun wiring exists for up/down/stop/restart's
	// teardown gate, not for status). Force it off regardless of the global --dry-run flag.
	exec := newCompositionExecutor(c, el, "")
	exec.forced.DryRun = false
	exec.unforced.DryRun = false
	orch, err := lifecycle.NewCompositionOrchestrator(comp, exec)
	if err != nil {
		return err
	}
	report, runErr := orch.Status(context.Background())
	return renderCompositionReport(report, runErr)
}
