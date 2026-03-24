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
  --foreground, -f          Run in foreground (attached) mode
  --force                   Bypass health check and force restart
  --no-wait                 Start services and return immediately without waiting
  --mode, -M MODE           Use a named mode from dva.yml modes section
  --env, -E ENV             Use a named environment from dva.yml environments section
  --tag, -T TAG[,TAG]       Include only compose services matching any of the given tags
  --exclude-tag TAG[,TAG]   Exclude compose services matching any of the given tags`,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		// Warn about compose file project name mismatches
		printComposeNameWarnings(c.ValidateComposeProjectNames())

		// Parse DVA-specific flags (--mode, --env, --tag, --exclude-tag extracted first)
		mode, envName, includeTags, excludeTags, args := parseDvaFlags(args)

		// Apply tag-based service filtering
		if len(includeTags) > 0 {
			included := c.GetComposeServicesIncluding(includeTags)
			if len(included) == 0 {
				fmt.Fprintf(os.Stderr, "[warn] no services matched tags: %v\n", includeTags)
				return nil
			}
			args = append(args, included...)
		} else if len(excludeTags) > 0 {
			if excluded := c.GetComposeServicesExcluding(excludeTags); len(excluded) > 0 {
				args = append(args, excluded...)
			}
		}

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

		// Resolve mode
		rm, err := resolveMode(c, mode)
		if err != nil {
			return err
		}
		if rm.Mode != nil {
			fmt.Fprintf(os.Stderr, "[mode: %s] %s\n", mode, rm.Mode.Description)
			if len(rm.Mode.Environment) > 0 {
				e.MergeVars(rm.Mode.Environment)
			}
		}
		if err := applyEnv(e, c, envName); err != nil {
			return err
		}
		filteredArgs = append(filteredArgs, rm.ServiceArgs...)

		// Suggest provision if mode defines one and marker doesn't exist
		if rm.Mode != nil && rm.Mode.Provision != "" {
			suggestProvision(c, rm.Mode.Provision)
		}

		// Native mode: skip compose, only run health checks
		if rm.SkipCompose {
			hcResults := runHealthChecksWithAutoStart(rm.HealthChecks, c.FileDir(), !noWait)
			if jsonOutput {
				return printServiceJSON(nil, c.Compose.ProjectName, false, hcResults)
			}
			printHealthCheckResults(hcResults, c.FileDir())
			printEndpointTable(c.Endpoints, rm.EndpointTags, hcResults)
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
					printRelatedServiceHints(services, c.Compose.Services)
					fmt.Fprintf(os.Stderr, "  Hint: use 'dva up --force' to force restart\n\n")
					printHealthCheckResults(hcResults, c.FileDir())
					printEndpointTable(c.Endpoints, rm.EndpointTags, hcResults)
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
		printRelatedServiceHints(services, c.Compose.Services)
		printHealthCheckResults(hcResults, c.FileDir())
		printEndpointTable(c.Endpoints, rm.EndpointTags, hcResults)
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

		mode, envName, includeTags, excludeTags, filteredArgs := parseDvaFlags(args)
		rm, err := resolveMode(c, mode)
		if err != nil {
			return err
		}
		if rm.Mode != nil {
			fmt.Fprintf(os.Stderr, "[mode: %s]\n", mode)
		}
		if err := applyEnv(e, c, envName); err != nil {
			return err
		}

		// Apply tag-based service filtering (down specific tagged services)
		if len(includeTags) > 0 {
			included := c.GetComposeServicesIncluding(includeTags)
			if len(included) == 0 {
				fmt.Fprintf(os.Stderr, "[warn] no services matched tags: %v\n", includeTags)
				return nil
			}
			filteredArgs = append(filteredArgs, included...)
		} else if len(excludeTags) > 0 {
			if included := c.GetComposeServicesExcluding(excludeTags); len(included) > 0 {
				filteredArgs = append(filteredArgs, included...)
			}
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

		mode, envName, includeTags, excludeTags, filteredArgs := parseDvaFlags(args)
		rm, err := resolveMode(c, mode)
		if err != nil {
			return err
		}
		if rm.Mode != nil {
			fmt.Fprintf(os.Stderr, "[mode: %s]\n", mode)
		}
		if err := applyEnv(e, c, envName); err != nil {
			return err
		}

		// Apply tag-based service filtering (stop specific tagged services)
		if len(includeTags) > 0 {
			included := c.GetComposeServicesIncluding(includeTags)
			if len(included) == 0 {
				fmt.Fprintf(os.Stderr, "[warn] no services matched tags: %v\n", includeTags)
				return nil
			}
			filteredArgs = append(filteredArgs, included...)
		} else if len(excludeTags) > 0 {
			if included := c.GetComposeServicesExcluding(excludeTags); len(included) > 0 {
				filteredArgs = append(filteredArgs, included...)
			}
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

		// When removing volumes, also clear provision markers so dva up re-suggests provisioning
		if volumes {
			clearProvisionMarkers(c.FileDir())
		}

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

		mode, envName, includeTags, excludeTags, filteredArgs := parseDvaFlags(args)
		rm, err := resolveMode(c, mode)
		if err != nil {
			return err
		}
		if rm.Mode != nil {
			fmt.Fprintf(os.Stderr, "[mode: %s]\n", mode)
			if len(rm.Mode.Environment) > 0 {
				e.MergeVars(rm.Mode.Environment)
			}
		}
		if err := applyEnv(e, c, envName); err != nil {
			return err
		}

		// Apply tag-based service filtering
		if len(includeTags) > 0 {
			included := c.GetComposeServicesIncluding(includeTags)
			if len(included) == 0 {
				fmt.Fprintf(os.Stderr, "[warn] no services matched tags: %v\n", includeTags)
				return nil
			}
			filteredArgs = append(filteredArgs, included...)
		} else if len(excludeTags) > 0 {
			if included := c.GetComposeServicesExcluding(excludeTags); len(included) > 0 {
				filteredArgs = append(filteredArgs, included...)
			}
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
		printEndpointTable(c.Endpoints, rm.EndpointTags, hcResults)
		return nil
	},
}

// parseDvaFlags extracts --mode/-M, --env/-E, --tags/-T, and --exclude-tags from args.
// excludeTags or includeTags is a slice of tag names.
func parseDvaFlags(args []string) (mode, env string, includeTags, excludeTags []string, filtered []string) {
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
		case a == "--tag" || a == "--tags" || a == "-T":
			if i+1 < len(args) {
				i++
				includeTags = append(includeTags, strings.Split(args[i], ",")...)
			}
		case strings.HasPrefix(a, "--tag="):
			val := strings.TrimPrefix(a, "--tag=")
			includeTags = append(includeTags, strings.Split(val, ",")...)
		case strings.HasPrefix(a, "--tags="):
			val := strings.TrimPrefix(a, "--tags=")
			includeTags = append(includeTags, strings.Split(val, ",")...)
		case strings.HasPrefix(a, "-T="):
			val := strings.TrimPrefix(a, "-T=")
			includeTags = append(includeTags, strings.Split(val, ",")...)
		case a == "--exclude-tag" || a == "--exclude-tags":
			if i+1 < len(args) {
				i++
				excludeTags = append(excludeTags, strings.Split(args[i], ",")...)
			}
		case strings.HasPrefix(a, "--exclude-tag="):
			val := strings.TrimPrefix(a, "--exclude-tag=")
			excludeTags = append(excludeTags, strings.Split(val, ",")...)
		case strings.HasPrefix(a, "--exclude-tags="):
			val := strings.TrimPrefix(a, "--exclude-tags=")
			excludeTags = append(excludeTags, strings.Split(val, ",")...)
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

// resolvedMode holds the result of resolving a --mode flag against config modes.
type resolvedMode struct {
	Mode         *config.ModeConfig
	ComposeArgs  []string // --profile flags for docker compose (global position)
	SkipCompose  bool
	ServiceArgs  []string // specific services to target
	HealthChecks map[string]config.HealthCheckConfig
	EndpointTags []string // filter endpoints by these tags (empty=show all)
}

// resolveMode looks up a mode name in config modes and returns resolved settings.
func resolveMode(c *config.Config, mode string) (*resolvedMode, error) {
	if mode == "" {
		return &resolvedMode{HealthChecks: c.HealthChecks}, nil
	}

	m, ok := c.Modes[mode]
	if !ok {
		available := make([]string, 0, len(c.Modes))
		for k := range c.Modes {
			available = append(available, k)
		}
		if len(available) == 0 {
			return nil, fmt.Errorf("mode '%s' not found. No modes defined in dva.yml under 'modes:'", mode)
		}
		return nil, fmt.Errorf("mode '%s' not found. Available: %s", mode, strings.Join(available, ", "))
	}

	rm := &resolvedMode{
		Mode:         &m,
		HealthChecks: c.HealthChecks,
	}

	for _, cp := range m.ComposeProfiles {
		rm.ComposeArgs = append(rm.ComposeArgs, "--profile", cp)
	}

	if m.ComposeServices != nil {
		if len(*m.ComposeServices) == 0 {
			rm.SkipCompose = true
		} else {
			rm.ServiceArgs = *m.ComposeServices
		}
	}

	if len(m.HealthChecks) > 0 {
		rm.HealthChecks = make(map[string]config.HealthCheckConfig)
		for _, name := range m.HealthChecks {
			if hc, ok := c.HealthChecks[name]; ok {
				rm.HealthChecks[name] = hc
			}
		}
	}

	rm.EndpointTags = m.EndpointTags

	return rm, nil
}

// suggestProvision checks if a provision profile has been run before
// (via marker file in .dva/) and prints a suggestion if not.
func suggestProvision(c *config.Config, provisionProfile string) {
	markerDir := filepath.Join(c.FileDir(), ".dva")
	markerFile := filepath.Join(markerDir, "provisioned-"+provisionProfile)

	if _, err := os.Stat(markerFile); err == nil {
		return // already provisioned
	}

	// Verify the provision profile exists
	if _, ok := c.Provision.Profiles[provisionProfile]; !ok {
		return
	}

	fmt.Fprintf(os.Stderr, "\n[hint] Provision profile '%s' has not been run yet.\n", provisionProfile)
	fmt.Fprintf(os.Stderr, "       Run: dva provision %s\n\n", provisionProfile)
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
// When forceSubprocess is true (set by hook wrapper for after-hooks), it delegates
// to execComposeSubprocess so the Go process survives for post-command hooks.
func execComposePassthrough(e *config.Environment, c *config.Config, args []string) error {
	if forceSubprocess {
		return execComposeSubprocess(e, c, args)
	}

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
