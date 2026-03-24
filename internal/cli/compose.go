package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
)

var composeCmd = &cobra.Command{
	Use:                "compose [ARGS...]",
	Short:              "Execute raw Docker Compose commands",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)
		return execComposePassthrough(e, c, args)
	},
}

var upCmd = &cobra.Command{
	Use:   "up [OPTIONS] [SERVICE...]",
	Short: "Create and start containers in the background",
	Long: `Create and start containers in detached mode by default.

If all services are already running and healthy, skips restart and shows status.
Local services with "start" in health_checks are auto-started and monitored.

DVA-specific flags (not passed to docker compose):
  --foreground, -f   Run in foreground (attached) mode
  --force            Bypass health check and force restart
  --no-wait          Start services and return immediately without waiting
  --mode, -M MODE    Use a named profile from dva.yml profiles section
  --env, -E ENV      Use a named environment from dva.yml environments section`,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		// Warn about compose file project name mismatches
		printComposeNameWarnings(c.ValidateComposeProjectNames())

		// Parse DVA-specific flags (--mode, --env extracted first)
		mode, envName, args := parseDvaFlags(args)

		foreground := false
		force := false
		noWait := false
		var filteredArgs []string
		for _, a := range args {
			switch a {
			case "--foreground", "-f":
				foreground = true
			case "--force":
				force = true
			case "--no-wait":
				noWait = true
			default:
				filteredArgs = append(filteredArgs, a)
			}
		}

		// Resolve profile/mode
		rm, err := resolveMode(c, mode)
		if err != nil {
			return err
		}
		if rm.Profile != nil {
			fmt.Fprintf(os.Stderr, "[mode: %s] %s\n", mode, rm.Profile.Description)
			if len(rm.Profile.Environment) > 0 {
				e.MergeVars(rm.Profile.Environment)
			}
		}
		if err := applyEnv(e, c, envName); err != nil {
			return err
		}
		filteredArgs = append(filteredArgs, rm.ServiceArgs...)

		// Native mode: skip compose, only run health checks
		if rm.SkipCompose {
			hcResults := runHealthChecksWithAutoStart(rm.HealthChecks, c.FileDir(), !noWait)
			if jsonOutput {
				return printServiceJSON(nil, c.Compose.ProjectName, false, hcResults)
			}
			printHealthCheckResults(hcResults, c.FileDir())
			return nil
		}

		if !foreground {
			defaults := c.Compose.UpOptions
			if len(defaults) == 0 {
				defaults = []string{"-d", "--wait"}
			}
			if noWait {
				// Remove --wait from defaults for immediate return
				var filtered []string
				for _, d := range defaults {
					if d != "--wait" {
						filtered = append(filtered, d)
					}
				}
				defaults = filtered
				if len(defaults) == 0 {
					defaults = []string{"-d"}
				}
			}
			// Prepend defaults if not already present
			existing := make(map[string]bool)
			for _, a := range filteredArgs {
				existing[a] = true
			}
			for i := len(defaults) - 1; i >= 0; i-- {
				if !existing[defaults[i]] {
					filteredArgs = append([]string{defaults[i]}, filteredArgs...)
				}
			}
		}

		// Build final args: [profile flags...] up [options...] [services...]
		upArgs := make([]string, 0, len(rm.ComposeArgs)+1+len(filteredArgs))
		upArgs = append(upArgs, rm.ComposeArgs...)
		upArgs = append(upArgs, "up")
		upArgs = append(upArgs, filteredArgs...)

		// Foreground mode: replace process (existing behavior)
		if foreground {
			return execComposePassthrough(e, c, upArgs)
		}

		// Detached mode: check if services are already running
		if !force {
			services, err := queryComposeServices(e, c)
			if err == nil && len(services) > 0 {
				requestedServices := extractServiceNames(filteredArgs)
				if allServicesHealthy(services, requestedServices) {
					projectName := c.Compose.ProjectName
					hcResults := runHealthChecksWithAutoStart(rm.HealthChecks, c.FileDir(), !noWait)
					if jsonOutput {
						return printServiceJSON(services, projectName, true, hcResults)
					}
					printServiceTable(services, projectName, true, c.Compose.Services)
					fmt.Fprintf(os.Stderr, "  Hint: use 'dva up --force' to force restart\n\n")
					printHealthCheckResults(hcResults, c.FileDir())
					return nil
				}
			}
		}

		// Run up as subprocess (not exec replace) so we can show status after
		if err := execComposeSubprocess(e, c, upArgs); err != nil {
			return err
		}

		// Show service status after successful up
		services, err := queryComposeServices(e, c)
		if err != nil {
			fmt.Fprintln(os.Stderr, "[warn] could not query service status")
			return nil
		}
		projectName := c.Compose.ProjectName
		hcResults := runHealthChecksWithAutoStart(rm.HealthChecks, c.FileDir(), !noWait)
		if jsonOutput {
			return printServiceJSON(services, projectName, false, hcResults)
		}
		printServiceTable(services, projectName, false, c.Compose.Services)
		printHealthCheckResults(hcResults, c.FileDir())
		return nil
	},
}

var downCmd = &cobra.Command{
	Use:                "down [OPTIONS]",
	Short:              "Stop and remove containers and network bridges",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		mode, envName, filteredArgs := parseDvaFlags(args)
		rm, err := resolveMode(c, mode)
		if err != nil {
			return err
		}
		if rm.Profile != nil {
			fmt.Fprintf(os.Stderr, "[mode: %s]\n", mode)
		}
		if err := applyEnv(e, c, envName); err != nil {
			return err
		}

		stopLocalServices(c.FileDir())

		if rm.SkipCompose {
			return nil
		}

		downArgs := make([]string, 0, len(rm.ComposeArgs)+2+len(filteredArgs))
		downArgs = append(downArgs, rm.ComposeArgs...)
		downArgs = append(downArgs, "down", "--remove-orphans")
		downArgs = append(downArgs, filteredArgs...)
		return execComposePassthrough(e, c, downArgs)
	},
}

var stopCmd = &cobra.Command{
	Use:                "stop [OPTIONS] [SERVICE...]",
	Short:              "Stop running containers without removing them",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		mode, envName, filteredArgs := parseDvaFlags(args)
		rm, err := resolveMode(c, mode)
		if err != nil {
			return err
		}
		if rm.Profile != nil {
			fmt.Fprintf(os.Stderr, "[mode: %s]\n", mode)
		}
		if err := applyEnv(e, c, envName); err != nil {
			return err
		}

		stopLocalServices(c.FileDir())

		if rm.SkipCompose {
			return nil
		}

		stopArgs := make([]string, 0, len(rm.ComposeArgs)+1+len(filteredArgs))
		stopArgs = append(stopArgs, rm.ComposeArgs...)
		stopArgs = append(stopArgs, "stop")
		stopArgs = append(stopArgs, filteredArgs...)
		return execComposePassthrough(e, c, stopArgs)
	},
}

var buildCmd = &cobra.Command{
	Use:                "build [OPTIONS] [SERVICE...]",
	Short:              "Build or rebuild services via Docker Compose",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)
		return execComposePassthrough(e, c, append([]string{"build"}, args...))
	},
}

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove all containers, networks, and isolated volumes",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		cleanArgs := []string{"down", "--remove-orphans"}

		volumes, _ := cmd.Flags().GetBool("volumes")
		images, _ := cmd.Flags().GetBool("images")
		force, _ := cmd.Flags().GetBool("force")

		if volumes {
			cleanArgs = append(cleanArgs, "--volumes")
		}
		if images {
			cleanArgs = append(cleanArgs, "--rmi", "local")
		}

		// Confirmation prompt for destructive operations
		if !force && (volumes || images) {
			msg := "This will remove all containers, networks"
			if volumes {
				msg += ", and VOLUMES (data loss!)"
			}
			if images {
				msg += ", and locally built images"
			}
			fmt.Fprintf(os.Stderr, "%s.\nContinue? [y/N] ", msg)
			var answer string
			fmt.Scanln(&answer)
			answer = strings.ToLower(strings.TrimSpace(answer))
			if answer != "y" && answer != "yes" {
				fmt.Println("Aborted.")
				return nil
			}
		}

		stopLocalServices(c.FileDir())
		return execComposePassthrough(e, c, cleanArgs)
	},
}

var logsCmd = &cobra.Command{
	Use:                "logs [OPTIONS] [SERVICE...]",
	Short:              "View output from containers",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)
		return execComposePassthrough(e, c, append([]string{"logs"}, args...))
	},
}

var restartCmd = &cobra.Command{
	Use:                "restart [OPTIONS] [SERVICE...]",
	Short:              "Restart containers (stop + start)",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		mode, envName, filteredArgs := parseDvaFlags(args)
		rm, err := resolveMode(c, mode)
		if err != nil {
			return err
		}
		if rm.Profile != nil {
			fmt.Fprintf(os.Stderr, "[mode: %s]\n", mode)
			if len(rm.Profile.Environment) > 0 {
				e.MergeVars(rm.Profile.Environment)
			}
		}
		if err := applyEnv(e, c, envName); err != nil {
			return err
		}

		stopLocalServices(c.FileDir())

		// Native mode: stop + restart health-checked services only
		if rm.SkipCompose {
			hcResults := runHealthChecksWithAutoStart(rm.HealthChecks, c.FileDir(), true)
			if jsonOutput {
				return printServiceJSON(nil, c.Compose.ProjectName, false, hcResults)
			}
			printHealthCheckResults(hcResults, c.FileDir())
			return nil
		}

		serviceArgs := append(filteredArgs, rm.ServiceArgs...)

		// Stop services
		stopArgs := make([]string, 0, len(rm.ComposeArgs)+1+len(serviceArgs))
		stopArgs = append(stopArgs, rm.ComposeArgs...)
		stopArgs = append(stopArgs, "stop")
		stopArgs = append(stopArgs, serviceArgs...)
		if err := execComposeSubprocess(e, c, stopArgs); err != nil {
			return err
		}

		// Start services
		upArgs := make([]string, 0, len(rm.ComposeArgs)+3+len(serviceArgs))
		upArgs = append(upArgs, rm.ComposeArgs...)
		upArgs = append(upArgs, "up", "-d", "--wait")
		upArgs = append(upArgs, serviceArgs...)
		if err := execComposeSubprocess(e, c, upArgs); err != nil {
			return err
		}

		// Show status
		services, err := queryComposeServices(e, c)
		if err != nil {
			return nil
		}
		hcResults := runHealthChecksWithAutoStart(rm.HealthChecks, c.FileDir(), true)
		if jsonOutput {
			return printServiceJSON(services, c.Compose.ProjectName, false, hcResults)
		}
		printServiceTable(services, c.Compose.ProjectName, false, c.Compose.Services)
		printHealthCheckResults(hcResults, c.FileDir())
		return nil
	},
}

// parseDvaFlags extracts --mode/-M and --env/-E from args.
func parseDvaFlags(args []string) (mode, env string, filtered []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--mode" || a == "-M":
			if i+1 < len(args) {
				i++
				mode = args[i]
			}
		case strings.HasPrefix(a, "--mode="):
			mode = strings.TrimPrefix(a, "--mode=")
		case strings.HasPrefix(a, "-M="):
			mode = strings.TrimPrefix(a, "-M=")
		case a == "--env" || a == "-E":
			if i+1 < len(args) {
				i++
				env = args[i]
			}
		case strings.HasPrefix(a, "--env="):
			env = strings.TrimPrefix(a, "--env=")
		case strings.HasPrefix(a, "-E="):
			env = strings.TrimPrefix(a, "-E=")
		default:
			filtered = append(filtered, a)
		}
	}
	return
}

// applyEnv resolves and applies environment configuration from --env flag.
func applyEnv(e *config.Environment, c *config.Config, envName string) error {
	if envName == "" {
		return nil
	}
	ec, ok := c.Environments[envName]
	if !ok {
		available := make([]string, 0, len(c.Environments))
		for k := range c.Environments {
			available = append(available, k)
		}
		if len(available) == 0 {
			return fmt.Errorf("env '%s' not found. No environments defined in dva.yml under 'environments:'", envName)
		}
		return fmt.Errorf("env '%s' not found. Available: %s", envName, strings.Join(available, ", "))
	}
	fmt.Fprintf(os.Stderr, "[env: %s] %s\n", envName, ec.Description)
	if len(ec.Environment) > 0 {
		e.MergeVars(ec.Environment)
	}
	return nil
}

// resolvedMode holds the result of resolving a --mode flag against config profiles.
type resolvedMode struct {
	Profile      *config.ProfileConfig
	ComposeArgs  []string // --profile flags for docker compose (global position)
	SkipCompose  bool
	ServiceArgs  []string // specific services to target
	HealthChecks map[string]config.HealthCheckConfig
}

// resolveMode looks up a mode name in config profiles and returns resolved settings.
func resolveMode(c *config.Config, mode string) (*resolvedMode, error) {
	if mode == "" {
		return &resolvedMode{HealthChecks: c.HealthChecks}, nil
	}

	p, ok := c.Profiles[mode]
	if !ok {
		available := make([]string, 0, len(c.Profiles))
		for k := range c.Profiles {
			available = append(available, k)
		}
		if len(available) == 0 {
			return nil, fmt.Errorf("mode '%s' not found. No profiles defined in dva.yml under 'profiles:'", mode)
		}
		return nil, fmt.Errorf("mode '%s' not found. Available: %s", mode, strings.Join(available, ", "))
	}

	rm := &resolvedMode{
		Profile:      &p,
		HealthChecks: c.HealthChecks,
	}

	for _, cp := range p.ComposeProfiles {
		rm.ComposeArgs = append(rm.ComposeArgs, "--profile", cp)
	}

	if p.ComposeServices != nil {
		if len(*p.ComposeServices) == 0 {
			rm.SkipCompose = true
		} else {
			rm.ServiceArgs = *p.ComposeServices
		}
	}

	if len(p.HealthChecks) > 0 {
		rm.HealthChecks = make(map[string]config.HealthCheckConfig)
		for _, name := range p.HealthChecks {
			if hc, ok := c.HealthChecks[name]; ok {
				rm.HealthChecks[name] = hc
			}
		}
	}

	return rm, nil
}

func init() {
	cleanCmd.Flags().BoolP("volumes", "v", false, "Also remove volumes (WARNING: data loss)")
	cleanCmd.Flags().BoolP("images", "i", false, "Also remove images built by docker compose")
	cleanCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
}

// execComposeSubprocess runs a docker compose command as a subprocess,
// returning control to the caller after completion.
func execComposeSubprocess(e *config.Environment, c *config.Config, args []string) error {
	composeCmd, composeArgs := buildComposeArgs(e, c, args)

	if dvaexec.Debug {
		fmt.Fprintf(os.Stderr, "[debug] compose subprocess: %s %v\n", composeCmd, composeArgs)
	}

	return dvaexec.ExecSubprocess(e, composeCmd, composeArgs, false)
}

// execComposePassthrough builds and execs a docker compose command using config.
func execComposePassthrough(e *config.Environment, c *config.Config, args []string) error {
	composeCmd, composeArgs := buildComposeArgs(e, c, args)

	if dvaexec.Debug {
		fmt.Fprintf(os.Stderr, "[debug] compose: %s %v\n", composeCmd, composeArgs)
	}

	return dvaexec.ExecReplace(e, composeCmd, composeArgs, false)
}

// buildComposeArgs builds docker compose arguments using config settings.
// Returns the command and args that can be used with exec or shell.
func buildComposeArgs(e *config.Environment, c *config.Config, args []string) (string, []string) {
	composeCmd := "docker"
	composeArgs := []string{"compose"}

	if c.Compose.Command != "" {
		parts := dvaexec.SplitCommand(c.Compose.Command)
		composeCmd = parts[0]
		if len(parts) > 1 {
			composeArgs = parts[1:]
		}
	}

	// Compose files
	cfgDir := c.FileDir()
	for _, f := range c.Compose.Files {
		f = e.Interpolate(f)
		if !filepath.IsAbs(f) {
			f = cfgDir + "/" + f
		}
		composeArgs = append(composeArgs, "-f", f)
	}

	// Project name
	if c.Compose.ProjectName != "" {
		composeArgs = append(composeArgs, "--project-name", e.Interpolate(c.Compose.ProjectName))
	}

	composeArgs = append(composeArgs, args...)
	return composeCmd, composeArgs
}
