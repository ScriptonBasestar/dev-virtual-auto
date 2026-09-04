package config

import (
	"fmt"
	"sort"
)

// CompositionEntry references a local leaf plan or an already-imported
// canonical/alias plan name to run as part of a composition plan
// (PlanConfig.Composes). TASK-260 §3.1 froze this shape.
type CompositionEntry struct {
	Plan      string            `yaml:"plan"`
	Order     int               `yaml:"order"`
	DependsOn []string          `yaml:"depends_on"`
	Vars      map[string]string `yaml:"vars"`
}

// validateCompositionPlans enforces TASK-260 §3 for every plan in cfg.Plans
// that declares composes:. It runs after subproject imports are resolved, so
// composes[].plan can be checked by direct lookup in cfg.Plans — that map
// already holds every local plan plus every imported canonical/alias name
// (§3.2's "already-imported" requirement falls out of that lookup: a name
// nothing imported simply is not there).
//
// Sorted by plan name so a config with two violations reports the same one on
// every run, matching validateHookPlacement's precedent (validate.go).
func validateCompositionPlans(cfg *Config) error {
	names := make([]string, 0, len(cfg.Plans))
	for name, plan := range cfg.Plans {
		if plan != nil && len(plan.Composes) > 0 {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	for _, name := range names {
		if err := validateCompositionPlan(cfg, name, cfg.Plans[name]); err != nil {
			return err
		}
	}
	return nil
}

func validateCompositionPlan(cfg *Config, name string, plan *PlanConfig) error {
	// §3.1: entries: and composes: are mutually exclusive on one plan.
	if len(plan.Entries) > 0 {
		return fmt.Errorf("plan %q declares both entries: and composes: — a plan is either a leaf plan (entries:) or a composition plan (composes:), not both", name)
	}

	// §3.6: a composition plan has no stack entries of its own to apply
	// environment/site/vars to.
	if plan.Environment != "" {
		return fmt.Errorf("plan %q is a composition plan (composes:) and cannot declare environment: — each composed child keeps its own owning environment", name)
	}
	if plan.Site != "" {
		return fmt.Errorf("plan %q is a composition plan (composes:) and cannot declare site: — each composed child keeps its own owning site", name)
	}
	if len(plan.Vars) > 0 {
		return fmt.Errorf("plan %q is a composition plan (composes:) and cannot declare top-level vars: — use composes[].vars to override a specific composed child", name)
	}

	seenNames := make(map[string]bool, len(plan.Composes))
	seenTargets := make(map[*PlanConfig]bool, len(plan.Composes))
	for i, entry := range plan.Composes {
		target := entry.Plan

		// §3.4: the same Plan value cannot appear twice in one composes: list.
		if seenNames[target] {
			return fmt.Errorf("plan %q composes[%d]: duplicate composed plan %q", name, i, target)
		}
		seenNames[target] = true

		// §3.2: composes[].plan must resolve to a local leaf plan or an
		// already-imported canonical/alias name. Nothing is loaded to satisfy
		// this — cfg.Plans already contains every name composition is allowed
		// to reference.
		resolved, ok := cfg.Plans[target]
		if !ok || resolved == nil {
			return fmt.Errorf("plan %q composes[%d]: %q is not a local plan or an already-imported subproject plan (import it under subprojects.<name>.import.plans first)", name, i, target)
		}

		// §3.4 (continued): two aliases resolving to the same canonical import
		// are the same target — cloneImportedPlan gives canonical and alias
		// names the same *PlanConfig, so pointer identity catches this.
		if seenTargets[resolved] {
			return fmt.Errorf("plan %q composes[%d]: %q resolves to a plan already composed under a different name in this list", name, i, target)
		}
		seenTargets[resolved] = true

		// §3.3: composition of composition is rejected outright — not a cycle
		// detector, a flat "the target cannot itself be a composition plan"
		// rule. Applies identically to local and imported targets because both
		// are reached through the same cfg.Plans lookup above.
		if len(resolved.Composes) > 0 {
			return fmt.Errorf("plan %q composes[%d]: %q is itself a composition plan (composes:) — composition plans cannot compose another composition plan", name, i, target)
		}
	}

	return nil
}
