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

// compositionChildProject and compositionChildPlanPart split a composed child's name
// (lifecycle.CompositionPlanEntry.ChildPlan.Name, e.g. "api/deploy") into the project label
// and the plan name within it. A child with no "/" is a local plan composed by name, and its
// project label is its own name — there is no narrower qualifier to strip.
func compositionChildProject(childName string) string {
	if i := strings.IndexByte(childName, '/'); i > 0 {
		return childName[:i]
	}
	return childName
}

func compositionChildPlanPart(childName string) string {
	if i := strings.IndexByte(childName, '/'); i > 0 {
		return childName[i+1:]
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

// errCompositionRuntimeNotImplemented is returned by the destructive/state-changing verbs
// (up/down/stop/restart) once flag scope has validated cleanly. TASK-292's scope is CLI-layer
// flag enforcement and aggregate read-only/build output only (see the task card's non-goals);
// the sequential wave execution and LIFO rollback orchestrator that would actually run these
// verbs across composed children is TASK-291, tracked separately and not yet available. No
// child is started on this path — validateCompositionFlagScope already refused before this
// point if anything was out of scope.
func errCompositionRuntimeNotImplemented(planName, verb string) error {
	return fmt.Errorf("composition plan %q: %s is not yet implemented — flag scope was validated and no composed child was started; the composition execution runtime lands in TASK-291", planName, verb)
}

// runCompositionLifecycleStub resolves and flag-validates a composition invocation for one of
// the state-changing verbs, then reports the TASK-291 gap. Shared by up/down/stop/restart so
// the four call sites cannot drift on what gets validated before the stub error.
func runCompositionLifecycleStub(c *config.Config, planName, verb string, extraArgs []string) error {
	comp, err := lifecycle.ResolveCompositionPlan(c, planName)
	if err != nil {
		return err
	}
	if _, err := validateCompositionFlagScope(comp, planName, verb, extraArgs); err != nil {
		return err
	}
	return errCompositionRuntimeNotImplemented(planName, verb)
}

func runCompositionUp(c *config.Config, _ *envLoad, planName string, extraArgs []string) error {
	return runCompositionLifecycleStub(c, planName, "up", extraArgs)
}

func runCompositionDown(c *config.Config, _ *envLoad, planName string, extraArgs []string) error {
	return runCompositionLifecycleStub(c, planName, "down", extraArgs)
}

func runCompositionStop(c *config.Config, _ *envLoad, planName string, extraArgs []string) error {
	return runCompositionLifecycleStub(c, planName, "stop", extraArgs)
}

func runCompositionRestart(c *config.Config, _ *envLoad, planName string, extraArgs []string) error {
	return runCompositionLifecycleStub(c, planName, "restart", extraArgs)
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

// compositionChildStatus and compositionStatusReport are TASK-260 §5.3's frozen JSON shape,
// reused by §5.5 for status's success case too (outcome "up", every child "up"). up/down/
// stop/restart do not populate this today — see errCompositionRuntimeNotImplemented — so
// Rollback is always the empty-but-present shape §5.3 shows, and State is only ever "up" or
// "failed" here; "rolled_back"/"rollback_failed"/"not_started" belong to the TASK-291
// rollback orchestrator this task does not implement.
type compositionChildStatus struct {
	Project string `json:"project"`
	Plan    string `json:"plan"`
	Wave    int    `json:"wave"`
	State   string `json:"state"`
	Error   string `json:"error,omitempty"`
}

type compositionRollbackReport struct {
	Attempted []string `json:"attempted"`
	Succeeded []string `json:"succeeded"`
	Failed    []string `json:"failed"`
}

type compositionStatusReport struct {
	DvaVersion string                    `json:"dva_version"`
	Plan       string                    `json:"plan"`
	Kind       string                    `json:"kind"`
	Outcome    string                    `json:"outcome"`
	Children   []compositionChildStatus  `json:"children"`
	Rollback   compositionRollbackReport `json:"rollback"`
	Error      string                    `json:"error,omitempty"`
}

// runCompositionStatus queries every composed child and aggregates §5.3's JSON shape for both
// --json and text output. §4.3 permits concurrent queries here (read-only, no side effects to
// serialize) but this loops sequentially: the contract is the aggregate shape and the
// project-labeled report, not a concurrency guarantee, and a sequential loop is the smaller
// change against the single-plan status path this reuses per child.
func runCompositionStatus(c *config.Config, el *envLoad, planName string) error {
	comp, err := lifecycle.ResolveCompositionPlan(c, planName)
	if err != nil {
		return err
	}

	report := compositionStatusReport{
		DvaVersion: config.Version,
		Plan:       planName,
		Kind:       "composition",
		Children:   make([]compositionChildStatus, 0, len(comp.Entries)),
		Rollback:   compositionRollbackReport{Attempted: []string{}, Succeeded: []string{}, Failed: []string{}},
	}

	failed := false
	for _, entry := range comp.Entries {
		childName := entry.ChildPlan.Name
		cs := compositionChildStatus{
			Project: compositionChildProject(childName),
			Plan:    compositionChildPlanPart(childName),
			Wave:    entry.Wave,
			State:   "up",
		}

		if childErr := queryCompositionChildStatus(c, el, childName); childErr != nil {
			cs.State = "failed"
			cs.Error = childErr.Error()
			failed = true
		}
		report.Children = append(report.Children, cs)
	}

	if failed {
		report.Outcome = "failed"
		report.Error = fmt.Sprintf("composition plan %q: one or more composed children are unrunnable", planName)
	} else {
		report.Outcome = "up"
	}

	if jsonOutput {
		if err := output.PrintJSON(report); err != nil {
			return err
		}
	} else {
		printCompositionStatusText(report)
	}

	if failed {
		return errors.New(report.Error)
	}
	return nil
}

// queryCompositionChildStatus runs one child through the same resolve-runtime + orchestrator
// Status path runPlanStatus uses for a directly named plan, returning nil only when the child
// resolved, its environment inputs were complete, and every one of its entries queried clean.
func queryCompositionChildStatus(c *config.Config, el *envLoad, childName string) error {
	runtime, err := resolvePlanRuntime(c, el, childName, nil)
	if err != nil {
		return err
	}
	if runtime.report.Incomplete() {
		return fmt.Errorf("environment inputs incomplete for %q", childName)
	}

	orch, err := lifecycle.NewPlanOrchestrator(runtime.config, runtime.env, runtime.plan)
	if err != nil {
		return err
	}
	status, err := orch.Status(context.Background())
	if err != nil {
		return err
	}
	status = filterStatusByNames(status, planEntryNames(runtime.plan))
	return lifecycle.StatusExitError(status)
}

func printCompositionStatusText(report compositionStatusReport) {
	fmt.Printf("Composition plan: %s (dva v%s)\n", report.Plan, report.DvaVersion)
	for _, child := range report.Children {
		fmt.Printf("\n  [%s] %s (wave %d): %s\n", child.Project, child.Plan, child.Wave, child.State)
		if child.Error != "" {
			fmt.Printf("    error: %s\n", child.Error)
		}
	}
	fmt.Printf("\noutcome: %s\n", report.Outcome)
}
