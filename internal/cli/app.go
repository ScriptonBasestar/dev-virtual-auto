package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/lifecycle"
)

var appCmd = &cobra.Command{
	Use:   "app [command]",
	Short: "Manage application lifecycle (ls, up, build, down, restart, log)",
	Long: `Manage application processes defined in the 'applications' section of dva.yml.

Use subcommands to list status, up, build, down, restart, and view logs of applications.`,
	Example: `  dva app ls              # List all applications and their status
  dva app up myapp        # Start a specific application
  dva app up myapp --dev  # Start in dev mode (hot-reload)
  dva app stop myapp      # Stop (preserves state for quick restart)
  dva app down myapp      # Stop and remove PID/log files
  dva app build myapp     # Build a specific application
  dva app restart myapp   # Restart a specific application
  dva app log myapp       # Show recent logs for an application`,
}

var appLsCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"status"},
	Short:   "List all applications with status, health, and PID",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		if len(c.Applications) == 0 {
			fmt.Fprintln(os.Stderr, "No applications defined in dva.yml")
			return nil
		}

		// Show active mode info (default mode only, no flag parsing for ls)
		printAppModeHeader(c)

		am := lifecycle.NewAppManager(c, e)
		statuses := am.AppStatuses()
		printAppStatuses(statuses)
		return nil
	},
}

var appStopCmd = &cobra.Command{
	Use:   "stop [APP...]",
	Short: "Stop applications without removing state (preserves PID for quick restart)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		if len(c.Applications) == 0 {
			return fmt.Errorf("no applications defined in dva.yml")
		}

		am := lifecycle.NewAppManager(c, e)
		am.HaltApps(args...)
		return nil
	},
}

var appDownCmd = &cobra.Command{
	Use:   "down [APP...]",
	Short: "Stop and remove application resources (PID files, logs)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		if len(c.Applications) == 0 {
			return fmt.Errorf("no applications defined in dva.yml")
		}

		am := lifecycle.NewAppManager(c, e)
		am.DownApps(args...)
		return nil
	},
}

var appUpCmd = &cobra.Command{
	Use:                "up [APP...] [--dev]",
	Short:              "Start applications (all if no name given)",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		if len(c.Applications) == 0 {
			return fmt.Errorf("no applications defined in dva.yml")
		}

		mode, _, _, _, args := parseDvaFlags(args)
		mode, isDefault := applyDefaultMode(c, mode)

		devMode := false
		var appNames []string
		for _, a := range args {
			if a == "--dev" {
				devMode = true
			} else {
				appNames = append(appNames, a)
			}
		}

		// Show mode header
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
			if rm.Mode.Provision != "" {
				suggestProvision(c, rm.Mode.Provision)
			}
		}

		am := lifecycle.NewAppManager(c, e)
		if err := am.StartApps(cmd.Context(), lifecycle.AppStartOptions{
			Names:   appNames,
			DevMode: devMode,
			Wait:    true,
			Mode:    mode,
		}); err != nil {
			return err
		}

		fmt.Fprintln(os.Stderr)
		statuses := am.AppStatuses()
		printAppStatuses(statuses)

		// Show endpoints with health status
		if len(c.Endpoints) > 0 {
			allHC := checkEndpointHealth(c.Endpoints)
			var epTags []string
			if rm.Mode != nil {
				epTags = rm.Mode.EndpointTags
			}
			printEndpointTable(c.Endpoints, epTags, allHC)
		}

		return nil
	},
}

var appRestartCmd = &cobra.Command{
	Use:                "restart [APP...] [--dev]",
	Short:              "Restart applications (stop then start)",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		if len(c.Applications) == 0 {
			return fmt.Errorf("no applications defined in dva.yml")
		}

		mode, _, _, _, args := parseDvaFlags(args)
		mode, _ = applyDefaultMode(c, mode)

		devMode := false
		var appNames []string
		for _, a := range args {
			if a == "--dev" {
				devMode = true
			} else {
				appNames = append(appNames, a)
			}
		}

		am := lifecycle.NewAppManager(c, e)
		am.HaltApps(appNames...)
		return am.StartApps(cmd.Context(), lifecycle.AppStartOptions{
			Names:   appNames,
			DevMode: devMode,
			Wait:    true,
			Mode:    mode,
		})
	},
}

var appBuildCmd = &cobra.Command{
	Use:                "build [APP...]",
	Short:              "Build applications (use --docker for container build)",
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
	appCmd.AddCommand(appUpCmd)
	appCmd.AddCommand(appStopCmd)
	appCmd.AddCommand(appDownCmd)
	appCmd.AddCommand(appRestartCmd)
	appCmd.AddCommand(appBuildCmd)
}

// printAppModeHeader shows the active mode description if a default mode is configured.
func printAppModeHeader(c *config.Config) {
	mode, isDefault := applyDefaultMode(c, "")
	if mode == "" {
		return
	}
	rm, err := resolveMode(c, mode)
	if err != nil || rm.Mode == nil {
		return
	}
	if isDefault {
		fmt.Fprintf(os.Stderr, "[mode: %s (default)] %s\n", mode, rm.Mode.Description)
	} else {
		fmt.Fprintf(os.Stderr, "[mode: %s] %s\n", mode, rm.Mode.Description)
	}
}

// printAppStatuses prints a formatted table of application statuses.
func printAppStatuses(statuses []lifecycle.AppStatus) {
	if len(statuses) == 0 {
		return
	}

	fmt.Fprintln(os.Stderr, "Applications:")
	w := tabwriter.NewWriter(os.Stderr, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "  NAME\tSTRATEGY\tSTATUS\tHEALTH\tURL\tPID")
	for _, s := range statuses {
		strategy := s.Strategy
		state := "stopped"
		if s.Running {
			state = "running"
		}
		health := "-"
		if s.Running && s.Healthy {
			health = "healthy"
		} else if s.Running {
			health = "unknown"
		}
		url := "-"
		if s.Port > 0 {
			url = fmt.Sprintf("http://localhost:%d", s.Port)
		}
		pid := "-"
		if s.PID > 0 {
			pid = fmt.Sprintf("%d", s.PID)
		}
		_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\t%s\n", s.Name, strategy, state, health, url, pid)
	}
	_ = w.Flush()
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
