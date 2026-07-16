package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/lifecycle"
)

type planRunFlags struct {
	cliVars map[string]string
	dryRun  bool
	force   bool
	wait    bool
	volumes bool
}

func requirePlanSelection(c *config.Config, command string, args []string) error {
	if c == nil || !c.HasPlans() || len(args) > 0 || c.DefaultPlan() != "" {
		return nil
	}

	names := make([]string, 0, len(c.Plans))
	for name := range c.Plans {
		names = append(names, name)
	}
	sort.Strings(names)
	return fmt.Errorf("multiple plans configured; specify one: dva %s <%s>", command, strings.Join(names, "|"))
}

func detectPlanRoute(c *config.Config, args []string) (planName string, extraArgs []string, ok bool) {
	if c == nil || !c.HasPlans() {
		return "", nil, false
	}

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
	available := make([]string, 0, len(c.Plans))
	for plan := range c.Plans {
		available = append(available, plan)
	}
	sort.Strings(available)
	return fmt.Errorf("plan '%s' not found. Available: %s", name, strings.Join(available, ", "))
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

	orch, err := lifecycle.NewPlanOrchestrator(c, e, plan)
	if err != nil {
		return err
	}
	if err := orch.Up(context.Background(), lifecycle.UpOptions{
		DryRun: dryRun || flags.dryRun,
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
		lifecycle.PrintStatus(filterStatusByNames(status, planEntryNames(plan)), c.FileDir())
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

	orch, err := lifecycle.NewPlanOrchestrator(c, e, plan)
	if err != nil {
		return err
	}
	return orch.Down(context.Background(), lifecycle.DownOptions{
		DryRun:  dryRun || flags.dryRun,
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

	orch, err := lifecycle.NewPlanOrchestrator(c, e, plan)
	if err != nil {
		return err
	}
	return orch.Stop(context.Background(), lifecycle.StopOptions{
		DryRun: dryRun || flags.dryRun,
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

	orch, err := lifecycle.NewPlanOrchestrator(c, e, plan)
	if err != nil {
		return err
	}
	return orch.Restart(context.Background(), lifecycle.UpOptions{
		DryRun: dryRun || flags.dryRun,
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

	lifecycle.PrintStatus(filterStatusByNames(status, planEntryNames(plan)), c.FileDir())
	return nil
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
