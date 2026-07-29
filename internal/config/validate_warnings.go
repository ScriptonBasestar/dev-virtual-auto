package config

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const migrationGuideURL = "https://github.com/ScriptonBasestar/dva/blob/main/docs/40-declarative-stack-and-plans.md#11-migration"

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

// warnDuplicateParentSubcommand warns when a parent interaction command has
// the same command value as one of its subcommands.
// TODO: consider checking nested subcommands recursively (schema allows recursive subcommands).
func (c *Config) warnDuplicateParentSubcommand() []string {
	var warnings []string

	for name, cmd := range c.Interaction {
		if cmd.Command == "" || len(cmd.Subcommands) == 0 {
			continue
		}
		for subName, sub := range cmd.Subcommands {
			if sub.Command == cmd.Command {
				warnings = append(warnings,
					fmt.Sprintf("interaction.%s.subcommands.%s: command %q is identical to parent; subcommand is redundant",
						name, subName, cmd.Command))
			}
		}
	}

	return warnings
}

// warnDuplicateStackOrder warns when multiple stack entries share the same order value.
func (c *Config) warnDuplicateStackOrder() []string {
	if len(c.Stack) < 2 || len(c.Plans) > 0 {
		return nil
	}

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
		sort.Strings(names)
		if order == 0 {
			warnings = append(warnings,
				fmt.Sprintf("stack: entries %s have order 0 (default); set explicit order values to control startup sequence",
					strings.Join(names, ", ")))
		} else {
			warnings = append(warnings,
				fmt.Sprintf("stack: entries %s share the same order value %d; execution order between them is undefined",
					strings.Join(names, ", "), order))
		}
	}

	return warnings
}

// warnMultiStackComposeSplit warns when multiple stack entries use the compose plugin,
// which often indicates an overlay split that should be consolidated into one entry with modes.
func (c *Config) warnMultiStackComposeSplit() []string {
	var composeEntries []string
	for name, entry := range c.Stack {
		if entry.Compose != nil {
			composeEntries = append(composeEntries, name)
		}
	}
	if len(composeEntries) < 2 {
		return nil
	}
	sort.Strings(composeEntries)
	return []string{
		fmt.Sprintf("stack: multiple compose entries [%s] detected — consider consolidating into one entry and using modes.compose_services for service selection",
			strings.Join(composeEntries, ", ")),
	}
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

	for name, cmd := range c.Interaction {
		for subName, sub := range cmd.Subcommands {
			if cmd.Runner != "" && sub.Runner != "" && cmd.Runner != sub.Runner {
				warnings = append(warnings,
					fmt.Sprintf("interaction.%s.subcommands.%s: overrides parent runner (%s → %s); this may change execution backend unexpectedly",
						name, subName, cmd.Runner, sub.Runner))
			}
			if cmd.Pod != "" && sub.Pod != "" && cmd.Pod != sub.Pod {
				warnings = append(warnings,
					fmt.Sprintf("interaction.%s.subcommands.%s: overrides parent pod (%s → %s); this may change execution backend unexpectedly",
						name, subName, cmd.Pod, sub.Pod))
			}
		}
	}

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

	for name, cmd := range c.Interaction {
		if len(cmd.Subcommands) > 0 {
			// A parent is essentially a "group" node if it has no execution directives.
			// Calling it directly typically requires at least one execution target.
			isCallable := cmd.Command != "" || len(cmd.CommandLines) > 0 || cmd.HasScript() ||
				cmd.ScriptFile != "" || cmd.HasSteps() || cmd.HasHooks() || cmd.Compose != nil ||
				cmd.Runner != "" || cmd.Service != "" || cmd.Pod != ""
			if !isCallable {
				warnings = append(warnings,
					fmt.Sprintf("interaction.%s: has subcommands but is not directly callable; add an execution target or remove subcommands",
						name))
			}
		}
	}

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
