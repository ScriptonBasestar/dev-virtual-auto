package lifecycle

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// CompositionPlan is the resolved form of a composes: plan (config.PlanConfig.Composes,
// frozen by TASK-260 §3) — a wave-ordered list of fully resolved child ExecutionPlans.
// Unlike ExecutionPlan, a CompositionPlan has no stack entries of its own: it resolves to
// multiple child ExecutionPlans, each owned and resolved exactly as TASK-262 already
// resolves a direct or imported-plan invocation.
type CompositionPlan struct {
	Name    string
	Entries []CompositionPlanEntry

	owner *config.Config
}

// OwnerConfig returns the configuration that declared this composition plan. Composition
// plans cannot themselves be imported (TASK-260 §3.3), so this is always the config whose
// Plans map was looked up to find it.
func (p *CompositionPlan) OwnerConfig(fallback *config.Config) *config.Config {
	if p != nil && p.owner != nil {
		return p.owner
	}
	return fallback
}

// CompositionPlanEntry is one composed child, placed into a wave by the same
// CalculateWaves algorithm (resolver.go:291) a leaf plan's stack entries use.
type CompositionPlanEntry struct {
	ChildPlan *ExecutionPlan
	Wave      int
	Order     int
	DependsOn []string
}

// ResolveCompositionPlan resolves a composes: plan into wave-numbered children.
//
// Ordering reuses CalculateWaves unchanged (TASK-260 §3.8) — CompositionEntry.Order and
// .DependsOn feed it exactly as PlanEntry.Order/.DependsOn do for stack entries, so no
// second DAG/ordering implementation exists for composed children.
//
// Each child is resolved with ResolvePlan against owner (the config that declared this
// composition plan) — the same owner-resolution TASK-262 already guarantees for a direct
// or imported-plan invocation. The root's own environment/site/vars never reach this call:
// a composition plan cannot declare them (validateCompositionPlan, composition_plan.go).
// Only CompositionEntry.Vars is passed through, as the cliVars argument — the same
// override-at-the-top-of-the-chain mechanism PlanEntry.Vars/CLI --var already use
// (TASK-260 §3.6: override, not merge).
func ResolveCompositionPlan(cfg *config.Config, planName string) (*CompositionPlan, error) {
	name, plan, err := ResolvePlanName(cfg, planName)
	if err != nil {
		return nil, &ResolveError{PlanName: planName, Step: "lookup", Cause: err}
	}
	if len(plan.Composes) == 0 {
		return nil, &ResolveError{PlanName: name, Step: "lookup", Cause: fmt.Errorf("plan %q is not a composition plan (composes:)", name)}
	}
	owner := plan.OwnerConfig(cfg)
	if owner == nil {
		return nil, &ResolveError{PlanName: name, Step: "owner", Cause: fmt.Errorf("plan owner config is nil")}
	}

	// Wave input built fresh from copied Order/DependsOn values so a later mutation of
	// plan.Composes (e.g. a config reload racing this resolve) cannot reach back into an
	// already-computed wave (TASK-260 §3.9).
	waveEntries := make([]ResolvedEntry, len(plan.Composes))
	for i, entry := range plan.Composes {
		childName := strings.TrimSpace(entry.Plan)
		if childName == "" {
			return nil, &ResolveError{PlanName: name, Step: "composes_ref", Cause: fmt.Errorf("composes[%d] has empty plan", i)}
		}
		waveEntries[i] = ResolvedEntry{
			Name:      childName,
			Order:     entry.Order,
			DependsOn: copyStringSlice(entry.DependsOn),
		}
	}
	if err := CalculateWaves(waveEntries); err != nil {
		return nil, &ResolveError{PlanName: name, Step: "waves", Cause: err}
	}

	resolved := &CompositionPlan{
		Name:    name,
		Entries: make([]CompositionPlanEntry, 0, len(plan.Composes)),
		owner:   owner,
	}

	for i, entry := range plan.Composes {
		childPlan, err := ResolvePlan(owner, entry.Plan, entry.Vars)
		if err != nil {
			return nil, &ResolveError{PlanName: name, Step: "compose_child", Cause: fmt.Errorf("composes[%d] %q: %w", i, entry.Plan, err)}
		}
		resolved.Entries = append(resolved.Entries, CompositionPlanEntry{
			ChildPlan: childPlan,
			Wave:      waveEntries[i].Wave,
			Order:     entry.Order,
			DependsOn: copyStringSlice(entry.DependsOn),
		})
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
		return a.ChildPlan.Name < b.ChildPlan.Name
	})

	return resolved, nil
}
