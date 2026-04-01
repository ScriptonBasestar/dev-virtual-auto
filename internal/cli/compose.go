package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
	"github.com/ScriptonBasestar/dva/internal/lifecycle"
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
	Short: "Create and start services",
	Long: `Create and start services via lifecycle plugins.

DVA-specific flags:
  --force                   Force restart even if already running
  --no-wait                 Start services and return immediately without waiting
  --mode, -M MODE           Use a named mode from dva.yml modes section
  --env, -E ENV             Use a named environment from dva.yml environments section
  --tag, -T TAG[,TAG]       Include only lifecycle entries matching any of the given tags
  --exclude-tag TAG[,TAG]   Exclude lifecycle entries matching any of the given tags`,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		mode, envName, includeTags, excludeTags, args := parseDvaFlags(args)
		mode, isDefault := applyDefaultMode(c, mode)

		force := false
		noWait := false
		for _, a := range args {
			switch a {
			case "--force":
				force = true
			case "--no-wait":
				noWait = true
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
			if rm.Mode.Provision != "" {
				suggestProvision(c, rm.Mode.Provision)
			}
		}

		orch := lifecycle.NewOrchestrator(c, e)
		upErr := orch.Up(context.Background(), lifecycle.UpOptions{
			DryRun:      dryRun,
			Force:       force,
			Wait:        !noWait,
			IncludeTags: includeTags,
			ExcludeTags: excludeTags,
			Mode:        mode,
			Env:         envName,
		})

		// Print status summary and endpoints regardless of up errors,
		// so users can see connection info for services that did start.
		fmt.Fprintln(os.Stderr)
		status, statusErr := orch.Status(context.Background())
		if statusErr == nil {
			lifecycle.PrintStatus(status, c.FileDir())
		}
		if len(c.Endpoints) > 0 {
			allHC := checkEndpointHealth(c.Endpoints)
			var epTags []string
			if rm.Mode != nil {
				epTags = rm.Mode.EndpointTags
			}
			printEndpointTable(c.Endpoints, epTags, allHC)
		}

		return upErr
	},
}

var downCmd = &cobra.Command{
	Use:                "down [OPTIONS]",
	Short:              "Stop and remove services",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		mode, envName, includeTags, excludeTags, _ := parseDvaFlags(args)
		mode, isDefault := applyDefaultMode(c, mode)

		if err := applyEnv(e, c, envName); err != nil {
			return err
		}

		rm, err := resolveMode(c, mode)
		if err != nil {
			return err
		}
		if rm.Mode != nil {
			if isDefault {
				fmt.Fprintf(os.Stderr, "[mode: %s (default)]\n", mode)
			} else {
				fmt.Fprintf(os.Stderr, "[mode: %s]\n", mode)
			}
			if len(rm.Mode.Environment) > 0 {
				e.MergeVars(rm.Mode.Environment)
			}
		}

		orch := lifecycle.NewOrchestrator(c, e)
		return orch.Down(context.Background(), lifecycle.DownOptions{
			DryRun:      dryRun,
			IncludeTags: includeTags,
			ExcludeTags: excludeTags,
			Mode:        mode,
			Env:         envName,
		})
	},
}

var stopCmd = &cobra.Command{
	Use:                "stop [OPTIONS] [SERVICE...]",
	Short:              "Stop running services without removing them",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		mode, envName, includeTags, excludeTags, _ := parseDvaFlags(args)
		mode, isDefault := applyDefaultMode(c, mode)

		if err := applyEnv(e, c, envName); err != nil {
			return err
		}

		rm, err := resolveMode(c, mode)
		if err != nil {
			return err
		}
		if rm.Mode != nil {
			if isDefault {
				fmt.Fprintf(os.Stderr, "[mode: %s (default)]\n", mode)
			} else {
				fmt.Fprintf(os.Stderr, "[mode: %s]\n", mode)
			}
			if len(rm.Mode.Environment) > 0 {
				e.MergeVars(rm.Mode.Environment)
			}
		}

		orch := lifecycle.NewOrchestrator(c, e)
		return orch.Stop(context.Background(), lifecycle.StopOptions{
			DryRun:      dryRun,
			IncludeTags: includeTags,
			ExcludeTags: excludeTags,
			Mode:        mode,
			Env:         envName,
		})
	},
}

var restartCmd = &cobra.Command{
	Use:                "restart [OPTIONS] [SERVICE...]",
	Short:              "Restart services (stop + start)",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		mode, envName, includeTags, excludeTags, _ := parseDvaFlags(args)
		mode, isDefault := applyDefaultMode(c, mode)

		if err := applyEnv(e, c, envName); err != nil {
			return err
		}

		rm, err := resolveMode(c, mode)
		if err != nil {
			return err
		}
		if rm.Mode != nil {
			if isDefault {
				fmt.Fprintf(os.Stderr, "[mode: %s (default)]\n", mode)
			} else {
				fmt.Fprintf(os.Stderr, "[mode: %s]\n", mode)
			}
			if len(rm.Mode.Environment) > 0 {
				e.MergeVars(rm.Mode.Environment)
			}
		}

		orch := lifecycle.NewOrchestrator(c, e)
		return orch.Restart(context.Background(), lifecycle.UpOptions{
			DryRun:      dryRun,
			Force:       true,
			Wait:        true,
			IncludeTags: includeTags,
			ExcludeTags: excludeTags,
			Mode:        mode,
			Env:         envName,
		})
	},
}

var buildCmd = &cobra.Command{
	Use:                "build [OPTIONS] [SERVICE...]",
	Short:              "Build or rebuild services (mode-aware: docker or native)",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		mode, _, _, _, remaining := parseDvaFlags(args)
		mode, _ = applyDefaultMode(c, mode)

		// Check mode build strategy
		if mode != "" {
			if m, ok := c.Modes[mode]; ok && m.Build != "" {
				switch m.Build {
				case "docker":
					return execComposePassthrough(e, c, append([]string{"build"}, remaining...))
				case "native":
					// Look for interaction.build.replace steps or run native build
					if ic, ok := c.Interaction["build"]; ok && len(ic.Replace) > 0 {
						for _, step := range ic.Replace {
							cmds := step.RunCommands()
							for _, cmdStr := range cmds {
								fmt.Printf("  $ %s\n", cmdStr)
								if err := runShellCommand(e, cmdStr); err != nil {
									return fmt.Errorf("native build failed: %w", err)
								}
							}
						}
						return nil
					}
					return fmt.Errorf("mode %q build=native but no interaction.build.replace defined", mode)
				default:
					// Custom build command
					fmt.Printf("  $ %s\n", m.Build)
					return runShellCommand(e, m.Build)
				}
			}
		}

		return execComposePassthrough(e, c, append([]string{"build"}, remaining...))
	},
}

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove all containers, networks, and isolated volumes",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		volumes, _ := cmd.Flags().GetBool("volumes")
		images, _ := cmd.Flags().GetBool("images")
		force, _ := cmd.Flags().GetBool("force")

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

		// When removing volumes, also clear provision markers
		if volumes {
			clearProvisionMarkers(c.FileDir())
		}

		// Orchestrator down for all lifecycle entries (with volume/image cleanup if requested)
		orch := lifecycle.NewOrchestrator(c, e)
		if err := orch.Down(context.Background(), lifecycle.DownOptions{
			DryRun:       dryRun,
			Volumes:      volumes,
			RemoveImages: images,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "[warn] lifecycle down: %v\n", err)
		}

		return nil
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

// parseDvaFlags extracts --mode/-M, --env/-E, --tags/-T, and --exclude-tags from args.
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
	if len(ec.Stack) > 0 {
		fmt.Fprintf(os.Stderr, "[env: %s] stack: %s\n", envName, strings.Join(ec.Stack, ", "))
	}
	if len(ec.Environment) > 0 {
		e.MergeVars(ec.Environment)
	}
	return nil
}

// resolvedMode holds the result of resolving a --mode flag against config modes.
type resolvedMode struct {
	Mode *config.ModeConfig
}

// applyDefaultMode returns the effective mode and whether it was applied from default_mode.
// It does NOT print any output — callers decide how to log.
func applyDefaultMode(c *config.Config, mode string) (string, bool) {
	if mode != "" || c.DefaultMode == "" {
		return mode, false
	}
	if _, ok := c.Modes[c.DefaultMode]; ok {
		return c.DefaultMode, true
	}
	fmt.Fprintf(os.Stderr, "[warn] default_mode '%s' not found in modes — starting all services\n", c.DefaultMode)
	return mode, false
}

// resolveMode looks up a mode name in config modes and returns resolved settings.
func resolveMode(c *config.Config, mode string) (*resolvedMode, error) {
	if mode == "" {
		return &resolvedMode{}, nil
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

	return &resolvedMode{
		Mode: &m,
	}, nil
}

// suggestProvision checks if a provision profile has been run before
// (via marker file in .sb/dva/) and prints a suggestion if not.
func suggestProvision(c *config.Config, provisionProfile string) {
	markerDir := filepath.Join(c.FileDir(), config.DotDirName)
	markerFile := filepath.Join(markerDir, "provisioned-"+provisionProfile)

	if _, err := os.Stat(markerFile); err == nil {
		return // already provisioned
	}

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

	cc := c.PrimaryComposeConfig()
	if cc != nil {
		if cc.Command != "" {
			parts := dvaexec.SplitCommand(cc.Command)
			composeCmd = parts[0]
			if len(parts) > 1 {
				composeArgs = parts[1:]
			}
		}

		cfgDir := c.FileDir()
		for _, f := range cc.Files {
			f = e.Interpolate(f)
			if !filepath.IsAbs(f) {
				f = cfgDir + "/" + f
			}
			composeArgs = append(composeArgs, "-f", f)
		}

		if cc.ProjectName != "" {
			composeArgs = append(composeArgs, "--project-name", e.Interpolate(cc.ProjectName))
		}
	}

	composeArgs = append(composeArgs, args...)
	return composeCmd, composeArgs
}
