package cli

import (
	"fmt"
	"os"

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
  --no-wait          Start services and return immediately without waiting`,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		// Parse custom flags from args
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

		upArgs := append([]string{"up"}, filteredArgs...)

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
					hcResults := runHealthChecksWithAutoStart(c.HealthChecks, c.FileDir(), !noWait)
					if jsonOutput {
						return printServiceJSON(services, projectName, true, hcResults)
					}
					printServiceTable(services, projectName, true)
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
		hcResults := runHealthChecksWithAutoStart(c.HealthChecks, c.FileDir(), !noWait)
		if jsonOutput {
			return printServiceJSON(services, projectName, false, hcResults)
		}
		printServiceTable(services, projectName, false)
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
		stopLocalServices(c.FileDir())
		return execComposePassthrough(e, c, append([]string{"down", "--remove-orphans"}, args...))
	},
}

var stopCmd = &cobra.Command{
	Use:                "stop [OPTIONS] [SERVICE...]",
	Short:              "Stop running containers without removing them",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)
		stopLocalServices(c.FileDir())
		return execComposePassthrough(e, c, append([]string{"stop"}, args...))
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

		if volumes {
			cleanArgs = append(cleanArgs, "--volumes")
		}
		if images {
			cleanArgs = append(cleanArgs, "--rmi", "local")
		}

		stopLocalServices(c.FileDir())
		return execComposePassthrough(e, c, cleanArgs)
	},
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
		if !isAbsPath(f) {
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

func isAbsPath(p string) bool {
	return len(p) > 0 && p[0] == '/'
}
