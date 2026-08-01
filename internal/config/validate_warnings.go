package config

import (
	"fmt"
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
	"checks", "applications", "default_mode", "suggestion_ignore", "modes",
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
	warnings = append(warnings, c.warnLegacyApplications()...)
	warnings = append(warnings, c.warnDuplicateComposeApplicationOwnership()...)
	warnings = append(warnings, c.warnLegacyStackOrder()...)
	warnings = append(warnings, c.warnLegacyEnvironmentFields()...)
	warnings = append(warnings, c.warnNoPlansHint()...)
	warnings = append(warnings, c.warnHealthCheckRedundancy()...)
	warnings = append(warnings, c.warnDuplicateParentSubcommand()...)
	warnings = append(warnings, c.warnDuplicateStackOrder()...)
	warnings = append(warnings, c.warnMultiStackComposeSplit()...)
	warnings = append(warnings, c.warnMissingDefaultMode()...)
	warnings = append(warnings, c.warnDefaultModeHeavyInfra()...)
	warnings = append(warnings, c.warnChildOverridesParentCritical()...)
	warnings = append(warnings, c.warnDeepSubcommandNesting()...)
	warnings = append(warnings, c.warnUnreachableCommands()...)
	warnings = append(warnings, c.warnInertProvisionSteps()...)

	// Build a contextual environment for accurate interpolation checks
	env := NewEnvironment(c.Environment, c.FileDir(), c.FileDir())
	_ = LoadEnvFile(c.EnvFile, c.FileDir(), env)

	warnings = append(warnings, c.warnUnresolvedEnvVars(env)...)
	warnings = append(warnings, c.warnSuspiciousEnvPatterns()...)

	if c.filePath != "" {
		warnings = append(warnings, validateCanonicalOrder(c.filePath)...)
	}
	return warnings
}

// warnDuplicateComposeApplicationOwnership warns when a legacy application
// docker path points at a service already owned by a compose stack entry. In
// that shape, top-level lifecycle can start the same service once through the
// stack orchestrator and again through the legacy application manager.
func (c *Config) warnDuplicateComposeApplicationOwnership() []string {
	composeServices := make(map[string][]string)
	for entryName, entry := range c.Stack {
		if entry == nil {
			continue
		}

		var composeConfigs []*ComposePluginConfig
		if entry.Compose != nil {
			composeConfigs = append(composeConfigs, entry.Compose)
		}
		if runner, ok := entry.Runners["compose"].(*ComposePluginConfig); ok && runner != nil {
			composeConfigs = append(composeConfigs, runner)
		}

		for _, composeConfig := range composeConfigs {
			for service := range composeConfig.Services {
				composeServices[service] = append(composeServices[service], entryName)
			}
		}
	}

	var warnings []string
	for appName, app := range c.Applications {
		if app == nil {
			continue
		}

		refs := []struct {
			path    string
			service string
		}{
			{path: "run", service: app.Run.Docker.Service},
			{path: "dev", service: app.Dev.Docker.Service},
			{path: "build", service: app.Build.Docker.Service},
		}
		for _, ref := range refs {
			entries := composeServices[ref.service]
			if ref.service == "" || len(entries) == 0 {
				continue
			}
			sort.Strings(entries)
			warnings = append(warnings, fmt.Sprintf(
				"applications.%s.%s.docker.service %q is also owned by compose stack entry/entries [%s] — choose one lifecycle owner; prefer stack + plans for Compose services",
				appName, ref.path, ref.service, strings.Join(entries, ", ")))
		}
	}

	sort.Strings(warnings)
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

// warnLegacyApplications warns when legacy `applications` are present and suggests migration.
func (c *Config) warnLegacyApplications() []string {
	if len(c.Applications) == 0 {
		return nil
	}

	return []string{
		fmt.Sprintf("⚠ 'applications' section detected — consider migrating to multi-runner 'stack' entries\n  Migration guide: %s\n  Hint: each application can become a stack entry with native/docker/helm runners", migrationGuideURL),
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
		fmt.Sprintf("⚠ 'stack.*.order' detected — execution order should move to 'plans.*.entries[].order'\n  Migration guide: %s\n  Affected entries: %s\n  Hint: stack is now a declaration store; execution order belongs in plans", migrationGuideURL, strings.Join(affected, ", ")),
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
	var walk func(path string, cmd *InteractionCommand)
	walk = func(path string, cmd *InteractionCommand) {
		if cmd == nil {
			return
		}
		collect(path+".steps", cmd.Steps)
		collect(path+".before", cmd.Before)
		collect(path+".replace", cmd.Replace)
		collect(path+".after", cmd.After)
		for subName, sub := range cmd.Subcommands {
			walk(path+"."+subName, sub)
		}
	}

	for name, cmd := range c.Interaction {
		walk("interaction."+name, cmd)
	}

	for profile, items := range c.Provision.Profiles {
		collect(fmt.Sprintf("provision.%s", profile), items)
	}

	sort.Strings(warnings)
	return warnings
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
func eachInteractionNode(interaction map[string]*InteractionCommand, visit func(path string, cmd *InteractionCommand)) {
	var walk func(path string, cmd *InteractionCommand)
	walk = func(path string, cmd *InteractionCommand) {
		if cmd == nil {
			return
		}
		visit(path, cmd)
		for subName, sub := range cmd.Subcommands {
			walk(path+".subcommands."+subName, sub)
		}
	}

	for name, cmd := range interaction {
		walk("interaction."+name, cmd)
	}
}

// warnDuplicateParentSubcommand warns when an interaction command has the same command value
// as one of its subcommands, at any depth.
//
// Results are sorted because both this tree and each node's subcommands are maps: without it
// the same dva.yml prints its warnings in a different order on consecutive runs, which is the
// defect TASK-107 closed for command suggestions.
func (c *Config) warnDuplicateParentSubcommand() []string {
	var warnings []string

	eachInteractionNode(c.Interaction, func(path string, cmd *InteractionCommand) {
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
// plan names has a declared position; the gap is only that `dva stack up` does not read plans
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
			msg = fmt.Sprintf("stack: entries %s are at the default order and no plan names them, so `dva stack up` starts them in name order rather than one you chose",
				strings.Join(unplanned, ", "))
		} else {
			msg = fmt.Sprintf("stack: entries %s share order value %d and no plan names them, so `dva stack up` starts them in name order rather than one you chose",
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
// It stays quiet when modes divide the entries up, because that is the shape DVA
// itself prescribes: modes.<name>.compose was removed, and one entry per mode
// selected by modes.<name>.stack is the replacement. Warning there told users to
// undo the migration DVA required of them.
func (c *Config) warnMultiStackComposeSplit() []string {
	composeEntries := map[string]bool{}
	for name, entry := range c.Stack {
		// ComposeConfig(), not entry.Compose: the supported shape stores compose under
		// runners, so reading the legacy field alone made this warning unreachable for
		// every config that follows the current schema.
		if entry.ComposeConfig() != nil {
			composeEntries[name] = true
		}
	}
	if len(composeEntries) < 2 || c.modesIsolateEntries(composeEntries) {
		return nil
	}

	names := make([]string, 0, len(composeEntries))
	for name := range composeEntries {
		names = append(names, name)
	}
	sort.Strings(names)
	return []string{
		fmt.Sprintf("stack: compose entries [%s] can run in the same invocation set — each is a separate 'docker compose' call, so an overlay entry cannot patch another entry's services; give each its own mode (modes.<name>.stack: [entry]), or merge them into one entry whose files: lists the overlays",
			strings.Join(names, ", ")),
	}
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

	eachInteractionNode(c.Interaction, func(path string, cmd *InteractionCommand) {
		for subName, sub := range cmd.Subcommands {
			if cmd.Runner != "" && sub.Runner != "" && cmd.Runner != sub.Runner {
				warnings = append(warnings,
					fmt.Sprintf("%s.subcommands.%s: overrides parent runner (%s → %s); this may change execution backend unexpectedly",
						path, subName, cmd.Runner, sub.Runner))
			}
			if cmd.Pod != "" && sub.Pod != "" && cmd.Pod != sub.Pod {
				warnings = append(warnings,
					fmt.Sprintf("%s.subcommands.%s: overrides parent pod (%s → %s); this may change execution backend unexpectedly",
						path, subName, cmd.Pod, sub.Pod))
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

// warnUnreachableCommands warns when a parent interaction has subcommands but lacks execution context
// (e.g. no command, no service, no compose) itself, making it unreachable directly.
//
// Severity: Semantic Warning
func (c *Config) warnUnreachableCommands() []string {
	var warnings []string

	eachInteractionNode(c.Interaction, func(path string, cmd *InteractionCommand) {
		if len(cmd.Subcommands) == 0 {
			return
		}
		// A parent is essentially a "group" node if it has no execution directives.
		// Calling it directly typically requires at least one execution target.
		isCallable := cmd.Command != "" || len(cmd.CommandLines) > 0 || cmd.HasScript() ||
			cmd.ScriptFile != "" || cmd.HasSteps() || cmd.HasHooks() || cmd.Compose != nil ||
			cmd.Runner != "" || cmd.Service != "" || cmd.Pod != ""
		if !isCallable {
			warnings = append(warnings,
				fmt.Sprintf("%s: has subcommands but is not directly callable; add an execution target or remove subcommands",
					path))
		}
	})

	sort.Strings(warnings)
	return warnings
}

// warnUnresolvedEnvVars checks if any variables in the environment block remain unresolved
// after config and OS interpolation, indicating a possible typo or missing variable.
//
// Severity: Semantic Warning
func (c *Config) warnUnresolvedEnvVars(env *Environment) []string {
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
