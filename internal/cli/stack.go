package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/lifecycle"
)

var stackCmd = &cobra.Command{
	Use:   "stack [command]",
	Short: "Manage infrastructure lifecycle (compose, helm, kubectl, ...)",
	Long: `Manage stack entries defined in the 'stack' section of dva.yml.

Each stack entry represents an infrastructure component managed by a driver
(compose, helm, kubectl, kustomize, script, process, etc.).

Use subcommands to control individual or all stack entries.`,
	Example: `  dva stack up                    # Start all stack entries
  dva stack up compose            # Start a specific stack entry
  dva stack stop                  # Stop all (preserves state)
  dva stack down                  # Remove all stack resources
  dva stack status                # Show stack entry statuses
  dva stack log compose           # View logs for a stack entry`,
}

var stackUpCmd = &cobra.Command{
	Use:                "up [NAME...] [OPTIONS]",
	Short:              "Start stack entries (all if no name given)",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		mode, envName, includeTags, excludeTags, names := parseDvaFlags(args)
		mode, isDefault := applyDefaultMode(c, mode)

		force := false
		noWait := false
		var filteredNames []string
		for _, a := range names {
			switch a {
			case "--force":
				force = true
			case "--no-wait":
				noWait = true
			default:
				filteredNames = append(filteredNames, a)
			}
		}

		if err := applyEnv(e, c, envName); err != nil {
			return err
		}

		rm, err := resolveMode(c, mode)
		if err != nil {
			return err
		}
		if rm.Mode != nil {
			if isDefault {
				fmt.Fprintf(os.Stderr, "[mode: %s (default)] %s\n", mode, rm.Mode.Description)
			} else {
				fmt.Fprintf(os.Stderr, "[mode: %s] %s\n", mode, rm.Mode.Description)
			}
			if len(rm.Mode.Environment) > 0 {
				e.MergeVars(rm.Mode.Environment)
			}
		}

		orch := lifecycle.NewOrchestrator(c, e)
		if err := orch.Up(context.Background(), lifecycle.UpOptions{
			DryRun:      dryRun,
			Force:       force,
			Wait:        !noWait,
			Names:       filteredNames,
			IncludeTags: includeTags,
			ExcludeTags: excludeTags,
			Mode:        mode,
			Env:         envName,
		}); err != nil {
			return err
		}

		// Print status summary
		fmt.Fprintln(os.Stderr)
		status, statusErr := orch.Status(context.Background())
		if statusErr == nil {
			lifecycle.PrintStatus(status, c.FileDir())
		}

		return nil
	},
}

var stackStopCmd = &cobra.Command{
	Use:                "stop [NAME...] [OPTIONS]",
	Short:              "Stop stack entries without removing resources",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		mode, envName, includeTags, excludeTags, names := parseDvaFlags(args)
		mode, _ = applyDefaultMode(c, mode)

		if err := applyEnv(e, c, envName); err != nil {
			return err
		}

		rm, err := resolveMode(c, mode)
		if err != nil {
			return err
		}
		if rm.Mode != nil && len(rm.Mode.Environment) > 0 {
			e.MergeVars(rm.Mode.Environment)
		}

		orch := lifecycle.NewOrchestrator(c, e)
		return orch.Stop(context.Background(), lifecycle.StopOptions{
			DryRun:      dryRun,
			Names:       names,
			IncludeTags: includeTags,
			ExcludeTags: excludeTags,
			Mode:        mode,
			Env:         envName,
		})
	},
}

var stackDownCmd = &cobra.Command{
	Use:                "down [NAME...] [OPTIONS]",
	Short:              "Stop and remove stack resources",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		mode, envName, includeTags, excludeTags, names := parseDvaFlags(args)
		mode, _ = applyDefaultMode(c, mode)

		volumes := false
		var filteredNames []string
		for _, a := range names {
			switch a {
			case "-v", "--volumes":
				volumes = true
			default:
				filteredNames = append(filteredNames, a)
			}
		}

		if err := applyEnv(e, c, envName); err != nil {
			return err
		}

		rm, err := resolveMode(c, mode)
		if err != nil {
			return err
		}
		if rm.Mode != nil && len(rm.Mode.Environment) > 0 {
			e.MergeVars(rm.Mode.Environment)
		}

		orch := lifecycle.NewOrchestrator(c, e)
		return orch.Down(context.Background(), lifecycle.DownOptions{
			DryRun:      dryRun,
			Volumes:     volumes,
			Names:       filteredNames,
			IncludeTags: includeTags,
			ExcludeTags: excludeTags,
			Mode:        mode,
			Env:         envName,
		})
	},
}

var stackStatusCmd = &cobra.Command{
	Use:   "status [NAME...]",
	Short: "Show status of stack entries",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		orch := lifecycle.NewOrchestrator(c, e)
		status, err := orch.Status(context.Background())
		if err != nil {
			return err
		}

		// Filter by names if specified
		if len(args) > 0 {
			nameSet := make(map[string]bool, len(args))
			for _, n := range args {
				nameSet[n] = true
			}
			var filtered []lifecycle.EntryStatus
			for _, e := range status.Entries {
				if nameSet[e.Name] {
					filtered = append(filtered, e)
				}
			}
			status.Entries = filtered
		}

		lifecycle.PrintStatus(status, c.FileDir())
		return nil
	},
}

var stackLogCmd = &cobra.Command{
	Use:                "log [NAME] [OPTIONS]",
	Short:              "View logs for a stack entry (compose logs passthrough)",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)
		// Delegate to compose logs for now
		return execComposePassthrough(e, c, append([]string{"logs"}, args...))
	},
}

func init() {
	stackCmd.AddCommand(stackUpCmd)
	stackCmd.AddCommand(stackStopCmd)
	stackCmd.AddCommand(stackDownCmd)
	stackCmd.AddCommand(stackStatusCmd)
	stackCmd.AddCommand(stackLogCmd)
}
