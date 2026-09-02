package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/lifecycle"
	"github.com/ScriptonBasestar/dva/internal/output"
)

type planRunFlags struct {
	cliVars map[string]string
	dryRun  bool
	force   bool
	wait    bool
	volumes bool
	purge   bool
}

// rejectDownOnlyFlags fails on flags that only `down` acts on.
//
// --purge removes data, so accepting and ignoring it is the one outcome that must not
// happen: `dva up p --purge` would report success having neither started what was named nor
// removed what was asked about, and the operator has no way to tell which. --volumes has
// the same shape; restart already rejected it and up/stop silently dropped it, so this
// closes that gap too rather than adding a second flag to the same silence.
func (f planRunFlags) rejectDownOnlyFlags() error {
	switch {
	case f.purge:
		return fmt.Errorf("--purge is only supported by down")
	case f.volumes:
		return fmt.Errorf("--volumes is only supported by down")
	}
	return nil
}

type planEndpointOutput struct {
	Name  string            `json:"name"`
	Label string            `json:"label"`
	URL   string            `json:"url"`
	Paths map[string]string `json:"paths,omitempty"`
}

type planUpOutput struct {
	Action    string                      `json:"action"`
	Plan      string                      `json:"plan"`
	DryRun    bool                        `json:"dry_run"`
	Status    *lifecycle.AggregatedStatus `json:"status,omitempty"`
	Endpoints []planEndpointOutput        `json:"endpoints,omitempty"`
}

// sortedPlanNames lists the configured plan names in a stable order. Every message that
// names the plans a user may type reads them from here, so they cannot disagree about the
// set or its order — map iteration would make the same message differ between runs.
func sortedPlanNames(c *config.Config) []string {
	names := make([]string, 0, len(c.Plans))
	for name := range c.Plans {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func requirePlanSelection(c *config.Config, command string, args []string) error {
	if c == nil || !c.HasPlans() {
		return nil
	}
	// dropLeadingTerminator, exactly as detectPlanRoute does eight lines below, and for the
	// same slot. Without it a bare `--` counted as a selection and stood the guard down:
	// `dva build --` reached docker while `dva build` refused with "multiple plans
	// configured". parseDvaFlags keeps the terminator on purpose so that "the callers that
	// reject unknown flags" can reject a stray one (compose.go), and this is the caller
	// where the token kept FOR rejection was the token that suppressed it. TASK-217.
	//
	// It is not only build, and the first draft of this comment said it was. up/down/stop
	// drop the FIRST terminator at their call sites (TASK-216) and consumeRootPersistentFlags
	// consumes logs' — so `dva up --` and `dva logs --` never arrive here with one. `dva up
	// -- --` and `dva logs -- --` do: only the leading terminator is a separator, so the
	// second is an ordinary argument that rides through. Measured by reverting this line
	// alone on a two-plan no-default fixture — five rows move, and four are not build:
	// up/down/stop `-- --` go from `unknown flag "--"`, and `logs -- --` from a docker call,
	// to the same "multiple plans configured" a bare verb gets. The rule belongs to the
	// guard, which is why it is written here once instead of at each caller.
	//
	// Pinned for up/down/stop by TestSecondTerminatorMeetsThePlanGuardNotTheFlagGuard; logs
	// cannot be pinned in-process, and that test's comment says why.
	//
	// This changes the guard's verdict only. The args handed to docker are the caller's
	// own slice and are untouched, so `dva build -- web` and `dva build -- --no-cache`
	// still reach docker spelled exactly as before.
	args = dropLeadingTerminator(planRoutingArgs(args))
	if len(args) > 0 || c.DefaultPlan() != "" {
		return nil
	}

	return fmt.Errorf("multiple plans configured; specify one: dva %s <%s>", command, strings.Join(sortedPlanNames(c), "|"))
}

func detectPlanRoute(c *config.Config, args []string) (planName string, extraArgs []string, ok bool) {
	if c == nil || !c.HasPlans() {
		return "", nil, false
	}
	args = dropLeadingTerminator(planRoutingArgs(args))

	if len(args) > 0 {
		if _, exists := c.Plans[args[0]]; exists {
			return args[0], args[1:], true
		}
		return "", nil, false
	}

	if p := c.DefaultPlan(); p != "" {
		return p, nil, true
	}

	return "", nil, false
}

// dropLeadingTerminator removes a `--` occupying the plan-name slot, and only there.
//
// Deliberately NOT dropFlagTerminator, whose contract is "the first `--` anywhere" — right for
// a positional name list, wrong here. detectPlanRoute hands everything after the plan name to
// runPlan*, so a terminator further in belongs to the plan runner's argument list, not to the
// router. Only args[0] is ever the plan-name slot, so only args[0] is ever a separator rather
// than an argument.
//
// What that costs today is one word in one message, and the honest version is worth writing
// down: with dropFlagTerminator, `dva up alpha -- --bogus` is refused as `unsupported plan
// flag: --bogus`; with this helper it is refused as `unsupported plan flag: --`. Both refuse —
// four lifecycle verbs x the three plan-bearing fixtures, 12 rows, every one rc=1 on both, so
// nothing changed from refused to accepted. The choice is not "refuse vs silently honour"; it
// is which layer owns a token the user wrote after the plan name. Consuming it here would let
// the router eat a separator meant for the runner — what a future passthrough needs. TASK-210.
//
// TASK-216 added three callers outside this file: up, down and stop each call it on their raw
// args once detectPlanRoute has declined to route them, which is what makes `dva up --` mean
// what a bare `dva up` means. The "args[0] only" contract is why they call this one and not
// dropFlagTerminator: on the whole-stack path a `--` further in is a token their own guards
// have to classify, and a helper that reached in to remove it would silence them. The choice of
// helper is therefore the same choice made here for the same reason, one route over.
func dropLeadingTerminator(args []string) []string {
	if len(args) > 0 && args[0] == "--" {
		return args[1:]
	}
	return args
}

// rejectSuppressedDefaultPlan refuses silent whole-stack fallthrough when exactly
// one plan exists (DefaultPlan) but leading args prevented detectPlanRoute from
// selecting it. Name the plan explicitly instead (e.g. dva up p1 --dev).
//
// Non-flag tokens are left to rejectUnknownPlanArg / rejectUpPositionalArg so
// unknown plan names keep their existing messages.
func rejectSuppressedDefaultPlan(c *config.Config, command string, args []string) error {
	// Classify what the terminator SEPARATES, not the terminator itself. `dva restart -- s1`
	// names an entry exactly as `dva restart s1` does, so judging args[0]=="--" refused a
	// perfectly explicit invocation. The message still echoes the untouched args, because the
	// suggestion has to be a command the user can paste back. TASK-210.
	head := dropLeadingTerminator(args)
	if c == nil || !c.HasPlans() || len(head) == 0 {
		return nil
	}
	// A terminator occupied the plan-name slot, so the dash test below cannot apply: the user
	// wrote no flag, and "flags suppress the default plan" would be a false account of an
	// invocation that contains none. `dva restart -- --no-wat` said exactly that where a
	// default plan resolved, while the same command in a plan-less config already answered
	// `unknown stack entry "--no-wat"`. The command's own name check is the one that can tell
	// the user what is wrong with the token; this guard steps aside so it is reached. TASK-210.
	if len(head) != len(args) {
		return nil
	}
	def := c.DefaultPlan()
	if def == "" {
		return nil
	}
	if _, exists := c.Plans[head[0]]; exists {
		return nil
	}
	if !isFlagToken(head[0]) {
		return nil
	}
	return fmt.Errorf(
		"flags suppress the default plan %q; name it explicitly: dva %s %s %s",
		def, command, def, strings.Join(args, " "),
	)
}

// rejectUnknownPlanArg reports a plan name that reached the non-plan fallthrough
// of a plan-aware command. It reads args exactly as detectPlanRoute does — only
// args[0] is ever the plan name slot — so it fires only where detectPlanRoute
// looked for a plan and found none. Every other position belongs to a flag or a
// flag value and keeps whatever behavior it had.
//
// detectPlanRoute returns ok=false both when no plans are configured and when
// args[0] matches none; only the latter is an error. Without plans args[0] was
// never a plan name, and a leading flag means detectPlanRoute never treated the
// invocation as plan-routed either.
func rejectUnknownPlanArg(c *config.Config, args []string) error {
	// Same separator rule as detectPlanRoute, and required because this guard reads the same
	// slot. Commands that let cobra parse flags never see the terminator the user typed — it is
	// stripped before RunE — so a `--` surviving to here is a second one, and the token after it
	// is the plan name as surely as in the one-terminator form. Left in place it took the
	// leading-dash early return with it: `dva status -- -- s1` ran a full status and exited 0
	// while `dva status -- s1` refused with "plan 's1' not found". TASK-210.
	args = dropLeadingTerminator(args)
	if c == nil || !c.HasPlans() || len(args) == 0 {
		return nil
	}
	name := args[0]
	if isFlagToken(name) {
		return nil
	}
	return fmt.Errorf("plan '%s' not found. Available: %s", name, strings.Join(sortedPlanNames(c), ", "))
}

// rejectUpPositionalArg guards the plan-name slot of 'up', which advertises
// "up [OPTIONS]" and so has no positional argument that means anything else.
// It reads args the same way rejectUnknownPlanArg does — only args[0], and a
// flag token returns early — so flag values such as '--var FOO=x' are never
// mistaken for a plan name. A lone '-' is not a flag token and so is reported
// as the unmatchable name it is; see isFlagToken. TASK-218.
//
// Unlike rejectUnknownPlanArg it also fires when no plans are configured. There
// args[0] was never a plan name either, and permitting it made 'dva up s1'
// start every entry in the stack and report success. 'down' and 'stop' already
// reject a stray argument with or without plans; this makes 'up' agree.
func rejectUpPositionalArg(c *config.Config, args []string) error {
	if len(args) == 0 {
		return nil
	}
	name := args[0]
	if isFlagToken(name) {
		return nil
	}
	if c == nil || !c.HasPlans() {
		return fmt.Errorf("unexpected argument '%s': 'dva up' takes no positional arguments and no plans are configured. Run 'dva up' to start everything declared, or declare a plan whose entries name '%s' and run 'dva up <plan>'", name, name)
	}
	return rejectUnknownPlanArg(c, args)
}

func parsePlanFlags(args []string) (planRunFlags, error) {
	flags := planRunFlags{
		cliVars: map[string]string{},
		wait:    true,
	}

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--dry-run":
			flags.dryRun = true
		case a == "--force":
			flags.force = true
		case a == "--no-wait":
			flags.wait = false
		case a == "-v" || a == "--volumes":
			flags.volumes = true
		case a == "--purge":
			flags.purge = true
		case a == "--var":
			if i+1 >= len(args) {
				return flags, fmt.Errorf("--var requires KEY=VAL")
			}
			i++
			if err := setPlanVar(flags.cliVars, args[i]); err != nil {
				return flags, err
			}
		case strings.HasPrefix(a, "--var="):
			if err := setPlanVar(flags.cliVars, strings.TrimPrefix(a, "--var=")); err != nil {
				return flags, err
			}
		case strings.HasPrefix(a, "-"):
			return flags, fmt.Errorf("unsupported plan flag: %s", a)
		default:
			return flags, fmt.Errorf("unexpected argument in plan mode: %s", a)
		}
	}

	return flags, nil
}

// printPlanResolution writes the steps ResolvePlan recorded while building the plan.
// It runs only on the dry-run path: there the user asked what would happen instead of
// asking for it to happen, and the resolution is the answer. Off that path it would be
// noise on every single invocation.
//
// stderr, not stdout, for the same reason the '[plan: ...]' header above it uses stderr —
// --json output has to stay parseable (TASK-116).
func printPlanResolution(plan *lifecycle.ExecutionPlan) {
	if plan == nil || len(plan.ResolutionTrace) == 0 {
		return
	}
	fmt.Fprintln(os.Stderr, "\nResolution:")
	for _, step := range plan.ResolutionTrace {
		fmt.Fprintf(os.Stderr, "  %s\n", step)
	}
}

// setPlanVar deliberately disagrees with takeValue (compose.go) on empty values; TASK-213
// asked whether the two should agree and the answer is that they cannot. There an empty
// scalar names nothing, so `--mode=` is an error. Here the unit is a KEY=VAL pair, and the
// guard below is a format check rather than an empty-value policy — which is provable from
// either side: `--var=K=` is ACCEPTED and sets K to "", while `--var=x` is REJECTED with the
// same message despite x being non-empty. Emptiness is not what is being tested, so adopting
// TASK-213's rule here would break `--var=K=`, a real thing to want.
func setPlanVar(dst map[string]string, kv string) error {
	parts := strings.SplitN(kv, "=", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		return fmt.Errorf("invalid --var format %q, expected KEY=VAL", kv)
	}
	dst[parts[0]] = parts[1]
	return nil
}

func runPlanUp(c *config.Config, e *config.Environment, planName string, extraArgs []string) error {
	flags, err := parsePlanFlags(extraArgs)
	if err != nil {
		return err
	}
	if err := flags.rejectDownOnlyFlags(); err != nil {
		return err
	}

	runtime, err := resolvePlanRuntime(c, e, planName, flags.cliVars)
	if err != nil {
		return err
	}
	plan, c, e := runtime.plan, runtime.config, runtime.env
	fmt.Fprintf(os.Stderr, "[plan: %s] environment=%s site=%s entries=%d\n", plan.Name, plan.EnvironmentName, plan.SiteName, len(plan.Entries))

	effectiveDryRun := dryRun || flags.dryRun
	if effectiveDryRun {
		printPlanResolution(plan)
	}

	orch, err := lifecycle.NewPlanOrchestrator(c, e, plan)
	if err != nil {
		return err
	}
	if err := orch.Up(context.Background(), lifecycle.UpOptions{
		DryRun: effectiveDryRun,
		Force:  flags.force,
		Wait:   flags.wait,
		Names:  planEntryNames(plan),
		Env:    plan.EnvironmentName,
	}); err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr)
	status, statusErr := orch.Status(context.Background())
	if statusErr == nil {
		status = filterStatusByNames(status, planEntryNames(plan))
		if !jsonOutput {
			lifecycle.PrintStatus(status, c.FileDir())
		}
	}
	endpoints := filterEndpoints(c.Endpoints, plan.EndpointTags)
	if jsonOutput {
		result := planUpOutput{
			Action: "up",
			Plan:   plan.Name,
			DryRun: effectiveDryRun,
			Status: status,
		}
		if !effectiveDryRun {
			result.Endpoints = planEndpointOutputs(endpoints)
		}
		return output.PrintJSON(result)
	}
	if !effectiveDryRun {
		printEndpointTable(endpoints, nil, nil)
	}

	return nil
}

func runPlanDown(c *config.Config, e *config.Environment, planName string, extraArgs []string) error {
	flags, err := parsePlanFlags(extraArgs)
	if err != nil {
		return err
	}

	runtime, err := resolvePlanRuntime(c, e, planName, flags.cliVars)
	if err != nil {
		return err
	}
	plan, c, e := runtime.plan, runtime.config, runtime.env
	fmt.Fprintf(os.Stderr, "[plan: %s] environment=%s site=%s entries=%d\n", plan.Name, plan.EnvironmentName, plan.SiteName, len(plan.Entries))

	effectiveDryRun := dryRun || flags.dryRun
	if effectiveDryRun {
		printPlanResolution(plan)
	}

	// --purge is `clean` folded into the plan path: named volumes, locally built images and
	// the provision markers all go. The safeguard comes with it — it belongs to the
	// destruction, not to the command name the destruction used to live under.
	//
	// --force is what waives the prompt here, matching clean. Nothing else on `down` reads
	// that flag, so it has no second meaning to collide with on this path.
	if flags.purge && !flags.force && !effectiveDryRun {
		proceed, err := confirmDestruction(fmt.Sprintf("dva down %s --purge", planName), true, true)
		if err != nil {
			return err
		}
		if !proceed {
			return nil
		}
	}
	if flags.purge {
		// Markers are keyed by provision profile, not by plan, so this clears every one in
		// the config directory — the same reach `clean` had. Narrowing it to the plan is not
		// possible without a per-plan marker, and leaving them behind would make `--purge`
		// claim a clean slate it did not deliver: the next `up` would skip provisioning.
		if effectiveDryRun {
			for _, m := range provisionMarkers(c.FileDir()) {
				fmt.Fprintf(os.Stderr, "[dry-run] would delete provision marker %s\n", m)
			}
		} else {
			clearProvisionMarkers(c.FileDir())
		}
	}

	orch, err := lifecycle.NewPlanOrchestrator(c, e, plan)
	if err != nil {
		return err
	}
	return orch.Down(context.Background(), lifecycle.DownOptions{
		DryRun:       effectiveDryRun,
		Volumes:      flags.volumes || flags.purge,
		RemoveImages: flags.purge,
		Names:        planEntryNames(plan),
		Env:          plan.EnvironmentName,
	})
}

func runPlanStop(c *config.Config, e *config.Environment, planName string, extraArgs []string) error {
	flags, err := parsePlanFlags(extraArgs)
	if err != nil {
		return err
	}
	if err := flags.rejectDownOnlyFlags(); err != nil {
		return err
	}

	runtime, err := resolvePlanRuntime(c, e, planName, flags.cliVars)
	if err != nil {
		return err
	}
	plan, c, e := runtime.plan, runtime.config, runtime.env
	fmt.Fprintf(os.Stderr, "[plan: %s] environment=%s site=%s entries=%d\n", plan.Name, plan.EnvironmentName, plan.SiteName, len(plan.Entries))

	effectiveDryRun := dryRun || flags.dryRun
	if effectiveDryRun {
		printPlanResolution(plan)
	}

	orch, err := lifecycle.NewPlanOrchestrator(c, e, plan)
	if err != nil {
		return err
	}
	return orch.Stop(context.Background(), lifecycle.StopOptions{
		DryRun: effectiveDryRun,
		Names:  planEntryNames(plan),
		Env:    plan.EnvironmentName,
	})
}

func runPlanRestart(c *config.Config, e *config.Environment, planName string, extraArgs []string) error {
	flags, err := parsePlanFlags(extraArgs)
	if err != nil {
		return err
	}
	if err := flags.rejectDownOnlyFlags(); err != nil {
		return err
	}

	runtime, err := resolvePlanRuntime(c, e, planName, flags.cliVars)
	if err != nil {
		return err
	}
	plan, c, e := runtime.plan, runtime.config, runtime.env
	fmt.Fprintf(os.Stderr, "[plan: %s] environment=%s site=%s entries=%d\n", plan.Name, plan.EnvironmentName, plan.SiteName, len(plan.Entries))

	effectiveDryRun := dryRun || flags.dryRun
	if effectiveDryRun {
		printPlanResolution(plan)
	}

	orch, err := lifecycle.NewPlanOrchestrator(c, e, plan)
	if err != nil {
		return err
	}
	return orch.Restart(context.Background(), lifecycle.UpOptions{
		DryRun: effectiveDryRun,
		Force:  true,
		Wait:   flags.wait,
		Names:  planEntryNames(plan),
		Env:    plan.EnvironmentName,
	})
}

func runPlanStatus(c *config.Config, e *config.Environment, planName string) error {
	runtime, err := resolvePlanRuntime(c, e, planName, nil)
	if err != nil {
		return err
	}
	plan, c, e := runtime.plan, runtime.config, runtime.env
	fmt.Fprintf(os.Stderr, "[plan: %s] environment=%s site=%s entries=%d\n", plan.Name, plan.EnvironmentName, plan.SiteName, len(plan.Entries))

	orch, err := lifecycle.NewPlanOrchestrator(c, e, plan)
	if err != nil {
		return err
	}
	status, err := orch.Status(context.Background())
	if err != nil {
		return err
	}

	filtered := filterStatusByNames(status, planEntryNames(plan))
	lifecycle.PrintStatus(filtered, c.FileDir())
	return lifecycle.StatusExitError(filtered)
}

func planEntryNames(plan *lifecycle.ExecutionPlan) []string {
	if plan == nil || len(plan.Entries) == 0 {
		return nil
	}
	names := make([]string, 0, len(plan.Entries))
	for _, entry := range plan.Entries {
		names = append(names, entry.Name)
	}
	return names
}

func planEndpointOutputs(endpoints map[string]config.EndpointConfig) []planEndpointOutput {
	names := make([]string, 0, len(endpoints))
	for name := range endpoints {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]planEndpointOutput, 0, len(names))
	for _, name := range names {
		endpoint := endpoints[name]
		result = append(result, planEndpointOutput{
			Name:  name,
			Label: endpoint.Label,
			URL:   endpoint.URL,
			Paths: endpoint.Paths,
		})
	}
	return result
}

func filterStatusByNames(status *lifecycle.AggregatedStatus, names []string) *lifecycle.AggregatedStatus {
	if status == nil || len(names) == 0 || len(status.Entries) == 0 {
		return status
	}
	nameSet := make(map[string]struct{}, len(names))
	for _, name := range names {
		nameSet[name] = struct{}{}
	}
	filtered := make([]lifecycle.EntryStatus, 0, len(status.Entries))
	for _, entry := range status.Entries {
		if _, ok := nameSet[entry.Name]; ok {
			filtered = append(filtered, entry)
		}
	}
	status.Entries = filtered
	return status
}
