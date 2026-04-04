package lifecycle

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
)

type ExecutionPlan struct {
	Name            string
	EnvironmentName string
	SiteName        string
	EnvVars         map[string]string
	Entries         []ResolvedEntry
	ResolutionTrace []string
}

type ResolvedEntry struct {
	Name         string
	StackEntry   *config.LifecycleEntry
	Runner       string
	RunnerConfig any
	Order        int
	DependsOn    []string
	Services     []string
	Wave         int
	WorkingDir   string
	Vars         map[string]string
}

type ResolveError struct {
	PlanName string
	Step     string
	Cause    error
}

func (e *ResolveError) Error() string {
	return fmt.Sprintf("plan %q: %s: %v", e.PlanName, e.Step, e.Cause)
}

func (e *ResolveError) Unwrap() error {
	return e.Cause
}

func ResolvePlanName(cfg *config.Config, name string) (planName string, plan *config.PlanConfig, err error) {
	if cfg == nil {
		return "", nil, fmt.Errorf("nil config")
	}
	if len(cfg.Plans) == 0 {
		return "", nil, fmt.Errorf("no plans configured")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil, fmt.Errorf("plan name is empty")
	}
	resolved, ok := cfg.Plans[name]
	if !ok || resolved == nil {
		return "", nil, fmt.Errorf("plan %q not found", name)
	}
	return name, resolved, nil
}

func ResolvePlan(cfg *config.Config, planName string, cliVars map[string]string) (*ExecutionPlan, error) {
	name, plan, err := ResolvePlanName(cfg, planName)
	if err != nil {
		return nil, &ResolveError{PlanName: planName, Step: "lookup", Cause: err}
	}

	resolved := &ExecutionPlan{
		Name:            name,
		EnvironmentName: plan.Environment,
		SiteName:        plan.Site,
		EnvVars:         make(map[string]string),
		Entries:         make([]ResolvedEntry, 0, len(plan.Entries)),
		ResolutionTrace: make([]string, 0, 16),
	}

	resolved.ResolutionTrace = append(resolved.ResolutionTrace, "lookup: resolved plan")

	// TODO: Add dedicated config.LoadEnvFileVars helper and merge here directly.
	if cfg.EnvFile != nil {
		resolved.ResolutionTrace = append(resolved.ResolutionTrace, "vars: env_file merge skipped (TODO)")
	} else {
		resolved.ResolutionTrace = append(resolved.ResolutionTrace, "vars: env_file empty")
	}

	mergeStringMap(resolved.EnvVars, cfg.Vars)
	resolved.ResolutionTrace = append(resolved.ResolutionTrace, "vars: merged global vars")

	var envProfile *config.EnvironmentProfile
	if plan.Environment != "" {
		p, ok := cfg.Environments[plan.Environment]
		if !ok {
			return nil, &ResolveError{
				PlanName: name,
				Step:     "environment",
				Cause:    fmt.Errorf("environment %q not found", plan.Environment),
			}
		}
		envProfile = &p
		mergeStringMap(resolved.EnvVars, envProfile.Environment)
		resolved.ResolutionTrace = append(resolved.ResolutionTrace, fmt.Sprintf("vars: merged environment %q", plan.Environment))
	}

	var site *config.SiteConfig
	if plan.Site != "" {
		s, ok := cfg.Sites[plan.Site]
		if !ok || s == nil {
			return nil, &ResolveError{
				PlanName: name,
				Step:     "site",
				Cause:    fmt.Errorf("site %q not found", plan.Site),
			}
		}
		site = s
		mergeStringMap(resolved.EnvVars, site.Vars)
		resolved.ResolutionTrace = append(resolved.ResolutionTrace, fmt.Sprintf("vars: merged site %q", plan.Site))
	}

	mergeStringMap(resolved.EnvVars, plan.Vars)
	resolved.ResolutionTrace = append(resolved.ResolutionTrace, "vars: merged plan vars")

	mergeStringMap(resolved.EnvVars, cliVars)
	resolved.ResolutionTrace = append(resolved.ResolutionTrace, "vars: merged cli vars")

	if len(plan.Entries) == 0 {
		resolved.ResolutionTrace = append(resolved.ResolutionTrace, "entries: empty")
		return resolved, nil
	}

	if len(cfg.Stack) == 0 {
		return nil, &ResolveError{PlanName: name, Step: "stack_ref", Cause: fmt.Errorf("stack is empty")}
	}

	for i := range plan.Entries {
		planEntry := plan.Entries[i]
		entryName := strings.TrimSpace(planEntry.Name)
		if entryName == "" {
			return nil, &ResolveError{PlanName: name, Step: "stack_ref", Cause: fmt.Errorf("entry[%d] has empty name", i)}
		}

		stackEntry, ok := cfg.Stack[entryName]
		if !ok || stackEntry == nil {
			return nil, &ResolveError{
				PlanName: name,
				Step:     "stack_ref",
				Cause:    fmt.Errorf("stack entry %q not found", entryName),
			}
		}

		finalRunner := normalizeRunnerName(stackEntry.DefaultRunner)
		entryOverride := (*config.SiteEntryOverride)(nil)
		if site != nil && site.EntryOverrides != nil {
			entryOverride = site.EntryOverrides[entryName]
			if entryOverride != nil && strings.TrimSpace(entryOverride.Runner) != "" {
				finalRunner = normalizeRunnerName(entryOverride.Runner)
			}
		}
		if strings.TrimSpace(planEntry.Runner) != "" {
			finalRunner = normalizeRunnerName(planEntry.Runner)
		}

		if finalRunner == "" {
			if len(stackEntry.Runners) == 1 {
				for k := range stackEntry.Runners {
					finalRunner = normalizeRunnerName(k)
					break
				}
			} else {
				finalRunner = normalizeRunnerName(stackEntry.DetectPlugin())
			}
		}

		if finalRunner == "" {
			return nil, &ResolveError{
				PlanName: name,
				Step:     "runner",
				Cause:    fmt.Errorf("entry %q has no resolvable runner", entryName),
			}
		}

		if len(stackEntry.Runners) > 0 && !runnerDeclared(stackEntry.Runners, finalRunner) {
			return nil, &ResolveError{
				PlanName: name,
				Step:     "runner",
				Cause:    fmt.Errorf("entry %q runner %q is not declared in stack.runners", entryName, finalRunner),
			}
		}

		runnerConfig, err := stackEntry.GetRunnerConfig(finalRunner)
		if err != nil {
			return nil, &ResolveError{PlanName: name, Step: "runner_config", Cause: fmt.Errorf("entry %q: %w", entryName, err)}
		}

		resolvedEntry := ResolvedEntry{
			Name:         entryName,
			StackEntry:   stackEntry,
			Runner:       finalRunner,
			RunnerConfig: runnerConfig,
			Order:        planEntry.Order,
			DependsOn:    copyStringSlice(planEntry.DependsOn),
			Services:     copyStringSlice(planEntry.Services),
			Wave:         0,
			Vars:         make(map[string]string),
		}

		mergeStringMap(resolvedEntry.Vars, resolved.EnvVars)
		mergeStringMap(resolvedEntry.Vars, stackEntry.Vars)
		if entryOverride != nil {
			mergeStringMap(resolvedEntry.Vars, entryOverride.Vars)
		}
		mergeStringMap(resolvedEntry.Vars, planEntry.Vars)

		switch rc := runnerConfig.(type) {
		case *config.ProcessPluginConfig:
			resolvedEntry.WorkingDir = rc.Dir
		case *config.NativeRunnerConfig:
			resolvedEntry.WorkingDir = rc.Dir
		}

		resolved.Entries = append(resolved.Entries, resolvedEntry)
		resolved.ResolutionTrace = append(
			resolved.ResolutionTrace,
			fmt.Sprintf("entry: %s -> runner=%s order=%d deps=%d", entryName, finalRunner, resolvedEntry.Order, len(resolvedEntry.DependsOn)),
		)
	}

	if err := CalculateWaves(resolved.Entries); err != nil {
		return nil, &ResolveError{PlanName: name, Step: "waves", Cause: err}
	}

	sort.Slice(resolved.Entries, func(i, j int) bool {
		a := resolved.Entries[i]
		b := resolved.Entries[j]
		if a.Wave != b.Wave {
			return a.Wave < b.Wave
		}
		if a.Order != b.Order {
			return a.Order < b.Order
		}
		return a.Name < b.Name
	})

	resolved.ResolutionTrace = append(resolved.ResolutionTrace, "waves: calculated and entries sorted")

	return resolved, nil
}

func CalculateWaves(entries []ResolvedEntry) error {
	if len(entries) == 0 {
		return nil
	}

	nameToIdx := make(map[string]int, len(entries))
	for i := range entries {
		name := strings.TrimSpace(entries[i].Name)
		if name == "" {
			return fmt.Errorf("entry[%d] has empty name", i)
		}
		if _, exists := nameToIdx[name]; exists {
			return fmt.Errorf("duplicate entry name %q", name)
		}
		nameToIdx[name] = i
	}

	inDegree := make([]int, len(entries))
	outgoing := make([][]int, len(entries))
	for i := range entries {
		for _, depName := range entries[i].DependsOn {
			depName = strings.TrimSpace(depName)
			if depName == "" {
				continue
			}
			depIdx, ok := nameToIdx[depName]
			if !ok {
				return fmt.Errorf("entry %q depends_on unknown entry %q", entries[i].Name, depName)
			}
			outgoing[depIdx] = append(outgoing[depIdx], i)
			inDegree[i]++
		}
	}

	queue := make([]int, 0, len(entries))
	for i := range entries {
		if inDegree[i] == 0 {
			entries[i].Wave = 0
			queue = append(queue, i)
		}
	}

	processed := 0
	for len(queue) > 0 {
		idx := queue[0]
		queue = queue[1:]
		processed++

		for _, next := range outgoing[idx] {
			if entries[next].Wave < entries[idx].Wave+1 {
				entries[next].Wave = entries[idx].Wave + 1
			}
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if processed != len(entries) {
		return fmt.Errorf("cycle detected in depends_on graph")
	}

	return nil
}

func mergeStringMap(dst map[string]string, src map[string]string) {
	if len(src) == 0 {
		return
	}
	for k, v := range src {
		dst[k] = v
	}
}

func copyStringSlice(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func normalizeRunnerName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return ""
	}
	return strings.ReplaceAll(name, "_", "-")
}

func runnerDeclared(runners map[string]any, runner string) bool {
	runner = normalizeRunnerName(runner)
	for k := range runners {
		if normalizeRunnerName(k) == runner {
			return true
		}
	}
	return false
}
