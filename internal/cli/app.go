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

		// nil, not args: ls ignores positional arguments when applications exist (it lists
		// every one regardless), so answering `dva app ls typo` with a per-target failure
		// would make the same invocation succeed or fail depending on the section's presence.
		if len(c.Applications) == 0 {
			return noApplications(c, "list", nil)
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
			return noApplications(c, "stop", args)
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
			return noApplications(c, "remove", args)
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
		if helpRequested(args) {
			return cmd.Help()
		}
		c := mustLoadConfig()
		e := loadEnv(c)

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

		// After flag parsing, so the answer can depend on whether a target was named.
		if len(c.Applications) == 0 {
			return noApplications(c, "start", appNames)
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

		// Show only app-related endpoints
		if len(c.Endpoints) > 0 {
			appEPs := filterEndpointsByApps(c.Endpoints, appNames, c.Applications)
			if len(appEPs) > 0 {
				allHC := checkEndpointHealth(appEPs)
				printEndpointTable(appEPs, nil, allHC)
			}
		}

		// Fail the command when a started app's port is held by a process
		// dva did not start — otherwise a crash-on-bind or a stale orphan
		// masquerades as a successful `up`.
		if conflicts := am.PortConflicts(appNames...); len(conflicts) > 0 {
			return fmt.Errorf("%d application port(s) held by processes dva did not start (see FAIL lines above); run 'dva app down' to reclaim the port(s), then retry", len(conflicts))
		}

		return nil
	},
}

var appRestartCmd = &cobra.Command{
	Use:                "restart [APP...] [--dev]",
	Short:              "Restart applications (stop then start)",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if helpRequested(args) {
			return cmd.Help()
		}
		c := mustLoadConfig()
		e := loadEnv(c)

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

		// After flag parsing, so the answer can depend on whether a target was named.
		if len(c.Applications) == 0 {
			return noApplications(c, "restart", appNames)
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
		if helpRequested(args) {
			return cmd.Help()
		}
		c := mustLoadConfig()
		e := loadEnv(c)

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

		// After flag parsing, so the answer can depend on whether a target was named.
		if len(c.Applications) == 0 {
			return noApplications(c, "build", appNames)
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
	appCmd.AddCommand(appLogCmd)
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
		switch {
		case s.Running && s.Healthy:
			health = "healthy"
		case s.PortPID > 0 && !s.PortOwned:
			// Port is answered by a process dva did not start — a stale orphan
			// from a previous run, or a child that crashed on bind. Surface the
			// real owner instead of masking it as healthy/unknown.
			health = fmt.Sprintf("foreign:%d", s.PortPID)
		case s.Running:
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
		// Indent variant names (contain a dot after the first segment)
		displayName := s.Name
		if strings.Contains(displayName, ".") {
			displayName = "  └ " + displayName
		}
		_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\t%s\n", displayName, strategy, state, health, url, pid)
	}
	_ = w.Flush()
}

// declaredSurfaces lists what the config does declare, with counts, in the order a user
// meets them. `applications:` is deliberately absent — the caller is answering its absence.
func declaredSurfaces(c *config.Config) []string {
	var parts []string
	if n := len(c.Plans); n > 0 {
		parts = append(parts, fmt.Sprintf("plans (%d)", n))
	}
	if n := len(c.Stack); n > 0 {
		parts = append(parts, fmt.Sprintf("stack (%d)", n))
	}
	if n := len(c.Interaction); n > 0 {
		parts = append(parts, fmt.Sprintf("interaction (%d)", n))
	}
	return parts
}

// absentApplicationsAdvice states what the config declares instead of applications, and
// which command acts on it. Three things it deliberately does not do:
//
//   - It never names a config file. Config is the merge of modules: and subprojects:, so the
//     file that would need an `applications:` block is not knowable from the loaded config.
//   - It never routes to `dva stack up` (USAGE.md — the stack is a declaration store, so
//     that is no longer the recommended model) or to another `dva app` subcommand, which
//     would land the reader back here.
//   - It never suggests a `dva up` form that would refuse. Bare `dva up` fails with
//     "multiple plans configured" when plans exist without a default, and reports
//     "(no entries configured)" when nothing but interactions are declared.
func absentApplicationsAdvice(c *config.Config) string {
	surfaces := declaredSurfaces(c)
	if len(surfaces) == 0 {
		return "this config declares no plans, stack entries, or interactions either"
	}
	declares := fmt.Sprintf("this config declares %s", strings.Join(surfaces, ", "))
	switch {
	case c.DefaultPlan() != "":
		return fmt.Sprintf("%s — run 'dva up %s' to start the declared lifecycle", declares, c.DefaultPlan())
	case c.HasPlans():
		return fmt.Sprintf("%s — run 'dva up <%s>' to start one of them", declares, strings.Join(sortedPlanNames(c), "|"))
	case len(c.Stack) > 0:
		return declares + " — run 'dva up' to start the declared lifecycle"
	default:
		return declares + " — run 'dva ls' to list them"
	}
}

// noApplications answers an absent `applications:` section for every `dva app` subcommand.
// action names what the subcommand would have done, and is used only by the named form.
//
// Bare invocation is not a failure: acting on all applications when there are none is a
// no-op, and a script that chains `dva app up` in a stack-only project must keep working.
// Naming a target is a failure — swallowing `dva app up myapp-typo` would report success
// for something that never ran.
//
// The named form does not say the application was "not found". The set it would be found in
// does not exist, and blaming the name for that sends the reader hunting for a typo.
func noApplications(c *config.Config, action string, names []string) error {
	if len(names) == 0 {
		fmt.Fprintf(os.Stderr, "no applications declared; %s\n", absentApplicationsAdvice(c))
		return nil
	}
	return fmt.Errorf("no applications declared, so there is no '%s' to %s; %s", names[0], action, absentApplicationsAdvice(c))
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

		// log had no guard at all: with no applications: section the status loop below
		// matched nothing and it fell through to "application 'x' not found", which blames
		// the name for a section that does not exist.
		if len(c.Applications) == 0 {
			return noApplications(c, "show logs for", args)
		}

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
