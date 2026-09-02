package config

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// migrationGuideURL is printed in validate warnings, so it is the link users are most
// likely to click. Repo dva on branch master — not "main", which this repo has never had.
// It named dev-virtual-auto until the repo was renamed (TASK-060); that name still resolves
// through GitHub's rename redirect, but the redirect dies if the old name is ever reused.
const migrationGuideURL = "https://github.com/ScriptonBasestar/dva/blob/master/docs/42-migration-and-compatibility.md#11-migration"

// canonicalSectionOrder defines the recommended top-level key order for dva.yml.
var canonicalSectionOrder = []string{
	"version", "vars", "environment", "env_file", "stack", "plans",
	"default_plan", "environments", "sites",
	// Legacy sections retain a deterministic position during migration.
	// `applications` is absent by removal, not by oversight (docs/43): the key no
	// longer validates, so a file carrying it is rejected before this order check
	// ever runs — listing it here would only describe where a key that cannot
	// appear would have gone.
	"checks", "default_mode", "suggestion_ignore", "modes",
	"health_checks", "interaction", "provision", "modules", "subprojects",
	"endpoints", "infra", "ssh", "devcontainer",
}

// canonicalOrderIndex maps section name to its position in canonical order.
var canonicalOrderIndex map[string]int

func init() {
	canonicalOrderIndex = make(map[string]int, len(canonicalSectionOrder))
	for i, name := range canonicalSectionOrder {
		canonicalOrderIndex[name] = i
	}
}

// CanonicalSectionOrder returns a copy of the recommended top-level dva.yml key order.
// This is the canonical source for the section-order list embedded in
// agent-mesh-flows/shared/library/shared-guardrails.md by tools/libgen.
func CanonicalSectionOrder() []string {
	return slices.Clone(canonicalSectionOrder)
}

// ValidateWarnings runs semantic warning checks and returns human-readable messages.
// These are non-fatal issues that should be surfaced by `dva config validate`.
func (c *Config) ValidateWarnings() []string {
	var warnings []string
	warnings = append(warnings, c.warnLegacyModes()...)
	warnings = append(warnings, c.warnLegacyStackOrder()...)
	warnings = append(warnings, c.warnLegacyEnvironmentFields()...)
	warnings = append(warnings, c.warnNoPlansHint()...)
	warnings = append(warnings, c.warnHealthCheckRedundancy()...)
	warnings = append(warnings, c.warnUnreachableHealthChecks()...)
	warnings = append(warnings, c.warnDuplicateParentSubcommand()...)
	warnings = append(warnings, c.warnDuplicateStackOrder()...)
	warnings = append(warnings, c.warnMultiStackComposeSplit()...)
	warnings = append(warnings, c.warnMissingDefaultMode()...)
	warnings = append(warnings, c.warnDefaultModeHeavyInfra()...)
	warnings = append(warnings, c.warnChildOverridesParentCritical()...)
	warnings = append(warnings, c.warnDeepSubcommandNesting()...)
	warnings = append(warnings, c.warnLiteralKeyShadowsSubproject()...)
	warnings = append(warnings, c.warnUnreachableCommands()...)
	warnings = append(warnings, c.warnInertProvisionSteps()...)
	warnings = append(warnings, c.warnIgnoredParallelSteps()...)
	warnings = append(warnings, c.warnDuplicatePlanDeclarations()...)
	warnings = append(warnings, c.warnMultiplePlansWithoutDefault()...)

	// Build a contextual environment for accurate interpolation checks.
	//
	// TASK-248: validation performs no env-file I/O. Structural validation has to
	// keep working when those files are missing or unreadable — availability is
	// doctor's and the runtime's to report, not the schema validator's. The cost
	// is that a reference satisfied only by an env file is no longer decidable
	// here, so the unresolved-variable check defers rather than reporting a
	// variable it can no longer see. It does not become a new warning category
	// and it does not claim success.
	env := NewEnvironment(c.Environment, c.FileDir(), c.FileDir())

	warnings = append(warnings, c.warnUnresolvedEnvVars(env, c.EnvFile != nil)...)
	warnings = append(warnings, c.warnSuspiciousEnvPatterns()...)

	if c.filePath != "" {
		warnings = append(warnings, validateCanonicalOrder(c.filePath)...)
	}
	return warnings
}

// warnLegacyModes warns when legacy `modes` are present and suggests migration.
func (c *Config) warnLegacyModes() []string {
	if len(c.Modes) == 0 {
		return nil
	}

	return []string{
		fmt.Sprintf("⚠ 'modes' section detected — consider migrating to 'plans' + 'environments' + 'sites'\n  Migration guide: %s\n  Hint: modes will continue to work but are deprecated in favor of the new plans model", migrationGuideURL),
	}
}

// warnLegacyStackOrder warns when `stack.*.order` is used and suggests moving order to plan entries.
func (c *Config) warnLegacyStackOrder() []string {
	if len(c.Stack) == 0 {
		return nil
	}

	var affected []string
	for name, entry := range c.Stack {
		if entry != nil && entry.Order > 0 {
			affected = append(affected, name)
		}
	}
	if len(affected) == 0 {
		return nil
	}

	sort.Strings(affected)
	return []string{
		fmt.Sprintf("⚠ 'stack.*.order' detected — execution order must move to 'plans.*.entries[].order'\n  Run: dva config migrate (preview) / dva config migrate --write (apply)\n  Migration guide: %s\n  Affected entries: %s\n  Hint: a plan entry's order is the one that runs, with no fallback to the declaration — an order left here is not read on the plan path", migrationGuideURL, strings.Join(affected, ", ")),
	}
}

// warnLegacyEnvironmentFields warns when deprecated environment stack fields are used.
func (c *Config) warnLegacyEnvironmentFields() []string {
	if len(c.Environments) == 0 {
		return nil
	}

	var affected []string
	for envName, profile := range c.Environments {
		if len(profile.Stack) > 0 || len(profile.StackOverrides) > 0 {
			affected = append(affected, envName)
		}
	}
	if len(affected) == 0 {
		return nil
	}

	sort.Strings(affected)
	return []string{
		fmt.Sprintf("⚠ 'environments.*.stack/stack_overrides' detected — these fields are deprecated\n  Migration guide: %s\n  Affected environments: %s\n  Hint: environments should use 'vars' only; stack selection belongs in plans", migrationGuideURL, strings.Join(affected, ", ")),
	}
}

// warnNoPlansHint emits a migration hint when stack exists but no plans are defined.
func (c *Config) warnNoPlansHint() []string {
	if len(c.Stack) == 0 || len(c.Plans) > 0 {
		return nil
	}

	return []string{
		fmt.Sprintf("ℹ No 'plans' defined — consider adding execution plans for 'dva up <name>' support\n  Migration guide: %s\n  Hint: plans combine stack entries, environments, and sites into named execution targets\n  Example:\n    plans:\n      local-dev:\n        environment: dev\n        site: local\n        entries:\n          - name: <stack-entry>\n            runner: <runner-name>\n            order: 10", migrationGuideURL),
	}
}

// No warning exists for a config version below the running binary, and none should.
// `version:` is the minimum DVA a config requires (USAGE.md), so config < binary is
// the correct, portable state: the binary satisfies the floor. The removed
// warnVersionOutdated advised raising the floor to match the running binary, which
// would strand every user on an older DVA and ratchet upward on every release.
// config > binary is the only real failure and Load() already rejects it.

// warnInertProvisionSteps flags provision and hook items that carry a label and no payload.
//
// A warning rather than an error, deliberately. Such an item is always a mistake — `note:`
// is what an author uses to print a message — but rejecting it would turn a config that
// validates today into one that fails, and the item has been quietly doing nothing since
// long before this check existed. The runtime notice (InertStepMessage, printed by every
// step runner) is the part that reaches the author at the moment the hook misbehaves;
// `validate` is not what anyone runs when a build silently produces nothing.
//
// Sorted, because both sources are maps and an unsorted result would reorder between runs.
func (c *Config) warnInertProvisionSteps() []string {
	var warnings []string

	collect := func(path string, items []ProvisionItem) {
		for i, item := range items {
			if !item.IsInert() {
				continue
			}
			label := item.Step
			if label == "" {
				label = fmt.Sprintf("step %d", i+1)
			}
			warnings = append(warnings, fmt.Sprintf("%s[%d] %q: %s", path, i, label, InertStepMessage))
		}
	}

	// Recursive, because hooks nest: `interaction.db.subcommands.migrate.before` is as real
	// a place to write an inert step as the top level, and a check that stopped at depth 1
	// would report the shallow mistake and stay silent on the identical deep one.
	//
	// Uses the shared walker rather than its own. It previously joined segments with a bare
	// dot, producing `interaction.db.migrate.before[0]` — a path that does not exist in the
	// document, since `migrate` lives under `db.subcommands`. A user searching their file for
	// it finds nothing. TASK-128.
	eachInteractionNode(c.Interaction, func(path string, cmd *InteractionCommand, _ inheritedExec) {
		collect(path+".steps", cmd.Steps)
		collect(path+".before", cmd.Before)
		collect(path+".replace", cmd.Replace)
		collect(path+".after", cmd.After)
	})

	for profile, items := range c.Provision.Profiles {
		collect(fmt.Sprintf("provision.%s", profile), items)
	}

	sort.Strings(warnings)
	return warnings
}

// warnIgnoredParallelSteps flags interaction steps that ask for concurrency the interaction
// path does not implement.
//
// `parallel:` reaches `interaction.*.steps` only because that field and `provision.*` share
// the ProvisionItem type, and schema.json documents the type. `Parallel` appears zero times
// in non-test code under internal/runner/, so runStepLoop has no batching: the key parses,
// validates, and is dropped. Measured — two `sleep 1` steps both marked parallel take 2.02s
// under `dva run` and 1.01s under `dva provision`, off one config.
//
// A warning rather than an error, for the reason warnInertProvisionSteps records: the key has
// been quietly doing nothing since it existed, and rejecting it would fail configs that
// validate today. The difference from an inert step is why the runtime notice matters more
// here — an inert step betrays itself by producing nothing, while this one produces exactly
// the right output and merely takes twice as long, so `validate` alone would reach nobody who
// was not already suspicious.
//
// Provision items are deliberately not walked: there the key works.
func (c *Config) warnIgnoredParallelSteps() []string {
	var warnings []string

	collect := func(path string, items []ProvisionItem) {
		for i, item := range items {
			if !item.Parallel {
				continue
			}
			label := item.Step
			if label == "" {
				label = fmt.Sprintf("step %d", i+1)
			}
			warnings = append(warnings, fmt.Sprintf("%s[%d] %q: %s", path, i, label, IgnoredParallelMessage))
		}
	}

	// Hooks as well as steps, and recursively: `interaction.db.subcommands.migrate.before` runs
	// through the same loop with the same absent scheduler, so a check that stopped at
	// `.steps` would report the shallow case and stay silent on the identical deep one.
	eachInteractionNode(c.Interaction, func(path string, cmd *InteractionCommand, _ inheritedExec) {
		collect(path+".steps", cmd.Steps)
		collect(path+".before", cmd.Before)
		collect(path+".replace", cmd.Replace)
		collect(path+".after", cmd.After)
	})

	sort.Strings(warnings)
	return warnings
}

// warnDuplicatePlanDeclarations warns when two plans declared in the same partition
// (see below) carry equal declaration fields: `environment`, `site`, `vars`,
// `endpoint_tags`, and per-entry `name`, `runner`, `order`, `depends_on`, `services`,
// `vars` (TASK-244 / PLAN-002's frozen D6 contract).
//
// This is deliberately narrower than "the plans are equivalent at runtime" — it does
// not resolve site overrides, environment profiles, or stack entries, so the message
// below states only that the compared declaration fields match. It also never
// recommends a canonical name or deletion: two authors may have equal declarations on
// purpose (e.g. one mid-migration to a renamed plan), and this check's job is to
// surface the coincidence, not to resolve it.
func (c *Config) warnDuplicatePlanDeclarations() []string {
	if len(c.Plans) < 2 {
		return nil
	}

	names := make([]string, 0, len(c.Plans))
	for name := range c.Plans {
		names = append(names, name)
	}
	sort.Strings(names)

	var warnings []string
	for i, nameA := range names {
		planA := c.Plans[nameA]
		if planA == nil {
			continue
		}
		for _, nameB := range names[i+1:] {
			planB := c.Plans[nameB]
			if planB == nil {
				continue
			}
			// A canonical import name and its `as:` alias are the SAME *PlanConfig
			// reachable under two map keys (subproject.go's import loop assigns
			// both `cfg.Plans[canonicalName]` and, when `as:` is given,
			// `cfg.Plans[alias]` to one `importedPlan`). That is one declaration
			// exposed twice, not two authors independently writing the same
			// plan — comparing declaration fields on it would always "duplicate"
			// and would warn on every aliased import.
			if planA == planB {
				continue
			}
			// Partition by SubprojectPath: "" for root-declared plans, the
			// subproject directory for imported ones (subproject.go's
			// cloneImportedPlan). Root and each subproject are independently
			// authored namespaces — subprojects are resolved once, flatly,
			// against the root (config.go's single Load call site), so this
			// never compares root against a child or one child against another;
			// a same-shaped plan across those boundaries is not one author's
			// accidental duplicate.
			if planA.SubprojectPath != planB.SubprojectPath {
				continue
			}
			if !plansHaveEqualDeclaration(planA, planB) {
				continue
			}
			warnings = append(warnings, fmt.Sprintf(
				"plans %q and %q declare equal environment, site, vars, endpoint_tags, and entries — review whether both are intentional",
				nameA, nameB,
			))
		}
	}
	return warnings
}

// plansHaveEqualDeclaration compares exactly the fields TASK-244's D6 contract
// freezes. Map fields (Vars) are order-insensitive by construction: maps.Equal
// compares key/value pairs, not iteration order, and treats a nil map as equal to
// an empty one because both have length zero. Slice fields (EndpointTags, and
// per-entry DependsOn/Services) are order-sensitive: slices.Equal compares
// position by position, so a reordered list is NOT a duplicate, and it likewise
// treats nil and empty as equal by length. Description is intentionally excluded —
// it is prose, not a declared executable field, and the card does not list it.
func plansHaveEqualDeclaration(a, b *PlanConfig) bool {
	if a.Environment != b.Environment || a.Site != b.Site {
		return false
	}
	if !maps.Equal(a.Vars, b.Vars) {
		return false
	}
	if !slices.Equal(a.EndpointTags, b.EndpointTags) {
		return false
	}
	if len(a.Entries) != len(b.Entries) {
		return false
	}
	for i := range a.Entries {
		if !planEntriesEqual(a.Entries[i], b.Entries[i]) {
			return false
		}
	}
	return true
}

// planEntriesEqual compares one PlanEntry pair on the fields D6 freezes: Name,
// Runner, Order, DependsOn, Services, Vars. Entries are compared positionally by
// plansHaveEqualDeclaration (list order in Entries is itself part of the
// declaration), so this only judges whether the entries at matching positions
// agree.
func planEntriesEqual(a, b PlanEntry) bool {
	return a.Name == b.Name &&
		a.Runner == b.Runner &&
		a.Order == b.Order &&
		slices.Equal(a.DependsOn, b.DependsOn) &&
		slices.Equal(a.Services, b.Services) &&
		maps.Equal(a.Vars, b.Vars)
}

// warnMultiplePlansWithoutDefault warns when two or more plans are declared but
// `default_plan` is not set, leaving bare lifecycle commands (e.g. `dva up`) with no
// bounded plan selection — plan_lifecycle.go's guard already refuses those at
// runtime ("multiple plans configured; specify one: ..."), so this surfaces the same
// condition earlier, at `validate` time.
//
// Checks c.DefaultPlanName == "" rather than c.DefaultPlan() == "" (or
// c.DefaultPlanSource() == "none"). Those two also resolve to "" / "none" when
// default_plan IS declared but names a plan that does not exist — and
// validate.go's Validate() already rejects exactly that as a hard error
// ("default_plan '%s' not found in plans"). Testing the resolved value here would
// re-warn on top of that hard error and would contradict this warning's own
// wording ("default_plan is not set"), which is only true when the key is absent,
// not when it is present but wrong.
//
// len(c.Plans) >= 2 excludes the single-plan implicit-default contract: DefaultPlan()
// already treats a lone plan as the default with no declaration required, so a
// single-plan config is not ambiguous and must not warn here.
func (c *Config) warnMultiplePlansWithoutDefault() []string {
	if len(c.Plans) < 2 || c.DefaultPlanName != "" {
		return nil
	}

	names := make([]string, 0, len(c.Plans))
	for name := range c.Plans {
		names = append(names, name)
	}
	sort.Strings(names)

	return []string{fmt.Sprintf(
		"%d plans are defined (%s) but default_plan is not set — bare lifecycle commands (e.g. 'dva up') require naming a plan explicitly; set default_plan to one of them",
		len(names), strings.Join(names, ", "),
	)}
}

// warnHealthCheckRedundancy warns when both start and start_hint are set on a health check.
func (c *Config) warnHealthCheckRedundancy() []string {
	var warnings []string

	// Top-level health checks
	for name, hc := range c.HealthChecks {
		if hc.Start != "" && hc.StartHint != "" {
			warnings = append(warnings,
				fmt.Sprintf("health_checks.%s: 'start_hint' is redundant when 'start' is set (auto-start takes priority)", name))
		}
	}

	// Stack-nested health checks
	for entryName, entry := range c.Stack {
		for name, hc := range entry.HealthChecks {
			if hc.Start != "" && hc.StartHint != "" {
				warnings = append(warnings,
					fmt.Sprintf("stack.%s.health_checks.%s: 'start_hint' is redundant when 'start' is set (auto-start takes priority)", entryName, name))
			}
		}
	}

	// Both sources are maps, so without this the same dva.yml prints these warnings in a
	// different order between runs — 4 distinct orderings over 15 runs of the real binary with
	// the sort removed, and that count is itself a sample, not a constant. TASK-125 sorted
	// three interaction checks and left this one. TASK-128.
	sort.Strings(warnings)
	return warnings
}

// warnUnreachableHealthChecks warns when a top-level health_checks entry declares start
// and/or start_hint but no modes.*.health_checks list references it.
//
// Top-level start is executed through exactly one path — Orchestrator.startModeProcesses —
// and that path returns early unless opts.Mode is set, the mode exists, and the mode's
// health_checks list names the entry (signalModeProcesses gates the down/stop half the
// same way). Without a mode reference, start is dead config and start_hint never surfaces
// through that path either. Nested stack.<entry>.health_checks is a different field on a
// different code path (un-gated readiness/wait); this pass does not touch it. TASK-179.
//
// The same pass also names modes.*.health_checks entries that point at a missing
// health_checks name: startModeProcesses silently continues on !ok, so a typo used to
// validate clean and do nothing at runtime.
//
// Severity: Semantic Warning
func (c *Config) warnUnreachableHealthChecks() []string {
	referenced := make(map[string]bool)
	for _, mode := range c.Modes {
		for _, name := range mode.HealthChecks {
			referenced[name] = true
		}
	}

	var warnings []string

	for name, hc := range c.HealthChecks {
		if hc.Start == "" && hc.StartHint == "" {
			continue
		}
		if referenced[name] {
			continue
		}
		var fields []string
		if hc.Start != "" {
			fields = append(fields, "start")
		}
		if hc.StartHint != "" {
			fields = append(fields, "start_hint")
		}
		warnings = append(warnings, fmt.Sprintf(
			"health_checks.%s: declares %s but no modes.*.health_checks entry references it, so auto-start/hint will never run; add it to a mode's health_checks list, move the check under stack.<entry>.health_checks, or remove the field",
			name, strings.Join(fields, " and ")))
	}

	// Dangling mode references: sort mode names so map iteration cannot reorder warnings.
	modeNames := make([]string, 0, len(c.Modes))
	for modeName := range c.Modes {
		modeNames = append(modeNames, modeName)
	}
	sort.Strings(modeNames)
	for _, modeName := range modeNames {
		mode := c.Modes[modeName]
		for _, hcName := range mode.HealthChecks {
			if _, ok := c.HealthChecks[hcName]; ok {
				continue
			}
			warnings = append(warnings, fmt.Sprintf(
				"modes.%s.health_checks: references %q which is not defined under top-level health_checks; startModeProcesses skips missing names silently",
				modeName, hcName))
		}
	}

	sort.Strings(warnings)
	return warnings
}

// eachInteractionNode visits every command in the interaction tree — top-level entries and
// nested subcommands alike — passing each node with its dotted config path so a warning can
// name the exact YAML location.
//
// `subcommands` is recursive by construction (map[string]*InteractionCommand), the runner
// executes it to unbounded depth, and examples/full-stack.yml already ships three levels
// (`dva rails db migrate`). A check that walks only the first level therefore reports a
// mistake at the top and stays silent on the identical one below it, which is the reasoning
// warnInertProvisionSteps already records for its own walker.
//
// The visitor also receives the execution context inherited from the node's ancestors. That
// is not a convenience: runner.mergeInteraction copies every execution field parent → child
// and lets the child override only what it sets, so the node the runtime executes is not the
// node parsed from YAML. At depth 1 the two coincide — there is no ancestor — which is why
// checks written against raw nodes were correct until they were made to recurse, and wrong
// immediately afterwards. Any check that asks "does this node have X" must ask it of the
// inherited view. TASK-128.
func eachInteractionNode(interaction map[string]*InteractionCommand, visit func(path string, cmd *InteractionCommand, inherited inheritedExec)) {
	var walk func(path string, cmd *InteractionCommand, inherited inheritedExec)
	walk = func(path string, cmd *InteractionCommand, inherited inheritedExec) {
		if cmd == nil {
			return
		}
		visit(path, cmd, inherited)

		descend := inheritedExec{
			callable: inherited.callable || cmd.hasExecutionTarget(),
			runner:   firstNonEmptyStr(cmd.Runner, inherited.runner),
			pod:      firstNonEmptyStr(cmd.Pod, inherited.pod),
		}
		for subName, sub := range cmd.Subcommands {
			walk(path+".subcommands."+subName, sub, descend)
		}
	}

	for name, cmd := range interaction {
		walk("interaction."+name, cmd, inheritedExec{})
	}
}

// inheritedExec is what a node receives from its ancestors under
// runner.mergeInteraction's rules. Only the fields the warnings below actually consult are
// carried; adding a check that depends on another inherited field means adding it here too.
type inheritedExec struct {
	// callable records whether any ancestor supplies something to execute, which the child
	// inherits and can therefore be invoked with.
	callable bool
	// runner and pod are the nearest ancestor's effective values, i.e. what the child ends
	// up running under when it does not set its own.
	runner string
	pod    string
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// hasExecutionTarget reports whether this node itself supplies something to execute, ignoring
// anything it would inherit.
//
// `default_args` counts because it executes on its own, which is not obvious from the name.
// exec.buildCommandLine appends the args to the command and, in shell mode — the default —
// hands the joined string to `sh -c`, so an empty command with args leaves the args as the
// whole shell line. Measured: a node with only `default_args: "echo reached"` and nothing else
// prints `reached` and exits 0. Omitting it here made the predicate disagree with the runtime
// in the one direction that produces a false warning — telling an author a node cannot run
// while it runs. TASK-165.
func (c *InteractionCommand) hasExecutionTarget() bool {
	return c.Command != "" || len(c.CommandLines) > 0 || c.HasScript() ||
		c.ScriptFile != "" || c.HasSteps() || c.HasHooks() || c.Compose != nil ||
		c.Runner != "" || c.Service != "" || c.Pod != "" || c.DefaultArgs != ""
}

// warnDuplicateParentSubcommand warns when an interaction command has the same command value
// as one of its subcommands, at any depth.
//
// Results are sorted because both this tree and each node's subcommands are maps: without it
// the same dva.yml prints its warnings in a different order on consecutive runs, which is the
// defect TASK-107 closed for command suggestions.
func (c *Config) warnDuplicateParentSubcommand() []string {
	var warnings []string

	eachInteractionNode(c.Interaction, func(path string, cmd *InteractionCommand, _ inheritedExec) {
		if cmd.Command == "" {
			return
		}
		for subName, sub := range cmd.Subcommands {
			if sub.Command == cmd.Command {
				warnings = append(warnings,
					fmt.Sprintf("%s.subcommands.%s: command %q is identical to parent; subcommand is redundant",
						path, subName, cmd.Command))
			}
		}
	})

	sort.Strings(warnings)
	return warnings
}

// warnDuplicateStackOrder warns when multiple stack entries share the same order value.
//
// Entries that modes keep apart are exempt: order only decides who starts first within
// one invocation, so when no invocation ever holds two members of a group, there is no
// sequence between them to control. Without this the warning tells users to order
// entries that never meet.
//
// Entries a plan names are exempt too, and that exemption is the point of TASK-084. It used to be
// `len(c.Plans) > 0` — any plan anywhere silenced everything — on the premise that "plan order owns
// sequencing when plans exist". Too coarse in one direction: it hid entries no plan mentions. Too
// coarse in the other: dropping it entirely warned about the shape `docs/40-declarative-stack-and-
// plans.md` prescribes, where order belongs to the plan layer, so three shipped examples started
// being told they had not chosen a sequence when their plan declares infra→api→worker→web.
//
// What is checkable is whether a plan names the entry, not whether some plan exists. An entry a
// plan names has a declared position; the gap is only that plan-less `dva up` does not read plans
// (its help says so), and that is a property of the command rather than of the config.
func (c *Config) warnDuplicateStackOrder() []string {
	if len(c.Stack) < 2 {
		return nil
	}
	planned := c.entriesNamedByPlans()

	orderMap := make(map[int][]string)
	for name, entry := range c.Stack {
		orderMap[entry.Order] = append(orderMap[entry.Order], name)
	}

	var warnings []string
	// Sort by order value for deterministic output
	var orders []int
	for order := range orderMap {
		orders = append(orders, order)
	}
	sort.Ints(orders)

	for _, order := range orders {
		names := orderMap[order]
		if len(names) < 2 {
			continue
		}
		group := make(map[string]bool, len(names))
		for _, name := range names {
			group[name] = true
		}
		if c.modesIsolateEntries(group) {
			continue
		}
		// Report only the entries no plan names. Dropping the covered ones rather than the whole
		// group is what keeps the warning useful on a plan that names two entries of five: the
		// other three still have no declared position anywhere, and they are the ones to say so
		// about. Below two, there is no pair left to sequence.
		unplanned := make([]string, 0, len(names))
		for _, name := range names {
			if !planned[name] {
				unplanned = append(unplanned, name)
			}
		}
		if len(unplanned) < 2 {
			continue
		}
		sort.Strings(unplanned)
		// Not "undefined": since TASK-084 half 1 the sequence is (Order, Name), so equal orders
		// resolve alphabetically and repeat run to run. What is left to report is not a hazard but
		// a position never declared — for these entries, in the plan layer or anywhere else.
		var msg string
		if order == 0 {
			msg = fmt.Sprintf("stack: entries %s are at the default order and no plan names them, so `dva up` starts them in name order rather than one you chose",
				strings.Join(unplanned, ", "))
		} else {
			msg = fmt.Sprintf("stack: entries %s share order value %d and no plan names them, so `dva up` starts them in name order rather than one you chose",
				strings.Join(unplanned, ", "), order)
		}
		// Named only when plans exist, because that is the reader who would otherwise assume the
		// plan already settled this. Saying it unconditionally would advertise plans to configs
		// that have none.
		if len(c.Plans) > 0 {
			msg += "; plan entry order governs `dva up <plan>` only"
		}
		warnings = append(warnings, msg)
	}

	return warnings
}

// entriesNamedByPlans is the set of stack entries some plan gives a position to. Membership is
// what warnDuplicateStackOrder treats as "the author declared where this runs" — a plan walks its
// entries in declaration order, so being listed is a position even with no explicit `order:`.
func (c *Config) entriesNamedByPlans() map[string]bool {
	planned := make(map[string]bool)
	for _, plan := range c.Plans {
		if plan == nil {
			continue
		}
		for _, e := range plan.Entries {
			planned[e.Name] = true
		}
	}
	return planned
}

// warnMultiStackComposeSplit warns when compose entries that can run together were
// split across stack entries. Each entry is its own `docker compose` invocation, so
// an overlay entry cannot patch a base entry's service definitions.
//
// Named plans are the authoritative lifecycle surface. Only two non-empty,
// fully literal project_name values that differ are provably independent Compose
// projects. Empty or interpolation-bearing names remain unknown: plan, site and
// entry variables can change their runtime value, which validation cannot prove.
func (c *Config) warnMultiStackComposeSplit() []string {
	byProjectName := make(map[string]map[string]bool)
	unknown := make(map[string]bool)
	for name, entry := range c.Stack {
		// ComposeConfig(), not entry.Compose: the supported shape stores compose under
		// runners, so reading the legacy field alone made this warning unreachable for
		// every config that follows the current schema.
		if entry.ComposeConfig() != nil {
			projectName, literal := literalComposeProjectName(entry.ComposeConfig().ProjectName)
			if !literal {
				unknown[name] = true
				continue
			}
			if byProjectName[projectName] == nil {
				byProjectName[projectName] = make(map[string]bool)
			}
			byProjectName[projectName][name] = true
		}
	}

	projectNames := make([]string, 0, len(byProjectName))
	for projectName := range byProjectName {
		projectNames = append(projectNames, projectName)
	}
	sort.Strings(projectNames)

	var warnings []string
	for _, projectName := range projectNames {
		entries := make(map[string]bool, len(byProjectName[projectName])+len(unknown))
		for name := range byProjectName[projectName] {
			entries[name] = true
		}
		for name := range unknown {
			entries[name] = true
		}
		if len(entries) < 2 || c.composeEntriesAreIsolated(entries) {
			continue
		}
		warnings = append(warnings, c.composeSplitWarning(entries))
	}
	if len(projectNames) == 0 && len(unknown) > 1 && !c.composeEntriesAreIsolated(unknown) {
		warnings = append(warnings, c.composeSplitWarning(unknown))
	}
	return warnings
}

func literalComposeProjectName(projectName string) (string, bool) {
	projectName = strings.TrimSpace(projectName)
	return projectName, projectName != "" && !strings.Contains(projectName, "$")
}

func (c *Config) composeEntriesAreIsolated(entries map[string]bool) bool {
	if len(c.Plans) > 0 {
		return c.plansIsolateEntries(entries)
	}
	return c.modesIsolateEntries(entries)
}

func (c *Config) composeSplitWarning(entries map[string]bool) string {
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)

	remedy := "give each its own named plan (plans.<name>.entries: [{name: entry}])"
	if len(c.Plans) > 0 && c.plansWouldIsolateEntries(entries) && c.DefaultPlan() == "" {
		remedy = "set default_plan to a minimal plan that selects one entry, so unnamed lifecycle commands have a bounded selection"
	} else if len(c.Plans) > 0 && c.anyPlanSelectsEntries(entries) {
		remedy = "merge them into one entry whose files: lists the overlays"
	}

	return fmt.Sprintf("stack: compose entries [%s] can run in the same invocation set — each is a separate 'docker compose' call, so an overlay entry cannot patch another entry's services; %s",
		strings.Join(names, ", "), remedy)
}

// plansIsolateEntries reports whether named plans make the compose entries
// mutually exclusive. A default plan is required before an unnamed lifecycle
// command has a bounded selection; explicit entry-oriented lifecycle commands
// are deliberate escape hatches and are not evidence that plans co-occur.
func (c *Config) plansIsolateEntries(entries map[string]bool) bool {
	return c.DefaultPlan() != "" && c.plansWouldIsolateEntries(entries)
}

// plansWouldIsolateEntries reports whether each named plan selects at most one
// entry and every entry is selected by a plan. It intentionally does not inspect
// explicit entry-oriented lifecycle commands, which bypass plan selection by
// design and should not make otherwise isolated plans warn.
func (c *Config) plansWouldIsolateEntries(entries map[string]bool) bool {
	if len(c.Plans) == 0 {
		return false
	}
	claimed := make(map[string]bool, len(entries))
	for _, plan := range c.Plans {
		if plan == nil {
			return false
		}
		hits := 0
		for _, entry := range plan.Entries {
			if entries[entry.Name] {
				hits++
				claimed[entry.Name] = true
			}
		}
		if hits > 1 {
			return false
		}
	}

	return len(claimed) == len(entries)
}

func (c *Config) anyPlanSelectsEntries(entries map[string]bool) bool {
	for _, plan := range c.Plans {
		if plan == nil {
			continue
		}
		hits := 0
		for _, entry := range plan.Entries {
			if entries[entry.Name] {
				hits++
			}
		}
		if hits > 1 {
			return true
		}
	}
	return false
}

// modesIsolateEntries reports whether every entry in the set is claimed by a mode and
// no single mode pulls in two of them — the arrangement where at most one member of the
// set is ever live, so no two of them can interact.
//
// Callers use this to answer "can these entries co-occur?", which is the question behind
// several warnings: two compose entries only collide if both are up, and two entries only
// race for startup order if both are in the same invocation.
//
// A mode with no stack: filter selects every entry (see Orchestrator.filterEntries),
// so it disqualifies the whole arrangement. Configs that set no default_mode leave the
// same unfiltered path reachable from a bare `dva up`; warnMissingDefaultMode already
// says so, and repeating it here would just be a second voice on one problem.
//
// Looking only at Modes is sound even though environments.<name>.stack is a second,
// independent entry filter (Orchestrator.filterEntries applies env and mode as separate
// steps). The two narrow the set by intersection, and every command path resolves the mode
// through applyDefaultMode first, so `dva up --env X` still gets the default mode's filter
// on top of the environment's. Verified: with default_mode set, an environment listing two
// same-order entries plans only the one its mode selects. Without default_mode the mode
// filter drops out and both do run — which is the unfiltered path the paragraph above
// defers to warnMissingDefaultMode.
func (c *Config) modesIsolateEntries(entries map[string]bool) bool {
	if len(c.Modes) == 0 {
		return false
	}
	claimed := make(map[string]bool, len(entries))
	for _, mode := range c.Modes {
		selected := mode.StackEntries()
		if len(selected) == 0 {
			return false
		}
		hits := 0
		for _, name := range selected {
			if entries[name] {
				hits++
				claimed[name] = true
			}
		}
		if hits > 1 {
			return false
		}
	}
	return len(claimed) == len(entries)
}

// warnMissingDefaultMode warns when modes are defined but no default_mode is set,
// meaning dva up without -M will start all services from all compose files.
// Note: invalid default_mode references are caught as hard errors in Validate().
func (c *Config) warnMissingDefaultMode() []string {
	if len(c.Modes) == 0 || c.DefaultMode != "" {
		return nil
	}
	return []string{
		"modes are defined but default_mode is not set — dva up without -M will start all services from all compose files; set default_mode to a minimal infrastructure mode (e.g., 'infra')",
	}
}

// heavyInfraServiceNames lists well-known service names that should NOT
// appear in the default (minimal) mode. These are non-core infrastructure
// services that consume significant resources.
var heavyInfraServiceNames = map[string]bool{
	"kafka":          true,
	"zookeeper":      true,
	"prometheus":     true,
	"alertmanager":   true,
	"grafana":        true,
	"jaeger":         true,
	"minio":          true,
	"elasticsearch":  true,
	"kibana":         true,
	"logstash":       true,
	"loki":           true,
	"tempo":          true,
	"otel-collector": true,
	"zipkin":         true,
	"rabbitmq":       true,
	"nats":           true,
}

// heavyInfraTags lists service tags that indicate non-core infrastructure.
var heavyInfraTags = map[string]bool{
	"monitoring": true,
	"storage":    true,
	"kafka":      true,
	"queue":      true,
	"search":     true,
}

// warnDefaultModeHeavyInfra warns when the default mode includes heavy
// infrastructure services (monitoring, event streaming, object storage, etc.)
// that should be in separate modes to keep `dva up` fast and lightweight.
func (c *Config) warnDefaultModeHeavyInfra() []string {
	if c.DefaultMode == "" || len(c.Modes) == 0 {
		return nil
	}
	mode, ok := c.Modes[c.DefaultMode]
	if !ok || mode.ComposeServices == nil {
		return nil
	}

	serviceTags := c.ComposeServices()

	var heavy []string
	for _, svc := range *mode.ComposeServices {
		if isHeavyInfra(svc, serviceTags) {
			heavy = append(heavy, svc)
		}
	}

	if len(heavy) == 0 {
		return nil
	}

	sort.Strings(heavy)
	return []string{
		fmt.Sprintf("default_mode %q includes non-core infrastructure services %v; "+
			"consider moving them to a separate mode (e.g., full-stack, infra-full) — "+
			"default mode should only include core data services (DB, cache)",
			c.DefaultMode, heavy),
	}
}

// isHeavyInfra checks whether a service is heavy infrastructure by its tags
// (if declared) or by well-known name heuristics as fallback.
func isHeavyInfra(svcName string, serviceTags map[string]ServiceTagConfig) bool {
	if cfg, ok := serviceTags[svcName]; ok && len(cfg.Tags) > 0 {
		for _, tag := range cfg.Tags {
			if heavyInfraTags[tag] {
				return true
			}
		}
		// Service has tags but none are heavy — trust the tags
		return false
	}
	// No tag info — fall back to name heuristic
	return heavyInfraServiceNames[svcName]
}

// validateCanonicalOrder reads the YAML file and checks that top-level keys
// follow the canonical section order. Returns warnings for out-of-order keys.
func validateCanonicalOrder(filePath string) []string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil
	}

	// doc is a DocumentNode wrapping a MappingNode
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil
	}

	// Extract top-level keys in file order, filtering to canonical-only keys
	var fileKeys []string
	for i := 0; i < len(root.Content)-1; i += 2 {
		key := root.Content[i].Value
		if _, ok := canonicalOrderIndex[key]; ok {
			fileKeys = append(fileKeys, key)
		}
	}

	if len(fileKeys) < 2 {
		return nil
	}

	// Check if file keys are in canonical order
	inOrder := true
	for i := 1; i < len(fileKeys); i++ {
		if canonicalOrderIndex[fileKeys[i]] < canonicalOrderIndex[fileKeys[i-1]] {
			inOrder = false
			break
		}
	}

	if inOrder {
		return nil
	}

	// Build the expected order for present keys
	var expected []string
	for _, s := range canonicalSectionOrder {
		if slices.Contains(fileKeys, s) {
			expected = append(expected, s)
		}
	}

	return []string{
		fmt.Sprintf("section order: found [%s] but canonical order is [%s]; consider reordering",
			strings.Join(fileKeys, " → "), strings.Join(expected, " → ")),
	}
}

// warnChildOverridesParentCritical warns when a child overrides its parent's runner or pod,
// potentially altering the backend unexpectedly.
//
// Severity: Semantic Warning
func (c *Config) warnChildOverridesParentCritical() []string {
	var warnings []string

	eachInteractionNode(c.Interaction, func(path string, cmd *InteractionCommand, inherited inheritedExec) {
		// The effective values, not the raw ones. A middle node that sets no runner still
		// hands its parent's down, so comparing against cmd.Runner made the warning fire
		// only when an author happened to restate a value they would have inherited anyway
		// — silent on the identical config written without the redundant line. TASK-128.
		parentRunner := firstNonEmptyStr(cmd.Runner, inherited.runner)
		parentPod := firstNonEmptyStr(cmd.Pod, inherited.pod)

		for subName, sub := range cmd.Subcommands {
			if parentRunner != "" && sub.Runner != "" && parentRunner != sub.Runner {
				warnings = append(warnings,
					fmt.Sprintf("%s.subcommands.%s: overrides parent runner (%s → %s); this may change execution backend unexpectedly",
						path, subName, parentRunner, sub.Runner))
			}
			if parentPod != "" && sub.Pod != "" && parentPod != sub.Pod {
				warnings = append(warnings,
					fmt.Sprintf("%s.subcommands.%s: overrides parent pod (%s → %s); this may change execution backend unexpectedly",
						path, subName, parentPod, sub.Pod))
			}
		}
	})

	sort.Strings(warnings)
	return warnings
}

const MaxSubcommandDepth = 5

// warnDeepSubcommandNesting warns when nested subcommands exceed a specific depth,
// signifying overly complex DSL structure.
//
// Severity: Semantic Warning
func (c *Config) warnDeepSubcommandNesting() []string {
	var warnings []string

	for name, cmd := range c.Interaction {
		depth := calculateSubcommandDepth(cmd, 0)
		if depth > MaxSubcommandDepth {
			warnings = append(warnings,
				fmt.Sprintf("interaction.%s: nested %d levels deep (max %d); consider flattening the command structure",
					name, depth, MaxSubcommandDepth))
		}
	}

	// c.Interaction is a map, so the order here is whatever Go's randomized range hands back.
	// TestFlatMapWarningsAreOrderStable diverges on the first repeat with this removed. TASK-128.
	sort.Strings(warnings)
	return warnings
}

// warnLiteralKeyShadowsSubproject warns when a declared colon key's prefix also names a
// subproject, so the literal key wins and the subproject's command of the same spelling
// becomes unreachable.
//
// This shape could not occur before TASK-167: run.go split every colon key, so the
// subproject always won and a literal `engine:test` in the parent was dead config. Routing
// the literal key first fixes the far larger silent-failure class that task measured, and
// creates exactly this one ambiguity in exchange — so the warning ships with the routing
// change rather than after it. Closing one silent shadowing by opening another would be
// TASK-137's silent-relocation shape, which is the thing that task exists to stop.
//
// A warning, not an error: both readings are legitimate config, the author may well mean the
// local one, and it stays runnable either way. The message names the form that lost, because
// knowing `engine:test` now runs the parent's command is only half of what a reader needs —
// the other half is how to reach the child's.
//
// The escape hatch is spelled `dva run --project`, with the verb, and it was executed against
// the binary before being written here — the bar ConflictAdvice sets. The shorter `dva
// --project engine test` reads better and does not work: --project is registered on runCmd, so
// it only parses after an explicit `run`, and the bare form's rewrite in cli.Execute does not
// look past a leading flag. Measured, it exits 1 with `unknown command "test" for "dva"`.
//
// Severity: Semantic Warning
func (c *Config) warnLiteralKeyShadowsSubproject() []string {
	var warnings []string

	for name := range c.Interaction {
		idx := strings.Index(name, ":")
		if idx <= 0 {
			continue
		}
		prefix, sub := name[:idx], name[idx+1:]
		if _, ok := c.Subprojects[prefix]; !ok {
			continue
		}
		// A reserved prefix is still unroutable, so nothing is shadowed — that config has a
		// hard error from ValidateReservedCommands already and does not need a second opinion.
		if IsReservedCommand(prefix) {
			continue
		}
		warnings = append(warnings, fmt.Sprintf(
			"interaction.%s: `dva %s` runs this key, not subproject `%s`'s `%s` — "+
				"the literal key takes precedence; use `dva run --project %s %s` to reach the subproject",
			name, name, prefix, sub, prefix, sub))
	}

	// c.Interaction is a map; see warnDeepSubcommandNesting. TASK-128.
	sort.Strings(warnings)
	return warnings
}

func calculateSubcommandDepth(cmd *InteractionCommand, current int) int {
	if len(cmd.Subcommands) == 0 {
		return current
	}

	maxDepth := current + 1
	for _, sub := range cmd.Subcommands {
		depth := calculateSubcommandDepth(sub, current+1)
		if depth > maxDepth {
			maxDepth = depth
		}
	}

	return maxDepth
}

// warnUnreachableCommands warns when an interaction node supplies no execution context
// (e.g. no command, no service, no compose) and inherits none, in either of the two shapes
// that produces: a parent that cannot be called directly, and a leaf that cannot be called
// at all.
//
// The leaf is the worse of the two and used to be the one nobody reported. Until TASK-165
// this check returned early on `len(cmd.Subcommands) == 0`, so it fired only where there was
// still something to route to. On the fixture below, `grp` — which at least reaches its
// children — got the warning, and `grp.leaf`, the node that can never do anything, got
// nothing from the validator and exit 0 from the runtime:
//
//	interaction:
//	  grp:
//	    subcommands:
//	      leaf: {description: does nothing at all}
//
// Severity: Semantic Warning
func (c *Config) warnUnreachableCommands() []string {
	var warnings []string

	eachInteractionNode(c.Interaction, func(path string, cmd *InteractionCommand, inherited inheritedExec) {
		// Inheritance is the whole point of asking both: examples/full-stack.yml's
		// `rails db` sets nothing itself, yet `dva run rails db` executes
		// `bundle exec rails` inherited from `rails`. Testing the raw node alone reported
		// that shipped, working config as unreachable. TASK-128.
		if cmd.hasExecutionTarget() || inherited.callable {
			return
		}
		if len(cmd.Subcommands) > 0 {
			warnings = append(warnings,
				fmt.Sprintf("%s: has subcommands but is not directly callable; add an execution target or remove subcommands",
					path))
			return
		}
		warnings = append(warnings,
			fmt.Sprintf("%s: has no execution target and no subcommands, so running it does nothing; add a command, script, steps or service — or remove the entry",
				path))
	})

	sort.Strings(warnings)
	return warnings
}

// warnUnresolvedEnvVars checks if any variables in the environment block remain unresolved
// after config and OS interpolation, indicating a possible typo or missing variable.
//
// Severity: Semantic Warning
func (c *Config) warnUnresolvedEnvVars(env *Environment, envFilesDeclared bool) []string {
	// An env_file declaration can define any of the names still unresolved here,
	// and validation does not open it. Warning anyway would report a defect that
	// does not exist for every project that keeps its variables in .env.
	if envFilesDeclared {
		return nil
	}

	var warnings []string

	for k, v := range c.Environment {
		finalVal := env.Interpolate(v)
		// Extract all remaining `${VAR}` or `$VAR` patterns
		matches := varRegex.FindAllString(finalVal, -1)
		if len(matches) > 0 {
			warnings = append(warnings,
				fmt.Sprintf("environment.%s: contains unresolved variable reference %v; verify variable name",
					k, matches))
		}
	}

	sort.Strings(warnings)
	return warnings
}

// warnSuspiciousEnvPatterns warns when users try to use shell-specific interpolation semantics
// like ${VAR:-default} or $#, which are not natively supported by the config parser.
//
// Severity: Semantic Warning
func (c *Config) warnSuspiciousEnvPatterns() []string {
	var warnings []string

	for k, v := range c.Environment {
		if strings.Contains(v, "$#") || (strings.Contains(v, "${") && strings.Contains(v, ":")) {
			warnings = append(warnings,
				fmt.Sprintf("environment.%s: contains shell-specific syntax that is not supported; use plain $VAR or ${VAR}", k))
		}
	}

	sort.Strings(warnings)
	return warnings
}
