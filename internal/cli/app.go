package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/lifecycle"
)

var appCmd = &cobra.Command{
	Use:   "app [command]",
	Short: "Manage application lifecycle (ls, start, build, stop, restart, log)",
	Long: `Manage application processes defined in the 'applications' section of dva.yml.

Use subcommands to list status, start, build, stop, restart, and view logs of applications.`,
	Example: `  dva app ls              # List all applications and their status
  dva app start myapp     # Start a specific application
  dva app build myapp     # Build a specific application
  dva app stop myapp      # Stop a specific application
  dva app restart myapp   # Restart a specific application
  dva app log myapp       # Show recent logs for an application`,
}

var appLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List all applications with status, health, and PID",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		if len(c.Applications) == 0 {
			fmt.Fprintln(os.Stderr, "No applications defined in dva.yml")
			return nil
		}

		am := lifecycle.NewAppManager(c, e)
		statuses := am.AppStatuses()
		printAppStatuses(statuses)
		return nil
	},
}

var appStopCmd = &cobra.Command{
	Use:   "stop [APP...]",
	Short: "Stop running applications (all if no name given)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		if len(c.Applications) == 0 {
			return fmt.Errorf("no applications defined in dva.yml")
		}

		am := lifecycle.NewAppManager(c, e)
		am.StopApps(args...)
		return nil
	},
}

var appStartCmd = &cobra.Command{
	Use:   "start [APP...]",
	Short: "Start applications (all if no name given)",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		if len(c.Applications) == 0 {
			return fmt.Errorf("no applications defined in dva.yml")
		}

		mode, _, _, _, args := parseDvaFlags(args)
		mode, _ = applyDefaultMode(c, mode)

		am := lifecycle.NewAppManager(c, e)
		return am.StartApps(cmd.Context(), lifecycle.AppStartOptions{
			Names:   args,
			DevMode: true,
			Wait:    true,
			Mode:    mode,
		})
	},
}

var appRestartCmd = &cobra.Command{
	Use:   "restart [APP...]",
	Short: "Restart applications (stop then start)",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		if len(c.Applications) == 0 {
			return fmt.Errorf("no applications defined in dva.yml")
		}

		mode, _, _, _, args := parseDvaFlags(args)
		mode, _ = applyDefaultMode(c, mode)

		am := lifecycle.NewAppManager(c, e)
		am.StopApps(args...)
		return am.StartApps(cmd.Context(), lifecycle.AppStartOptions{
			Names:   args,
			DevMode: true,
			Wait:    true,
			Mode:    mode,
		})
	},
}

var appBuildCmd = &cobra.Command{
	Use:   "build [APP...]",
	Short: "Build applications (use --docker for container build)",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		if len(c.Applications) == 0 {
			return fmt.Errorf("no applications defined in dva.yml")
		}

		mode, _, _, _, args := parseDvaFlags(args)
		mode, _ = applyDefaultMode(c, mode)

		docker := false
		var appNames []string
		for _, a := range args {
			if a == "--docker" {
				docker = true
			} else {
				appNames = append(appNames, a)
			}
		}

		am := lifecycle.NewAppManager(c, e)
		return am.BuildApps(cmd.Context(), lifecycle.AppStartOptions{
			Strategy: boolToStrategy(docker),
			Names:    appNames,
			DryRun:   dryRun,
			Mode:     mode,
		})
	},
}

func init() {
	appCmd.AddCommand(appLsCmd)
	appCmd.AddCommand(appStartCmd)
	appCmd.AddCommand(appStopCmd)
	appCmd.AddCommand(appRestartCmd)
	appCmd.AddCommand(appBuildCmd)
}

// printAppStatuses prints a formatted table of application statuses.
func printAppStatuses(statuses []lifecycle.AppStatus) {
	if len(statuses) == 0 {
		return
	}

	fmt.Fprintln(os.Stderr, "Applications:")
	w := tabwriter.NewWriter(os.Stderr, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  NAME\tSTATUS\tHEALTH\tPID")
	for _, s := range statuses {
		state := s.Strategy
		if s.Running {
			state = "running"
		}
		health := "-"
		if s.Running && s.Healthy {
			health = "healthy"
		} else if s.Running {
			health = "unknown"
		}
		pid := "-"
		if s.PID > 0 {
			pid = fmt.Sprintf("%d", s.PID)
		}
		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", s.Name, state, health, pid)
	}
	w.Flush()
}

func boolToStrategy(docker bool) string {
	if docker {
		return "docker"
	}
	return ""
}

// appLogCmd prints app log file contents.
var appLogCmd = &cobra.Command{
	Use:   "log <APP>",
	Short: "Show recent logs for an application (last 100 lines)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		am := lifecycle.NewAppManager(c, e)
		statuses := am.AppStatuses()

		for _, s := range statuses {
			if s.Name == args[0] {
				if s.LogFile == "" {
					return fmt.Errorf("no log file for %s", args[0])
				}
				data, err := os.ReadFile(s.LogFile)
				if err != nil {
					return fmt.Errorf("read log: %w", err)
				}
				// Print last 100 lines
				lines := strings.Split(string(data), "\n")
				start := 0
				if len(lines) > 100 {
					start = len(lines) - 100
				}
				for _, line := range lines[start:] {
					fmt.Println(line)
				}
				return nil
			}
		}

		return fmt.Errorf("application '%s' not found", args[0])
	},
}

func init() {
	appCmd.AddCommand(appLogCmd)
}
