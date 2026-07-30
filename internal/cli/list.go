package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/output"
	"github.com/ScriptonBasestar/dva/internal/runner"
)

var (
	lsFormat   string
	lsDetailed bool
)

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List all available interaction scripts and commands",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		tree := runner.NewInteractionTree(c.Interaction)
		commands := tree.List()

		keys := sortedKeys(commands)

		if jsonOutput {
			lsFormat = "json"
		}

		switch lsFormat {
		case "json":
			return printJSON(c, commands, keys)
		case "yaml":
			return printYAML(c, commands, keys)
		default:
			return printTable(c, commands, keys)
		}
	},
}

func init() {
	lsCmd.Flags().StringVarP(&lsFormat, "format", "f", "table", "Output format (table, json, yaml)")
	lsCmd.Flags().BoolVarP(&lsDetailed, "detailed", "d", false, "Show detailed information")
}

func printTable(c *config.Config, commands map[string]*runner.ResolvedCommand, keys []string) error {
	// Calculate max width for alignment
	maxName := 0
	for _, k := range keys {
		if len(k) > maxName {
			maxName = len(k)
		}
	}

	for _, k := range keys {
		cmd := commands[k]
		// A shadowed row is still listed — the user declared it and needs to see dva received
		// it — but it must carry the form that reaches it, or the listing reads as an offer of
		// `dva <k>`, which runs the built-in instead.
		usage, shadowedBy := interactionUsage(c, k)
		mark := ""
		if shadowedBy != "" {
			mark = fmt.Sprintf("  (built-in '%s' takes this name; run: %s)", shadowedBy, usage)
		}
		if lsDetailed {
			runnerType := runner.DetectRunnerType(cmd)
			detail := ""
			if cmd.Service != "" {
				detail = fmt.Sprintf("service:%s", cmd.Service)
			} else if cmd.Pod != "" {
				detail = fmt.Sprintf("pod:%s", cmd.Pod)
			}
			fmt.Printf("%-*s  [%-14s]  %-20s  %s%s\n", maxName, k, runnerType, detail, cmd.Command, mark)
			if cmd.Description != "" {
				fmt.Printf("%s  # %s\n", strings.Repeat(" ", maxName), cmd.Description)
			}
		} else {
			if cmd.Description != "" {
				fmt.Printf("%-*s  # %s%s\n", maxName, k, cmd.Description, mark)
			} else {
				fmt.Printf("%-*s%s\n", maxName, k, mark)
			}
		}
	}

	if len(c.Plans) > 0 {
		planKeys := sortedKeys(c.Plans)
		fmt.Println()
		fmt.Println("Plans (dva up <name>):")
		maxLen := maxKeyLen(planKeys)
		for _, name := range planKeys {
			plan := c.Plans[name]
			desc := plan.Description
			if desc == "" {
				entryNames := make([]string, 0, len(plan.Entries))
				for _, e := range plan.Entries {
					entryNames = append(entryNames, e.Name)
				}
				desc = strings.Join(entryNames, " → ")
			}
			fmt.Printf("  %-*s  # %s\n", maxLen, name, desc)
		}
	}
	return nil
}

// interactionUsage returns the invocation that reaches the interaction named k, and the built-in
// that runs instead when the bare `dva k` form is typed ("" when nothing does).
//
// One function because `dva ls` and `dva manifest` describe the same key to the same reader and
// must not disagree about which form works — the manifest used to print `dva build` for an
// interaction that `dva build` provably could not reach.
//
// Subcommand keys arrive from the resolved tree space-separated ("build fast"), and a subcommand
// is reached through its parent, so it is the parent's name the built-in set is asked about. The
// parent's hook exemption does not carry over: measured, a `build` declaring replace: dispatches
// to that hook for `dva build fast` and ignores the argument, so the subcommand is reachable only
// through `dva run` even though the parent itself is reachable bare. That is why the exemption
// lives in ShadowedByBuiltin, consulted here for top-level keys only.
func interactionUsage(c *config.Config, k string) (usage, shadowedBy string) {
	if parent, _, nested := strings.Cut(k, " "); nested {
		if config.IsReservedCommand(parent) {
			return fmt.Sprintf("dva run %s", k), parent
		}
		return fmt.Sprintf("dva %s", k), ""
	}
	// c.Interaction, not the tree: hooks live on the declared command, and a top-level tree key
	// is always a declared key, so this lookup is exact.
	if config.ShadowedByBuiltin(k, c.Interaction[k]) {
		return fmt.Sprintf("dva run %s", k), k
	}
	return fmt.Sprintf("dva %s", k), ""
}

func buildCommandEntries(c *config.Config, commands map[string]*runner.ResolvedCommand, keys []string) map[string]any {
	entries := make(map[string]any, len(keys))
	for _, k := range keys {
		cmd := commands[k]
		entry := map[string]any{
			"command": cmd.Command,
			"runner":  runner.DetectRunnerType(cmd),
			"shell":   cmd.Shell,
		}
		if cmd.Description != "" {
			entry["description"] = cmd.Description
		}
		if cmd.Service != "" {
			entry["service"] = cmd.Service
			entry["compose_method"] = cmd.Compose.Method
		}
		if cmd.Pod != "" {
			entry["pod"] = cmd.Pod
		}
		// Only on the shadowed entries, so the field's presence is the signal and a consumer
		// never has to parse the description to learn the short form will not reach this.
		if _, shadowedBy := interactionUsage(c, k); shadowedBy != "" {
			entry["shadowed_by_builtin"] = shadowedBy
		}
		entries[k] = entry
	}
	return entries
}

func printJSON(c *config.Config, commands map[string]*runner.ResolvedCommand, keys []string) error {
	entries := buildCommandEntries(c, commands, keys)
	if len(c.Plans) == 0 {
		return output.PrintJSON(entries)
	}

	plans := make(map[string]any, len(c.Plans))
	for _, name := range sortedKeys(c.Plans) {
		p := c.Plans[name]
		if p == nil {
			continue
		}
		entryNames := make([]string, 0, len(p.Entries))
		for _, e := range p.Entries {
			entryNames = append(entryNames, e.Name)
		}
		plans[name] = map[string]any{
			"description": p.Description,
			"environment": p.Environment,
			"site":        p.Site,
			"entries":     entryNames,
		}
	}

	return output.PrintJSON(map[string]any{
		"interaction_commands": entries,
		"plans":                plans,
	})
}

func printYAML(c *config.Config, commands map[string]*runner.ResolvedCommand, keys []string) error {
	entries := buildCommandEntries(c, commands, keys)
	if len(c.Plans) == 0 {
		return output.PrintYAML(entries)
	}

	plans := make(map[string]any, len(c.Plans))
	for _, name := range sortedKeys(c.Plans) {
		p := c.Plans[name]
		if p == nil {
			continue
		}
		entryNames := make([]string, 0, len(p.Entries))
		for _, e := range p.Entries {
			entryNames = append(entryNames, e.Name)
		}
		plans[name] = map[string]any{
			"description": p.Description,
			"environment": p.Environment,
			"site":        p.Site,
			"entries":     entryNames,
		}
	}

	return output.PrintYAML(map[string]any{
		"interaction_commands": entries,
		"plans":                plans,
	})
}
