package config

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// TASK-308: reference-integrity warnings. Each check names a declaration that
// validates clean structurally but is never consulted at runtime, or points at
// something that does not exist. None of them is a hard error: the config still
// runs, it just does not do what the author thinks it does.

// warnPlanServicesNotDeclared warns when plans.<p>.entries[<e>].services names a
// service that stack.<e>'s compose runner does not declare under services:.
//
// The compose runner's services: map is optional — when absent the entry runs
// whatever the compose file defines and this pass cannot see the file, so it
// stays quiet. When the map is present it is the entry's declared surface, and a
// plan selecting a name outside it starts nothing at runtime (compose reports
// "no such service" only after the plan has already begun).
func (c *Config) warnPlanServicesNotDeclared() []string {
	var warnings []string
	for _, planName := range sortedPlanNames(c) {
		plan := c.Plans[planName]
		for i, pe := range plan.Entries {
			if len(pe.Services) == 0 {
				continue
			}
			entry, ok := c.Stack[pe.Name]
			if !ok || entry == nil {
				continue // undeclared entries are reported elsewhere
			}
			cc := entry.ComposeConfig()
			if cc == nil || len(cc.Services) == 0 {
				continue
			}
			var missing []string
			for _, svc := range pe.Services {
				if _, declared := cc.Services[svc]; !declared {
					missing = append(missing, svc)
				}
			}
			if len(missing) == 0 {
				continue
			}
			declared := make([]string, 0, len(cc.Services))
			for name := range cc.Services {
				declared = append(declared, name)
			}
			sort.Strings(declared)
			warnings = append(warnings, fmt.Sprintf(
				"plans.%s.entries[%d].services: %s not declared under stack.%s.runners.compose.services (declared: %s); either the plan names a service the compose file lacks, or the services map is stale — add the service there so tags and validation see it",
				planName, i, strings.Join(missing, ", "), pe.Name, strings.Join(declared, ", ")))
		}
	}
	return warnings
}

// warnUnreferencedEnvironmentsAndSites warns when an environments.* or sites.*
// profile is declared but no plan selects it via environment:/site:.
//
// On the plan path those two fields are the only way a profile is consulted, so
// an unreferenced profile is dead config. The pass is skipped when no plans are
// declared: a legacy (modes-only) config reaches environments through the -E
// flag, and warnNoPlansHint already covers that shape.
func (c *Config) warnUnreferencedEnvironmentsAndSites() []string {
	if len(c.Plans) == 0 {
		return nil
	}
	envUsed := make(map[string]bool)
	siteUsed := make(map[string]bool)
	envDynamic, siteDynamic := false, false
	for _, plan := range c.Plans {
		if plan == nil {
			continue
		}
		// `environment: ${ENV:-dev}` selects at run time; which profiles it can
		// reach is not decidable here, so the whole check stays quiet for that axis.
		if strings.Contains(plan.Environment, "${") {
			envDynamic = true
		} else if plan.Environment != "" {
			envUsed[plan.Environment] = true
		}
		if strings.Contains(plan.Site, "${") {
			siteDynamic = true
		} else if plan.Site != "" {
			siteUsed[plan.Site] = true
		}
	}

	var warnings []string
	for _, name := range sortedKeysOf(c.Environments) {
		if !envDynamic && !envUsed[name] {
			warnings = append(warnings, fmt.Sprintf(
				"environments.%s: no plan selects it via environment:, so its values never apply; reference it from a plan or remove it", name))
		}
	}
	for _, name := range sortedKeysOf(c.Sites) {
		if !siteDynamic && !siteUsed[name] {
			warnings = append(warnings, fmt.Sprintf(
				"sites.%s: no plan selects it via site:, so its vars and entry_overrides never apply; reference it from a plan or remove it", name))
		}
	}
	return warnings
}

// warnNoOpEntryOverrides warns about sites.<s>.entry_overrides.<e> declarations
// that cannot change anything at runtime:
//
//   - <e> is not a stack entry, so the resolver never looks the override up;
//   - the override sets neither runner nor vars;
//   - the override's runner equals the entry's default_runner and it sets no vars.
//
// Overrides on a site no plan selects are covered by
// warnUnreferencedEnvironmentsAndSites and are not repeated here.
func (c *Config) warnNoOpEntryOverrides() []string {
	var warnings []string
	for _, siteName := range sortedKeysOf(c.Sites) {
		site := c.Sites[siteName]
		if site == nil {
			continue
		}
		for _, entryName := range sortedKeysOf(site.EntryOverrides) {
			ov := site.EntryOverrides[entryName]
			prefix := fmt.Sprintf("sites.%s.entry_overrides.%s", siteName, entryName)
			entry, declared := c.Stack[entryName]
			if !declared || entry == nil {
				warnings = append(warnings, fmt.Sprintf(
					"%s: %q is not a stack entry, so the override is never applied; fix the name or remove it", prefix, entryName))
				continue
			}
			if ov == nil || (strings.TrimSpace(ov.Runner) == "" && len(ov.Vars) == 0) {
				warnings = append(warnings, fmt.Sprintf(
					"%s: sets neither runner nor vars, so it changes nothing; remove it", prefix))
				continue
			}
			if len(ov.Vars) == 0 && normalizeRunnerKey(ov.Runner) == normalizeRunnerKey(entry.DefaultRunner) {
				warnings = append(warnings, fmt.Sprintf(
					"%s: runner %q is already stack.%s.default_runner and no vars are set, so it changes nothing; remove it",
					prefix, ov.Runner, entryName))
			}
		}
	}
	return warnings
}

// warnEmptyInteractionCommands warns when an interaction command (at any depth)
// declares no execution target and has no subcommands to dispatch to. Such a node
// is reachable — `dva run <path>` resolves it — but has nothing to execute, which
// is almost always a `command: ""` left behind by an edit.
func (c *Config) warnEmptyInteractionCommands() []string {
	var warnings []string
	eachInteractionNode(c.Interaction, func(path string, cmd *InteractionCommand, _ inheritedExec) {
		if cmd.hasExecutionTarget() || len(cmd.Subcommands) > 0 {
			return
		}
		warnings = append(warnings, fmt.Sprintf(
			"%s: declares no command, script, steps, or subcommands, so 'dva run %s' has nothing to execute; add a command or remove the entry",
			path, strings.ReplaceAll(strings.TrimPrefix(path, "interaction."), ".subcommands.", " ")))
	})
	sort.Strings(warnings)
	return warnings
}

// removedCLI maps a removed dva command surface to its current replacement. The
// keys are matched as `dva <key>` in free text (descriptions, notes, command
// strings); -M/--mode is matched separately because it is a flag, not a verb.
// docs/43 records the removal.
var removedCLI = []struct{ verb, replacement string }{
	{"stack", "dva <up|down|...> <plan>"},
	{"app", "a stack entry with a native runner, selected by a plan"},
	{"infra", "dva up <plan>"},
	{"clean", "dva down <plan> --purge"},
	{"dev", "dva up <plan>"},
}

// RemovedCommands returns the removed top-level dva verbs (docs/43) mapped to the
// current replacement. It is the sibling of ReservedCommands for text that still
// references the old surface.
func RemovedCommands() map[string]string {
	out := make(map[string]string, len(removedCLI))
	for _, r := range removedCLI {
		out[r.verb] = r.replacement
	}
	return out
}

var removedVerbRe = func() map[string]*regexp.Regexp {
	out := make(map[string]*regexp.Regexp, len(removedCLI))
	for _, r := range removedCLI {
		out[r.verb] = regexp.MustCompile(`\bdva\s+` + r.verb + `\b`)
	}
	return out
}()

var removedModeFlagRe = regexp.MustCompile(`\bdva\b[^\n;&|]*\s(-M|--mode)(?:[\s=]|$)`)

// warnRemovedCLIReferences warns when a human-facing string still tells the
// reader to run a dva command that no longer exists: `dva stack|app|infra|clean|dev`
// or a dva invocation carrying -M/--mode. Only text the loader keeps is scanned
// (descriptions, step notes, start_hint, interaction command strings); YAML
// comments are gone before validation runs.
//
// A verb the user redefined as their own interaction (e.g. interaction.clean) is
// legitimately runnable as `dva clean`, so it is excluded from the scan.
func (c *Config) warnRemovedCLIReferences() []string {
	type located struct{ path, text string }
	var texts []located
	add := func(path, text string) {
		if strings.TrimSpace(text) != "" {
			texts = append(texts, located{path, text})
		}
	}

	for _, name := range sortedKeysOf(c.Stack) {
		if e := c.Stack[name]; e != nil {
			add("stack."+name+".description", e.Description)
		}
	}
	for _, name := range sortedPlanNames(c) {
		add("plans."+name+".description", c.Plans[name].Description)
	}
	for _, name := range sortedKeysOf(c.Sites) {
		if s := c.Sites[name]; s != nil {
			add("sites."+name+".description", s.Description)
		}
	}
	for _, name := range sortedKeysOf(c.Environments) {
		add("environments."+name+".description", c.Environments[name].Description)
	}
	for _, name := range sortedKeysOf(c.HealthChecks) {
		add("health_checks."+name+".start_hint", c.HealthChecks[name].StartHint)
	}
	eachInteractionNode(c.Interaction, func(path string, cmd *InteractionCommand, _ inheritedExec) {
		add(path+".description", cmd.Description)
		add(path+".command", cmd.Command)
		for i, line := range cmd.CommandLines {
			add(fmt.Sprintf("%s.command[%d]", path, i), line)
		}
		for i, step := range cmd.Steps {
			add(fmt.Sprintf("%s.steps[%d].note", path, i), step.Note)
			add(fmt.Sprintf("%s.steps[%d].step", path, i), step.Step)
			if s, ok := step.Run.(string); ok {
				add(fmt.Sprintf("%s.steps[%d].run", path, i), s)
			}
		}
	})

	var warnings []string
	for _, t := range texts {
		for _, r := range removedCLI {
			if _, own := c.Interaction[r.verb]; own {
				continue
			}
			if !removedVerbRe[r.verb].MatchString(t.text) {
				continue
			}
			warnings = append(warnings, fmt.Sprintf(
				"%s: mentions 'dva %s', which was removed (docs/43); use %s instead",
				t.path, r.verb, r.replacement))
		}
		if m := removedModeFlagRe.FindStringSubmatch(t.text); m != nil {
			warnings = append(warnings, fmt.Sprintf(
				"%s: mentions '%s', which was removed (docs/43); select a plan by name instead: dva up <plan>",
				t.path, m[1]))
		}
	}
	sort.Strings(warnings)
	return warnings
}

// warnOrphanHealthChecks warns when a top-level health_checks entry is referenced
// by no modes.*.health_checks list. Top-level checks are consulted only through a
// mode reference (see warnUnreachableHealthChecks); on a plan-based config with no
// modes every top-level check is dead config, and the fix is to move it under the
// stack entry it guards. Checks with start/start_hint are already reported by
// warnUnreachableHealthChecks and are skipped here to avoid a duplicate line.
func (c *Config) warnOrphanHealthChecks() []string {
	referenced := make(map[string]bool)
	for _, mode := range c.Modes {
		for _, name := range mode.HealthChecks {
			referenced[name] = true
		}
	}
	var warnings []string
	for _, name := range sortedKeysOf(c.HealthChecks) {
		hc := c.HealthChecks[name]
		if referenced[name] || hc.Start != "" || hc.StartHint != "" {
			continue
		}
		warnings = append(warnings, fmt.Sprintf(
			"health_checks.%s: no modes.*.health_checks entry references it and plans do not read top-level health_checks, so it never runs; move it under stack.<entry>.health_checks or remove it", name))
	}
	return warnings
}

func sortedPlanNames(c *Config) []string {
	names := make([]string, 0, len(c.Plans))
	for name, p := range c.Plans {
		if p != nil {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func sortedKeysOf[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func normalizeRunnerKey(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
