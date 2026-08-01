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
	args = planRoutingArgs(args)
	if len(args) > 0 || c.DefaultPlan() != "" {
		return nil
	}

	return fmt.Errorf("multiple plans configured; specify one: dva %s <%s>", command, strings.Join(sortedPlanNames(c), "|"))
}

func detectPlanRoute(c *config.Config, args []string) (planName string, extraArgs []string, ok bool) {
	if c == nil || !c.HasPlans() {
		return "", nil, false
	}
	args = planRoutingArgs(args)

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

// rejectSuppressedDefaultPlan refuses silent whole-stack fallthrough when exactly
// one plan exists (DefaultPlan) but leading args prevented detectPlanRoute from
// selecting it. Name the plan explicitly instead (e.g. dva up p1 --dev).
//
// Non-flag tokens are left to rejectUnknownPlanArg / rejectUpPositionalArg so
// unknown plan names keep their existing messages.
func rejectSuppressedDefaultPlan(c *config.Config, command string, args []string) error {
	if c == nil || !c.HasPlans() || len(args) == 0 {
		return nil
	}
	def := c.DefaultPlan()
	if def == "" {
		return nil
	}
	if _, exists := c.Plans[args[0]]; exists {
		return nil
	}
	if !strings.HasPrefix(args[0], "-") {
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
	if c == nil || !c.HasPlans() || len(args) == 0 {
		return nil
	}
	name := args[0]
	if strings.HasPrefix(name, "-") {
		return nil
	}
	return fmt.Errorf("plan '%s' not found. Available: %s", name, strings.Join(sortedPlanNames(c), ", "))
}

// rejectUpPositionalArg guards the plan-name slot of 'up', which advertises
// "up [OPTIONS]" and so has no positional argument that means anything else.
// It reads args the same way rejectUnknownPlanArg does — only args[0], and a
// leading '-' returns early — so flag values such as '--var FOO=x' are never
// mistaken for a plan name.
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
	if strings.HasPrefix(name, "-") {
		return nil
	}
	if c == nil || !c.HasPlans() {
		return fmt.Errorf("unexpected argument '%s': 'dva up' takes no positional arguments and no plans are configured. Run 'dva up' to start the whole stack, or 'dva stack up %s' to start a single entry", name, name)
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

	plan, err := lifecycle.ResolvePlan(c, planName, flags.cliVars)
	if err != nil {
		return err
	}

	e.MergeVars(plan.EnvVars)
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

	plan, err := lifecycle.ResolvePlan(c, planName, flags.cliVars)
	if err != nil {
		return err
	}

	e.MergeVars(plan.EnvVars)
	fmt.Fprintf(os.Stderr, "[plan: %s] environment=%s site=%s entries=%d\n", plan.Name, plan.EnvironmentName, plan.SiteName, len(plan.Entries))

	effectiveDryRun := dryRun || flags.dryRun
	if effectiveDryRun {
		printPlanResolution(plan)
	}

	orch, err := lifecycle.NewPlanOrchestrator(c, e, plan)
	if err != nil {
		return err
	}
	return orch.Down(context.Background(), lifecycle.DownOptions{
		DryRun:  effectiveDryRun,
		Volumes: flags.volumes,
		Names:   planEntryNames(plan),
		Env:     plan.EnvironmentName,
	})
}

func runPlanStop(c *config.Config, e *config.Environment, planName string, extraArgs []string) error {
	flags, err := parsePlanFlags(extraArgs)
	if err != nil {
		return err
	}

	plan, err := lifecycle.ResolvePlan(c, planName, flags.cliVars)
	if err != nil {
		return err
	}

	e.MergeVars(plan.EnvVars)
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
	if flags.volumes {
		return fmt.Errorf("--volumes is only supported by down")
	}

	plan, err := lifecycle.ResolvePlan(c, planName, flags.cliVars)
	if err != nil {
		return err
	}

	e.MergeVars(plan.EnvVars)
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
	plan, err := lifecycle.ResolvePlan(c, planName, nil)
	if err != nil {
		return err
	}

	e.MergeVars(plan.EnvVars)
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
