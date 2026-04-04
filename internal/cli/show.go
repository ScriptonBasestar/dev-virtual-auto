package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/output"
)

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Show registered configuration summary (modes, environments, commands)",
	Long: `Display a human-readable summary of the current dva.yml configuration.
Shows all registered modes (--mode), environments (--env), interaction commands,
provision profiles, health checks, and subprojects at a glance.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()

		if jsonOutput {
			return showJSON(c)
		}
		return showText(c)
	},
}

func showText(c *config.Config) error {
	// Header
	fmt.Printf("DVA v%s\n", config.Version)
	fmt.Printf("Config: %s\n", c.FilePath())
	if c.Version != "" {
		fmt.Printf("  Required version: %s\n", c.Version)
	}

	// Lifecycle / Compose
	if cc := c.PrimaryComposeConfig(); cc != nil {
		if cc.ProjectName != "" || len(cc.Files) > 0 {
			fmt.Println()
			fmt.Println("Compose:")
			if cc.ProjectName != "" {
				fmt.Printf("  Project: %s\n", cc.ProjectName)
			}
			if len(cc.Files) > 0 {
				fmt.Printf("  Files:   %s\n", strings.Join(cc.Files, ", "))
			}
		}
	}

	// Modes (--mode)
	if len(c.Modes) > 0 {
		fmt.Println()
		fmt.Println("Modes (--mode/-M):")
		names := sortedKeys(c.Modes)
		maxLen := maxKeyLen(names)
		for _, name := range names {
			m := c.Modes[name]
			fmt.Printf("  %-*s  %s\n", maxLen, name, m.Description)
		}
	}

	// Environments (--env)
	if len(c.Environments) > 0 {
		fmt.Println()
		fmt.Println("Environments (--env/-E):")
		names := sortedKeys(c.Environments)
		maxLen := maxKeyLen(names)
		for _, name := range names {
			ep := c.Environments[name]
			desc := ep.Description
			var parts []string
			if len(ep.Environment) > 0 {
				parts = append(parts, fmt.Sprintf("%d vars", len(ep.Environment)))
			}
			if len(ep.Stack) > 0 {
				parts = append(parts, fmt.Sprintf("%d stack filters", len(ep.Stack)))
			}
			if len(parts) > 0 {
				if desc != "" {
					desc += " "
				}
				desc += "(" + strings.Join(parts, ", ") + ")"
			}
			fmt.Printf("  %-*s  %s\n", maxLen, name, desc)
		}
	}

	if len(c.Plans) > 0 {
		fmt.Println()
		fmt.Println("Plans (dva up <name>):")
		names := sortedKeys(c.Plans)
		maxLen := maxKeyLen(names)
		for _, name := range names {
			plan := c.Plans[name]
			if plan == nil {
				continue
			}
			var parts []string
			if plan.Environment != "" {
				parts = append(parts, "env:"+plan.Environment)
			}
			if plan.Site != "" {
				parts = append(parts, "site:"+plan.Site)
			}
			if len(plan.Entries) > 0 {
				parts = append(parts, fmt.Sprintf("%d entries", len(plan.Entries)))
			}
			desc := plan.Description
			if len(parts) > 0 {
				if desc == "" {
					desc = "[" + strings.Join(parts, ", ") + "]"
				} else {
					desc += " [" + strings.Join(parts, ", ") + "]"
				}
			}
			fmt.Printf("  %-*s  %s\n", maxLen, name, desc)
		}
	}

	if len(c.Sites) > 0 {
		fmt.Println()
		fmt.Println("Sites:")
		names := sortedKeys(c.Sites)
		maxLen := maxKeyLen(names)
		for _, name := range names {
			site := c.Sites[name]
			if site == nil {
				continue
			}
			desc := site.Description
			if len(site.Vars) > 0 {
				if desc == "" {
					desc = fmt.Sprintf("(%d vars)", len(site.Vars))
				} else {
					desc += fmt.Sprintf(" (%d vars)", len(site.Vars))
				}
			}
			fmt.Printf("  %-*s  %s\n", maxLen, name, desc)
		}
	}

	// Applications
	if len(c.Applications) > 0 {
		fmt.Println()
		fmt.Printf("Applications: %d defined\n", len(c.Applications))
		names := sortedKeys(c.Applications)
		maxLen := maxKeyLen(names)
		for _, name := range names {
			app := c.Applications[name]
			strategies := []string{}
			if app.Run.HasNative() || app.Dev.HasNative() {
				strategies = append(strategies, "native")
			}
			if app.Run.HasDocker() || app.Dev.HasDocker() {
				strategies = append(strategies, "docker")
			}
			desc := app.Description
			if len(strategies) > 0 {
				desc += fmt.Sprintf(" [%s]", strings.Join(strategies, "/"))
			}
			fmt.Printf("  %-*s  %s\n", maxLen, name, desc)
		}
	}

	// Interaction commands
	if len(c.Interaction) > 0 {
		fmt.Println()
		fmt.Printf("Interaction Commands: %d defined\n", len(c.Interaction))
		names := sortedKeys(c.Interaction)
		maxLen := maxKeyLen(names)
		for _, name := range names {
			ic := c.Interaction[name]
			desc := ic.Description
			subCount := countSubcommands(ic)
			if subCount > 0 {
				desc += fmt.Sprintf(" (+%d sub)", subCount)
			}
			fmt.Printf("  %-*s  %s\n", maxLen, name, desc)
		}
	}

	// Provision profiles
	if len(c.Provision.Profiles) > 0 {
		fmt.Println()
		names := sortedKeys(c.Provision.Profiles)
		if c.Provision.DefaultProfile != "" {
			for i, n := range names {
				if n == c.Provision.DefaultProfile {
					names[i] = n + " (default)"
				}
			}
		}
		fmt.Printf("Provision Profiles: %s\n", strings.Join(names, ", "))
	}

	// Health checks
	if len(c.HealthChecks) > 0 {
		fmt.Println()
		fmt.Printf("Health Checks: %s\n", strings.Join(sortedKeys(c.HealthChecks), ", "))
	}

	// Endpoints
	if len(c.Endpoints) > 0 {
		fmt.Println()
		fmt.Printf("Endpoints:\n")
		names := sortedKeys(c.Endpoints)
		maxLen := maxKeyLen(names)
		for _, name := range names {
			ep := c.Endpoints[name]
			fmt.Printf("  %-*s  %s  %s\n", maxLen, name, ep.Label, ep.URL)
		}
	}

	// Subprojects
	if len(c.Subprojects) > 0 {
		fmt.Println()
		fmt.Printf("Subprojects:\n")
		for _, name := range sortedKeys(c.Subprojects) {
			sub := c.Subprojects[name]
			tags := ""
			if len(sub.ExcludeTags) > 0 {
				tags = fmt.Sprintf(" (exclude: %s)", strings.Join(sub.ExcludeTags, ", "))
			}
			fmt.Printf("  %s -> %s%s\n", name, sub.Path, tags)
		}
	}

	// Infra
	if len(c.Infra) > 0 {
		fmt.Println()
		fmt.Printf("Infra: %s\n", strings.Join(sortedKeys(c.Infra), ", "))
	}

	// Environment variables count
	if len(c.Environment) > 0 {
		fmt.Println()
		fmt.Printf("Environment Variables: %d defined\n", len(c.Environment))
	}

	return nil
}

func showJSON(c *config.Config) error {
	data := map[string]any{
		"dva_version":    config.Version,
		"config_path":    c.FilePath(),
		"config_version": c.Version,
	}

	if cc := c.PrimaryComposeConfig(); cc != nil {
		if cc.ProjectName != "" || len(cc.Files) > 0 {
			compose := map[string]any{}
			if cc.ProjectName != "" {
				compose["project_name"] = cc.ProjectName
			}
			if len(cc.Files) > 0 {
				compose["files"] = cc.Files
			}
			data["compose"] = compose
		}
	}

	if len(c.Modes) > 0 {
		modes := make(map[string]string, len(c.Modes))
		for k, v := range c.Modes {
			modes[k] = v.Description
		}
		data["modes"] = modes
	}

	if len(c.Environments) > 0 {
		envs := make(map[string]any, len(c.Environments))
		for k, v := range c.Environments {
			envs[k] = map[string]any{
				"description": v.Description,
				"vars_count":  len(v.Environment),
				"stack_count": len(v.Stack),
			}
		}
		data["environments"] = envs
	}

	if len(c.Plans) > 0 {
		plans := make(map[string]any, len(c.Plans))
		for k, v := range c.Plans {
			if v == nil {
				continue
			}
			entryNames := make([]string, 0, len(v.Entries))
			for _, e := range v.Entries {
				entryNames = append(entryNames, e.Name)
			}
			plans[k] = map[string]any{
				"description": v.Description,
				"environment": v.Environment,
				"site":        v.Site,
				"entries":     entryNames,
			}
		}
		data["plans"] = plans
	}

	if len(c.Sites) > 0 {
		sites := make(map[string]any, len(c.Sites))
		for k, v := range c.Sites {
			if v == nil {
				continue
			}
			sites[k] = map[string]any{
				"description": v.Description,
				"vars_count":  len(v.Vars),
			}
		}
		data["sites"] = sites
	}

	if len(c.Applications) > 0 {
		apps := make(map[string]any, len(c.Applications))
		for k, v := range c.Applications {
			entry := map[string]any{"description": v.Description}
			if v.Run.HasNative() || v.Dev.HasNative() {
				entry["native"] = true
			}
			if v.Run.HasDocker() || v.Dev.HasDocker() {
				entry["docker"] = true
			}
			if len(v.Tags) > 0 {
				entry["tags"] = v.Tags
			}
			apps[k] = entry
		}
		data["applications"] = apps
	}

	if len(c.Interaction) > 0 {
		cmds := make(map[string]any, len(c.Interaction))
		for k, v := range c.Interaction {
			entry := map[string]any{"description": v.Description}
			if sub := countSubcommands(v); sub > 0 {
				entry["subcommands"] = sub
			}
			cmds[k] = entry
		}
		data["interaction_commands"] = cmds
	}

	if len(c.Provision.Profiles) > 0 {
		provData := map[string]any{
			"profiles": sortedKeys(c.Provision.Profiles),
		}
		if c.Provision.DefaultProfile != "" {
			provData["default_profile"] = c.Provision.DefaultProfile
		}
		data["provision"] = provData
	}

	if len(c.HealthChecks) > 0 {
		data["health_checks"] = sortedKeys(c.HealthChecks)
	}

	if len(c.Endpoints) > 0 {
		eps := make(map[string]any, len(c.Endpoints))
		for k, v := range c.Endpoints {
			eps[k] = map[string]any{"label": v.Label, "url": v.URL}
		}
		data["endpoints"] = eps
	}

	if len(c.Subprojects) > 0 {
		subs := make(map[string]string, len(c.Subprojects))
		for k, v := range c.Subprojects {
			subs[k] = v.Path
		}
		data["subprojects"] = subs
	}

	if len(c.Infra) > 0 {
		data["infra"] = sortedKeys(c.Infra)
	}

	data["environment_variables_count"] = len(c.Environment)

	return output.PrintJSON(data)
}

func countSubcommands(ic *config.InteractionCommand) int {
	if ic.Subcommands == nil {
		return 0
	}
	count := len(ic.Subcommands)
	for _, sub := range ic.Subcommands {
		count += countSubcommands(sub)
	}
	return count
}

// sortedKeys returns sorted keys from a map. Uses type parameter for flexibility.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func maxKeyLen(keys []string) int {
	max := 0
	for _, k := range keys {
		if len(k) > max {
			max = len(k)
		}
	}
	return max
}
