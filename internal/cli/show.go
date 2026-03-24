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

	// Compose
	if c.Compose.ProjectName != "" || len(c.Compose.Files) > 0 {
		fmt.Println()
		fmt.Println("Compose:")
		if c.Compose.ProjectName != "" {
			fmt.Printf("  Project: %s\n", c.Compose.ProjectName)
		}
		if len(c.Compose.Files) > 0 {
			fmt.Printf("  Files:   %s\n", strings.Join(c.Compose.Files, ", "))
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
			fmt.Printf("  %-*s  %s\n", maxLen, name, ep.Description)
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

	if c.Compose.ProjectName != "" || len(c.Compose.Files) > 0 {
		compose := map[string]any{}
		if c.Compose.ProjectName != "" {
			compose["project_name"] = c.Compose.ProjectName
		}
		if len(c.Compose.Files) > 0 {
			compose["files"] = c.Compose.Files
		}
		data["compose"] = compose
	}

	if len(c.Modes) > 0 {
		modes := make(map[string]string, len(c.Modes))
		for k, v := range c.Modes {
			modes[k] = v.Description
		}
		data["modes"] = modes
	}

	if len(c.Environments) > 0 {
		envs := make(map[string]string, len(c.Environments))
		for k, v := range c.Environments {
			envs[k] = v.Description
		}
		data["environments"] = envs
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
